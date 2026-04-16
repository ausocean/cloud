/*
AUTHORS
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2025 the Australian Ocean Lab (AusOcean)

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  It is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

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
//
// TODO: Consider thread-safe implementation.
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
		// Update the stored token (curr)
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
