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
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"cloud.google.com/go/datastore"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"

	"github.com/ausocean/cloud/backend"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
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
	p, err := svc.auth.CallbackHandler(backend.NewFiberHandler(c))
	if errors.Is(err, &gauth.ErrOauth2RedirectError{}) {
		log.Warn(err)
		return c.Redirect("/", fiber.StatusFound)
	} else if err != nil {
		return logAndReturnError(c, fmt.Sprintf("error handling callback: %v", err))
	}

	// Create a new subscriber if one does not exist.
	ctx := context.Background()
	_, err = model.GetSubscriberByEmail(ctx, svc.store, p.Email)
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		subscriber := &model.Subscriber{GivenName: p.GivenName, FamilyName: p.FamilyName, Email: p.Email}
		err := model.CreateSubscriber(ctx, svc.store, subscriber)
		if err != nil {
			return logAndReturnError(c, fmt.Sprintf("unable to create susbcriber %v: %v", subscriber, err))
		}
		if !svc.lite {
			_, err = svc.getCustomer(subscriber)
			if err != nil {
				return logAndReturnError(c, fmt.Sprintf("unable to create stripe customer: %v", err))
			}
		}
	} else if err != nil {
		return logAndReturnError(c, fmt.Sprintf("failed getting subscriber by email: %v", err))
	}

	return nil
}

// profileHandler handles requests to get the profile of the logged in user.
func (svc *service) profileHandler(c *fiber.Ctx) error {
	p, err := svc.auth.GetProfile(backend.NewFiberHandler(c))
	if errors.Is(err, gauth.SessionNotFound) || errors.Is(err, gauth.TokenNotFound) {
		return logAndReturnError(c, fmt.Sprintf("error getting profile: %v", err), withStatus(fiber.StatusUnauthorized))
	} else if err != nil {
		return logAndReturnError(c, fmt.Sprintf("unable to get profile: %v", err))
	}
	bytes, err := json.Marshal(p)
	if err != nil {
		return logAndReturnError(c, fmt.Sprintf("unable to marshal profile: %v", err))
	}
	c.Write(bytes)
	return nil
}
