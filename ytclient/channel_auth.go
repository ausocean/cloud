//go:build !standalone
// +build !standalone

/*
DESCRIPTION
  youtube.go provides functionality for setting up youtube livestream service
  and broadcast scheduling.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Russell Stanley <russell@ausocean.org>
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package ytclient

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"uuid"

	"cloud.google.com/go/storage"
	"github.com/ausocean/cloud/backend"
	"github.com/ausocean/cloud/gauth"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"google.golang.org/api/youtube/v3"
)

// Exported Errors.
var ErrChannelAuthNotConfigured = errors.New("ChannelAuth not configured")

// Authorisation callbacks.
const YoutubeCredsRedirect = "/ytCredsCallback"
const defaultYTAuthCallback = "http://localhost:8080" + YoutubeCredsRedirect

// Session keys.
const tokenURLKey = "token-url"

// ChannelAuth handles session management and configuration for
// authorising YouTube Channels.
type ChannelAuth struct {
	sync.Mutex
	SessionID string
	ProjectID string                // GAE project ID.
	ClientID  string                // Oauth2 client ID.
	cfg       *oauth2.Config        // Oauth2 configuration.
	NetStore  *sessions.CookieStore // Session state (only used for net/http implementations)
}

// Init initialises the ChannelAuth configuration.
func (ca *ChannelAuth) Init(h backend.Handler) {
	ca.Lock()
	defer ca.Unlock()

	if ca.cfg != nil {
		return // Already Initialised!
	}

	// Regsiter gob for Oauth2 token.
	gob.Register(&oauth2.Token{})

	if ca.SessionID == "" {
		ca.SessionID = ca.ProjectID + "ChannelAuth"
	}

	ctx := context.Background()
	var err error
	ca.cfg, err = ca.configureClient(ctx)
	if err != nil {
		log.Printf("failed to configure ytclient: %v", err)
		return
	}

	secrets, err := gauth.GetSecrets(ctx, ca.ProjectID, []string{"sessionKey"})
	if err != nil {
		log.Printf("GetSecrets failed with error: %v", err)
		return
	}

	// Only create a CookieStore if using net/http.
	_, isNetHandler := h.(*backend.NetHandler)
	if isNetHandler {
		ca.NetStore = sessions.NewCookieStore([]byte(secrets["sessionKey"]))
	}
}

// configureYTClient configures oauth2 with the YouTube scope and the configured redirect URL, defaulting
// if not set.
func (ca *ChannelAuth) configureClient(ctx context.Context) (*oauth2.Config, error) {
	cfg, err := googleConfig(ctx, youtube.YoutubeScope)
	if err != nil {
		return nil, fmt.Errorf("could not get google config: %w", err)
	}

	redirectURL := os.Getenv("YOUTUBE_AUTH_CALLBACK")
	if redirectURL == "" {
		log.Printf("YOUTUBE_AUTH_CALLBACK not defined, defaulting to %v", defaultYTAuthCallback)
		redirectURL = defaultYTAuthCallback
	}
	cfg.RedirectURL = redirectURL

	return cfg, nil
}

// AuthChannel checks for a current token under the passed tokenURI, and generates one if it does not
// yet exist.
// The generated token will have the YouTube scope.
func (ca *ChannelAuth) AuthChannel(ctx context.Context, h backend.Handler, tokenURI string) error {
	ca.Lock()
	defer ca.Unlock()
	// Don't regenerate a token if one already exists.
	_, err := getToken(ctx, tokenURI)

	if err == nil {
		log.Println("don't generate token, already exists")
		// Token exists.
		return nil
	}
	if !errors.Is(err, storage.ErrObjectNotExist) && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error getting token with uri: %s: %w", tokenURI, err)
	}

	// No token with the given URI exists yet. Generate a new token.
	return ca.generateToken(h, tokenURI)
}

// generateToken redirects the user to an authorisation page for generation of an
// authorisation token.
func (ca *ChannelAuth) generateToken(h backend.Handler, url string) error {
	sessID := uuid.New().String()
	oauthFlowSession, err := h.LoadSession(sessID)
	if err != nil {
		return fmt.Errorf("could not create session %s: %w", sessID, err)
	}

	err = oauthFlowSession.Set("token-url", url)
	if err != nil {
		return fmt.Errorf("unable to set token-url value in oauth flow session: %w", err)
	}

	maxAge := 10 * 60 // 10 minutes
	err = h.SaveSession(oauthFlowSession, maxAge)
	if err != nil {
		return fmt.Errorf("could not save session %s: %w", sessID, err)
	}

	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline, oauth2.ApprovalForce}

	return h.Redirect(ca.cfg.AuthCodeURL(sessID, opts...), http.StatusSeeOther)
}

func (ca *ChannelAuth) CallbackHandler(h backend.Handler) error {
	ca.Lock()
	defer ca.Unlock()

	if ca.cfg == nil {
		return ErrChannelAuthNotConfigured
	}

	redirectErr := h.FormValue("error")
	if redirectErr != "" {
		return fmt.Errorf("oauth2 redirect failure: %s", redirectErr)
	}

	oauthFlowSession, err := h.LoadSession(h.FormValue("state"))
	if err != nil {
		return fmt.Errorf("could not get state parameter from session store: %w", err)
	}

	tokenURL := ""
	oauthFlowSession.Get(tokenURLKey, &tokenURL)

	ctx := h.Context()
	code := h.FormValue("code")
	tok, err := ca.cfg.Exchange(ctx, code)
	if err != nil {
		log.Printf("could not exchange token: %v", err)
	}

	if production {
		err = saveTokObj(ctx, tok, tokenURL)
	} else {
		err = saveTokFile(tok, tokenURL)
	}

	if err != nil {
		log.Printf("could not save new token: %v", err)
	} else {
		log.Printf("saved new token to URL: %v", tokenURL)
	}

	return h.Redirect("/admin/broadcast", http.StatusSeeOther)
}
