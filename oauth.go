package main

import (
	"fmt"
	"io"
	"net/http"
)

var (
	// keycloak server
	keycloakIssuer = "http://localhost:8080/realms/myrealm"
	clientID       = "myapp"
	// where?
	clientSecret = "CLIENT_SECRET"
	redirectURL  = "http://localhost:8080/callback"
)

func loginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("SomeHeader", "Foobar")
	foo := 22
	io.WriteString(w, fmt.Sprintf("login-time!", foo))
}

func SetupOauth(mux *http.ServeMux) {
	mux.HandleFunc("/login", loginHandler)
}
