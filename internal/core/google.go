package core

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleProvider struct {
	config *oauth2.Config
}

func NewGoogleProvider() *GoogleProvider {
	serverUrl := os.Getenv("SERVER_URL")
	if serverUrl == "" {
		log.Fatal("SERVER_URL env not set")
	}
	redirectUrl := fmt.Sprintf("%s/auth/google/callback", strings.TrimRight(serverUrl, "/"))
	// config docs - https://developers.google.com/identity/protocols/oauth2/web-server#creatingclient
	cfg := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  redirectUrl,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
	return &GoogleProvider{
		config: cfg,
	}
}

func (g *GoogleProvider) AuthCodeUrl(state string, opts ...oauth2.AuthCodeOption) string {
	return g.config.AuthCodeURL(state, opts...)
}

func (g *GoogleProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return g.config.Exchange(ctx, code)
}

func (g *GoogleProvider) ProviderName() string {
	return "Google"
}
