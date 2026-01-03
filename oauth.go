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
	// Verify state and errors.

	fmt.Println("Entered handleOAuth2Callback", r.URL.Path)

	oauth2Token, err := oauth2Config.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		fmt.Println("handleOAuth2Callback: oauth2configexchange failed. Err:", err)
	}

	// Extract the ID Token from OAuth2 token.
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		// handle missing token
		fmt.Println("handleOAuth2Callback: TODO handle missing token. Err:", err)

	}

	// Parse and verify ID Token payload.
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		// handle error
		fmt.Println("handleOAuth2Callback: TODO parse and verify ID token payloa. Err:", err)

	}

	// Extract custom claims
	var claims struct {
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		// handle error
		fmt.Println("handleOAuth2Callback: TODO hanndle idtoken.Claims error. Err:", err)
	}

	fmt.Println("handleOAuth2Callback: Maybe reached end of oauthflow successfully. Claims:", claims)

	io.WriteString(w, fmt.Sprintf("%s <br>", claims))

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

}
