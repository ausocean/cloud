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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// loginHandler handles login requests, and starts the oauth2 login flow.
func (svc *service) loginHandler(c *fiber.Ctx) error {
	p, _ := svc.GetProfile(c)
	if p != nil {
		return c.Redirect(c.FormValue("target", "/"), fiber.StatusFound)
	}
	return svc.LoginHandler(c)
}

// logoutHandler removes the current session, and logs out the user.
func (svc *service) logoutHandler(c *fiber.Ctx) error {
	return svc.LogoutHandler(c)
}

// callbackHandler handles callbacks from google's oauth2 flow.
func (svc *service) callbackHandler(c *fiber.Ctx) error {
	return svc.CallbackHandler(c)
}

// profileHandler handles requests to get the profile of the logged in user.
func (svc *service) profileHandler(c *fiber.Ctx) error {
	p, err := svc.GetProfile(c)
	if errors.Is(err, SessionNotFound) || errors.Is(err, TokenNotFound) {
		return fiber.ErrUnauthorized
	} else if err != nil {
		return fmt.Errorf("unable to get profile: %w", err)
	}
	bytes, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("unable to marshal profile: %w", err)
	}
	c.Write(bytes)
	return nil
}
