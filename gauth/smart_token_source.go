package gauth

import (
	"context"
	"log"

	"golang.org/x/oauth2"
)

// tokenNotifyFunc is a callback function signature for notifying when a token
// event happens.
type tokenNotifyFunc func(*oauth2.Token) error

// SmartTokenSource implements the TokenSource Interface, with an additional
// callback function which is called when the underlying token is refreshed.
type SmartTokenSource struct {
	// Token Source used to get a refresh token.
	src oauth2.TokenSource

	// Callback function which is called when the token is refreshed.
	RefreshNotifyFunc tokenNotifyFunc

	// Most recent known token.
	curr *oauth2.Token
}

// NewSmartTokenSource creates a SmartTokenSource with the passed oauth2 config
// and token. The passed refreshCallback function will be called whenever the
// token is refreshed.
func NewSmartTokenSource(
	ctx context.Context,
	cfg *oauth2.Config,
	tok *oauth2.Token,
	refreshCallback tokenNotifyFunc,
) *SmartTokenSource {
	return &SmartTokenSource{
		src:               cfg.TokenSource(ctx, tok),
		RefreshNotifyFunc: refreshCallback,
		curr:              tok,
	}
}

// Token returns a Token with a valid Access Token, calling the RefreshNotifyFunc
// callback if the token is refreshed.
func (s *SmartTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.src.Token()
	if err != nil {
		return nil, err
	}

	// Check if the token was refreshed (or no previous access token was known).
	if s.curr == nil || s.curr.AccessToken != tok.AccessToken {
		// Update the sored token (curr)
		s.curr = tok
		// Call the RefreshNotifyFunc since the token was refreshed.
		if err := s.RefreshNotifyFunc(s.curr); err != nil {
			// Log the error.
			//
			// This shouldn't cause anything to misbehave as the new refresh Token
			// will still be returned, but the updated token will not be persisted.
			// (assuming that this is what the RefreshNotifyFunc is used for).
			//
			// TODO: Use an optional logging function that can be passed
			// by the caller at the smart token source creation.
			log.Printf("error from refresh notify func: %v", err)
		}
	}

	return s.curr, nil
}
