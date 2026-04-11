package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"

	"encoding/gob"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

var (
	ctx = context.Background()

	// keycloak server
	//keycloakIssuer = "http://localhost:8080/realms/myrealm"
	keycloakIssuer = "https://my-keycloak.apps.testcluster.lab.local/realms/master"

	clientID = "gosrv"
	// where?
	// Got this secret form the keycloak UI after setting up the client!
	// TODO set via Config abstraction
	clientSecret = "2O0SHFSkfrWqZKVo6jklYzl8frwaJzXZ"
	redirectURL  = "http://localhost:5000/callback"

	// globals
	oauth2Config oauth2.Config
	verifier     *oidc.IDTokenVerifier

	// Cookie-based session store
	store = sessions.NewCookieStore([]byte("dev-secret-CHANGE-ME"))
)

type UserProfile struct {
	Name      string `json:"preferred_username"`
	Email     string `json:"email"`
	FirstName string `json:"given_name"`
	LastName  string `json:"family_name"`
	Verified  bool   `json:"email_verified"`
}

func init() {
	// this is necessary or else you can NOT store UserProfile structs in gorilla sessions!
	// The Sesison won't be restoreable!
	gob.Register(UserProfile{})
}

func randomState() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("SomeHeader", "Foobar")
	state := randomState()

	// THIS creates the Cookie header! And it is necessary in sofar
	// as we need the state in the state header to be set!!
	session, _ := store.Get(r, "auth")
	session.Values["state"] = state
	session.Save(r, w)

	url := oauth2Config.AuthCodeURL(state)
	// redirecting to Keycloak, starting the Auth flow

	// code http.StatusFound: this is fully intended redirection, not because the site has
	// moved or similar
	//
	// How does the oauth-flow authorization server, keycloak, know what
	// redirect URL to use? That's configured in keycloak for each Client, we configured
	// localhost:5000/callback for example
	http.Redirect(w, r, url, http.StatusFound)
}

func handleOAuth2Callback(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Entered handleOAuth2Callback", r.URL.Path)

	session, _ := store.Get(r, "auth")
	if r.URL.Query().Get("state") != session.Values["state"] {
		// How to trigger invalid state:
		// open two tabs localhost:5000/login
		http.Error(w, "invalid state", http.StatusBadRequest)
		fmt.Printf("handleOAuth2Callback: verify state failed. request state=%v, session.values[\"state\"]=%v \n", r.URL.Query().Get("state"), session.Values["state"])
		return
	}
	// Verify state and errors.
	// Apparently here _CLIENT AUTHENTICATION_ happens. So the gosrv backend is
	// getting authorizied, not any user.
	oauth2Token, err := oauth2Config.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		fmt.Println("FATAL: handleOAuth2Callback: oauth2configexchange failed. Err:", err)
		return // fatal error so we must return
	}

	// Extract the ID Token from OAuth2 token.
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		// handle missing token
		fmt.Println("FATAL: handleOAuth2Callback: TODO handle missing token. Err:", err)
		return // fatal error so we must return
	}

	// Parse and verify ID Token payload.
	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		// handle error
		fmt.Println("FATAL: handleOAuth2Callback: TODO parse and verify ID token payloa. Err:", err)
		return // fatal error so we must return
	}

	// I think at this point, only the client is authenticated

	// Extract custom claims
	// THese are the information about WHO logged in
	claims := UserProfile{}
	if err := idToken.Claims(&claims); err != nil {
		// handle error
		fmt.Println("handleOAuth2Callback: TODO handling idtoken.Claims error. Err:", err)
	}

	// The user is logged in at this point, we save the session state:
	session.Values["user"] = claims.Name
	session.Values["id_token"] = rawIDToken
	session.Values["email"] = claims.Email
	session.Values["userProfile"] = claims

	fmt.Println("session written:", session.Values["userProfile"]) // works!
	fmt.Println("session user:", session.Values["user"])           // works!
	fmt.Println("## callback session id=", session.ID)             // stays empty, this is correct as we use Cookies instead

	fmt.Println("handleOAuth2Callback: Maybe reached end of oauthflow successfully. Claims:", claims)
	fmt.Println("handleOAuth2Callback: store options Secure:", store.Options.Secure)
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)

}

// testing if we can set cookies willy-nilly
func setMyCookie(w http.ResponseWriter, r *http.Request) {
	cookie := "mycookie=bXlBd2Vzb21lQ29va2llIGJlbGl2ZSBpdAo="

	w.Header().Add("Set-Cookie", cookie)

	io.WriteString(w, fmt.Sprintf("attempted to set cookie in your browser setting the following header on this Response:<br> Set-Cookie: %v", cookie))
}

// Protected endpoints (require user login)

func profileHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "auth")

	userProfile, ok := session.Values["userProfile"]
	fmt.Println("profileHandler: session restored:", session.Values["userProfile"])
	// testing serilization issues
	fmt.Println("profileHandler: session user:", session.Values["user"])
	fmt.Println("profileHandler: session ID:", session.ID)

	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	// At this poind we have a User token => We have an session with an authenticated user!
	templatePath := filepath.Join("templates", "profile.html")
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
	tmpl.Execute(w, userProfile)
}

func SetupOauth(mux *http.ServeMux) {
	provider, err := oidc.NewProvider(ctx, keycloakIssuer)
	if err != nil {
		log.Fatal(err)
	}

	oauth2Config = oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	verifier = provider.Verifier(&oidc.Config{
		ClientID: clientID,
	})

	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/callback", handleOAuth2Callback)

	mux.HandleFunc("/profile", profileHandler)

	mux.HandleFunc("/setcookie", setMyCookie)

}
