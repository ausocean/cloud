/*
DESCRIPTION
  Authentication routes for AusOcean TV using OAuth2.

AUTHORS
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

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

package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"

	"github.com/ausocean/cloud/backend"
	"github.com/ausocean/cloud/gauth"
)

// loginHandler handles login requests, and starts the oauth2 login flow.
func (svc *service) loginHandler(c *fiber.Ctx) error {
	p, _ := svc.auth.GetProfile(backend.NewFiberHandler(c))
	if p != nil {
		return c.Redirect(c.FormValue("redirect", "/"), fiber.StatusFound)
	}
	return svc.auth.LoginHandler(backend.NewFiberHandler(c))
}

// logoutHandler removes the current session, and logs out the user.
func (svc *service) logoutHandler(c *fiber.Ctx) error {
	return svc.auth.LogoutHandler(backend.NewFiberHandler(c))
}

// callbackHandler handles callbacks from google's oauth2 flow.
func (svc *service) callbackHandler(c *fiber.Ctx) error {
	_, err := svc.auth.CallbackHandler(backend.NewFiberHandler(c))
	if errors.Is(err, &gauth.ErrOauth2RedirectError{}) {
		log.Warn(err)
		return c.Redirect("/", fiber.StatusFound)
	} else if err != nil {
		return logAndReturnError(c, fmt.Sprintf("error handling callback: %v", err))
	}

	// Here is where we would create a general user but since we only create users for sites, we don't need to do anything here.

	return nil
}

type profile struct {
	Role string
	gauth.Profile
}

// profileHandler handles requests to get the profile of the logged in user.
func (svc *service) profileHandler(c *fiber.Ctx) error {
	gauthProfile, err := svc.auth.GetProfile(backend.NewFiberHandler(c))
	if errors.Is(err, gauth.SessionNotFound) || errors.Is(err, gauth.TokenNotFound) {
		return logAndReturnError(c, fmt.Sprintf("error getting profile: %v", err), withStatus(fiber.StatusUnauthorized))
	} else if err != nil {
		return logAndReturnError(c, fmt.Sprintf("unable to get profile: %v", err))
	}

	p := &profile{"", *gauthProfile}
	p.Role, err = getRole(p.Email)
	if err != nil {
		return logAndReturnError(c, fmt.Sprintf("unable to get role: %v", err))
	}

	c.JSON(p)
	return nil
}

var errInvalidEmail = errors.New("invalid email")

// Possible roles that a user can be.
const (
	admin = iota
	user
)

var roles = []string{"admin", "user"}

func getRole(email string) (string, error) {
	// Give Admin role to users with ausocean emails.
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "", errInvalidEmail
	}

	if parts[1] == "ausocean.org" {
		return roles[admin], nil
	}

	return roles[user], nil
}
