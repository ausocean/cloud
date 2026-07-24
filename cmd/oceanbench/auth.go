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
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/ausocean/cloud/backend"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/gofiber/fiber/v2"
)

// standaloneData holds (temporary) profile data in standalone mode.
var standaloneData string

// loginHandler handles user login requests.
func loginHandler(c *fiber.Ctx) error {
	logRequest(c)
	if standalone {
		return nil
	}

	err := auth.LoginHandler(backend.NewFiberHandler(c))
	if err != nil {
		writeError(c, err)
		return err
	}
	return nil
}

// logoutHandler handles user logout requests.
func logoutHandler(c *fiber.Ctx) error {
	logRequest(c)
	if standalone {
		return nil
	}

	err := auth.LogoutHandler(backend.NewFiberHandler(c))
	if err != nil {
		writeError(c, err)
		return err
	}
	return nil
}

// oauthCallbackHandler implements the OAuth2 callback that completes the authentication process.
func oauthCallbackHandler(c *fiber.Ctx) error {
	logRequest(c)
	if standalone {
		return nil
	}
	_, err := auth.CallbackHandler(backend.NewFiberHandler(c))
	log.Println("errors is:", errors.Is(err, &gauth.ErrOauth2RedirectError{}))
	if errors.Is(err, &gauth.ErrOauth2RedirectError{}) {
		return c.Redirect("/", fiber.StatusFound)
	} else if err != nil {
		log.Println("got error:", err)
		writeError(c, err)
		return err
	}
	return nil
}

// getProfile returns the profile for the logged-in user.
func getProfile(c *fiber.Ctx) (*gauth.Profile, error) {
	if standalone {
		return &gauth.Profile{Email: localEmail, Data: standaloneData}, nil
	}
	return auth.GetProfile(backend.NewFiberHandler(c))
}

// putProfileData puts profile data.
func putProfileData(c *fiber.Ctx, val string) error {
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

// getDefaultSkey returns the default site key associated with a user's email. This
// is stored in a variable scoped to the users email with '@' and '.' replaced with '_'.
//
// If no default site key yet exists, one will be automatically assigned based on the
// user's current sites.
//
// A site key of `-1` will be returned on an error.
// TODO: Make this configurable.
func getDefaultSkey(ctx context.Context, profile *gauth.Profile) (int64, error) {
	if profile == nil {
		return -1, errors.New("no profile supplied")
	}
	scope := strings.ReplaceAll(profile.Email, "@", "_")
	scope = strings.ReplaceAll(scope, ".", "_")
	name := scope + ".defaultSkey"
	v, err := model.GetVariable(ctx, settingsStore, -1, name)
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		// Choose a default site from the user's current sites.
		users, err := model.GetUsers(ctx, settingsStore, profile.Email)
		if err != nil {
			return -1, fmt.Errorf("failed to get users for email (%s): %v", profile.Email, err)
		}
		err = model.PutVariable(ctx, settingsStore, -1, name, fmt.Sprintf("%d", users[0].Skey))
		if err != nil {
			// This isn't considered an error, as the caller is still returned the default site,
			// but we log it anyway as this likely indicates a systemic issue.
			log.Printf("failed to put default site: %v", err)
		}
		return users[0].Skey, nil
	} else if err != nil {
		return -1, fmt.Errorf("unable to get default skey var: %v", err)
	}

	skey, err := strconv.ParseInt(v.Value, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("unable to parse skey from default skey var (%s): %v", v.Value, err)
	}
	return skey, nil
}

// requestSiteData gets the site key from the request URL if present,
// otherwise falls back to the user's profile data.
func requestSiteData(c *fiber.Ctx, profile *gauth.Profile) (int64, string) {
	if siteStr := c.Query("site"); siteStr != "" {
		if siteKey, err := strconv.ParseInt(siteStr, 10, 64); err == nil {
			// A valid site key is in the URL.
			return siteKey, ""
		}
	}
	return profileData(profile)
}
