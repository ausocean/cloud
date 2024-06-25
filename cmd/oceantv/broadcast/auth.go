/*
DESCRIPTION
  auth.go provides functionality to obtain a google authorisation token for use
  by google APIs to allow access for control to a users account. If a file or
  google storage bucket object does not exist i.e. a token does not exist, the
  user is prompted to provided authorisation for a chosen account for which a
  token is generated and stored.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2021 the Australian Ocean Lab (AusOcean)

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

package broadcast

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"golang.org/x/oauth2"
)

// Authorisation related constants.
const youtubeCredsRedirect = "/ytCredsCallback"

// Exported error values.
var ErrGeneratedToken = errors.New("needed to generate token")

// Used to indicate if we're running in production or locally.
var production bool

// This will set the production flag i.e. to indicate whether we are running in
// cloud or locally.
func init() {
	u := os.Getenv("YOUTUBE_SECRETS")
	if u == "" {
		log.Println("error: YOUTUBE_SECRETS env var not defined")
		return
	}
	if strings.HasPrefix(u, "gs://") {
		production = true
	}
}

// getToken returns an oauth2.0 credentials token that can be used to authorise
// control of a user's account through a google API, such as the YouTube API.
// We try to get the token from storage, and if it does not exist, the user is
// prompted to provide authorisation from which a token is generated and stored.
// The token is stored either in a google storage bucket object, or in the
// filesystem, for production and local execution of this code respectively.
// In the case that the token does not exist and must be generated, a ErrGeneratedToken
// is returned. The calling handler function will need to return in response for the
// authorisation redirect to occur.
func getToken(ctx context.Context, url string) (*oauth2.Token, error) {
	var tok *oauth2.Token
	var err error
	if production {
		tok, err = objTok(ctx, url)
	} else {
		tok, err = fileTok(url)
	}

	if err != nil {
		return nil, fmt.Errorf("could not load token: %w", err)
	}

	return tok, nil
}

// getSecrets provides google app client secrets stored at the path provided
// by the YOUTUBE_SECRETS environment variable, required for set up of
// a google app configuration.
func getSecrets(ctx context.Context) ([]byte, error) {
	url := os.Getenv("YOUTUBE_SECRETS")
	if url == "" {
		return nil, errors.New("YOUTUBE_SECRETS env var not defined")
	}
	var (
		secrets []byte
		err     error
	)
	if production { // We're running in cloud.
		secrets, err = objBytes(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("could not get client secrets: %w", err)
		}
	} else {
		secrets, err = ioutil.ReadFile(url)
		if err != nil {
			return nil, fmt.Errorf("could not read secrets from local secrets file: %w", err)
		}
	}
	return secrets, nil
}
