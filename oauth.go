package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

var (
	ctx = context.Background()

	// keycloak server
	keycloakIssuer = "http://localhost:8080/realms/myrealm"
	clientID       = "gosrv"
	// where?
	clientSecret = "CLIENT_SECRET"
	redirectURL  = "http://localhost:5000/callback"

	// globals
	oauth2Config oauth2.Config
	verifier     *oidc.IDTokenVerifier

	// Cookie-based session store
	store = sessions.NewCookieStore([]byte("dev-secret-CHANGE-ME"))
)

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

	// Extract custom claims
	// THese are the information about WHO logged in
	var claims struct {
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		// handle error
		fmt.Println("handleOAuth2Callback: TODO handling idtoken.Claims error. Err:", err)
	}

	fmt.Println("handleOAuth2Callback: Maybe reached end of oauthflow successfully. Claims:", claims)
	io.WriteString(w, fmt.Sprintf("%v <br>", claims))
	// this sholud return the full raw JWT (Oauth OIDC ID TOKEN)
	io.WriteString(w, fmt.Sprintf("rawIDToken: %v", rawIDToken))

}

// testing if we can set cookies willy-nilly
func setMyCookie(w http.ResponseWriter, r *http.Request) {
	cookie := "mycookie=bXlBd2Vzb21lQ29va2llIGJlbGl2ZSBpdAo="

	w.Header().Add("Set-Cookie", cookie)

	io.WriteString(w, fmt.Sprintf("attempted to set cookie in your browser setting the following header on this Response:<br> Set-Cookie: %v", cookie))
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

	mux.HandleFunc("/setcookie", setMyCookie)

}
