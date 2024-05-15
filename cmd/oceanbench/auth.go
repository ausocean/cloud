/*
DESCRIPTION
  Ocean Bench authentication handling.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2019-2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Bench. Ocean Bench is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean Bench is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt.  If not, see http://www.gnu.org/licenses/.
*/

package main

import (
	"net/http"
	"strconv"
	"strings"

	"bitbucket.org/ausocean/iotsvc/gauth"
)

// standaloneData holds (temporary) profile data in standalone mode.
var standaloneData string

// loginHandler handles user login requests.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	if standalone {
		return
	}
	err := auth.LoginHandler(w, r)
	if err != nil {
		writeError(w, err)
	}
}

// logoutHandler handles user logout requests.
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	if standalone {
		return
	}
	err := auth.LogoutHandler(w, r)
	if err != nil {
		writeError(w, err)
	}
}

// oauthCallbackHandler implements the OAuth2 callback that completes the authentication process.
func oauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	if standalone {
		return
	}
	err := auth.CallbackHandler(w, r)
	if err != nil {
		writeError(w, err)
	}
}

// getProfile returns the profile for the logged-in user.
func getProfile(w http.ResponseWriter, r *http.Request) (*gauth.Profile, error) {
	if standalone {
		return &gauth.Profile{Email: localEmail, Data: standaloneData}, nil
	}
	return auth.GetProfile(w, r)
}

// putProfileData puts profile data.
func putProfileData(w http.ResponseWriter, r *http.Request, val string) error {
	if standalone {
		standaloneData = val
		return nil
	}
	return auth.PutData(w, r, val)
}

// profileData extracts site key and name from the given profile.
func profileData(profile *gauth.Profile) (int64, string) {
	p := strings.SplitN(profile.Data, ":", 2)
	if len(p) == 0 {
		return 0, ""
	}
	key, err := strconv.ParseInt(p[0], 10, 64)
	if err != nil {
		return 0, ""
	}
	if len(p) == 1 {
		return key, ""
	}
	return key, p[1]
}
