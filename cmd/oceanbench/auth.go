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
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/ausocean/cloud/backend"
	"github.com/ausocean/cloud/gauth"
	"github.com/gofiber/fiber/v2"
)

// standaloneData holds (temporary) profile data in standalone mode.
var standaloneData string

// loginHandler handles user login requests.
func loginHandler(c *fiber.Ctx) error {
	logRequestFiber(c)
	if standalone {
		return nil
	}

	err := auth.LoginHandler(backend.NewFiberHandler(c))
	if err != nil {
		writeErrorFiber(c, err)
		return err
	}
	return nil
}

// logoutHandler handles user logout requests.
func logoutHandler(c *fiber.Ctx) error {
	logRequestFiber(c)
	if standalone {
		return nil
	}

	err := auth.LogoutHandler(backend.NewFiberHandler(c))
	if err != nil {
		writeErrorFiber(c, err)
		return err
	}
	return nil
}

// oauthCallbackHandler implements the OAuth2 callback that completes the authentication process.
func oauthCallbackHandler(c *fiber.Ctx) error {
	logRequestFiber(c)
	if standalone {
		return nil
	}
	_, err := auth.CallbackHandler(backend.NewFiberHandler(c))
	log.Println("errors is:", errors.Is(err, &gauth.ErrOauth2RedirectError{}))
	if errors.Is(err, &gauth.ErrOauth2RedirectError{}) {
		return c.Redirect("/", fiber.StatusFound)
	} else if err != nil {
		log.Println("got error:", err)
		writeErrorFiber(c, err)
		return err
	}
	return nil
}

// getProfile returns the profile for the logged-in user.
func getProfile(w http.ResponseWriter, r *http.Request) (*gauth.Profile, error) {
	if standalone {
		return &gauth.Profile{Email: localEmail, Data: standaloneData}, nil
	}
	return auth.GetProfile(backend.NewNetHandler(w, r, auth.NetStore))
}

// getProfileFiber returns the profile for the logged-in user.
func getProfileFiber(c *fiber.Ctx) (*gauth.Profile, error) {
	if standalone {
		return &gauth.Profile{Email: localEmail, Data: standaloneData}, nil
	}
	return auth.GetProfile(backend.NewFiberHandler(c))
}

// putProfileData puts profile data.
func putProfileData(w http.ResponseWriter, r *http.Request, val string) error {
	if standalone {
		standaloneData = val
		return nil
	}
	return auth.PutData(backend.NewNetHandler(w, r, auth.NetStore), val)
}

// putProfileDataFiber puts profile data.
func putProfileDataFiber(c *fiber.Ctx, val string) error {
	if standalone {
		standaloneData = val
		return nil
	}
	return auth.PutData(backend.NewFiberHandler(c), val)
}

// profileData extracts site key and name from the given profile.
func profileData(profile *gauth.Profile) (int64, string) {
	if profile == nil {
		return 0, ""
	}
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

// requestSiteData gets the site key from the request URL if present,
// otherwise falls back to the user's profile data.
func requestSiteData(r *http.Request, profile *gauth.Profile) (int64, string) {
	if siteStr := r.URL.Query().Get("site"); siteStr != "" {
		if siteKey, err := strconv.ParseInt(siteStr, 10, 64); err == nil {
			// A valid site key is in the URL.
			return siteKey, ""
		}
	}
	return profileData(profile)
}

// requestSiteDataFiber gets the site key from the request URL if present,
// otherwise falls back to the user's profile data.
func requestSiteDataFiber(c *fiber.Ctx, profile *gauth.Profile) (int64, string) {
	if siteStr := c.Query("site"); siteStr != "" {
		if siteKey, err := strconv.ParseInt(siteStr, 10, 64); err == nil {
			// A valid site key is in the URL.
			return siteKey, ""
		}
	}
	return profileData(profile)
}
