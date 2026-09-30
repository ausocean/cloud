/*
DESCRIPTION
  OceanMedia JWT authentication.

AUTHORS
  Elliot Shine <elliot@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean).

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
  or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
  License for more details.

  You should have received a copy of the GNU General Public License in
  gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package main

import (
	"errors"
	"fmt"

	"github.com/ausocean/cloud/gauth"
	"github.com/gofiber/fiber/v2"
)

var (
	errMissingToken = errors.New("missing token")
	errInvalidToken = errors.New("invalid token")
	errUnauthorized = errors.New("unauthorized")
)

// authenticate validates the request's JWT and ensures that it authorises
// access to the given broadcast. Tokens are minted by referring services
// (such as OceanBench and AusOceanTV) using the shared JWT secret and
// supplied to players as the "token" query parameter. A Bearer token in
// the Authorization header is also accepted for programmatic clients.
//
// Required claims are "sub" (the broadcast ID) and "exp".
func (s *server) authenticate(c *fiber.Ctx, broadcastID string) error {
	token := c.Query("token")
	if token == "" {
		token = c.Get(fiber.HeaderAuthorization)
	}
	if token == "" {
		return errMissingToken
	}

	claims, err := gauth.GetClaims(token, s.jwtSecret)
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidToken, err)
	}
	if _, ok := claims["exp"]; !ok {
		return fmt.Errorf("%w: missing exp claim", errInvalidToken)
	}
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return fmt.Errorf("%w: missing sub claim", errInvalidToken)
	}
	if sub != broadcastID {
		return fmt.Errorf("%w: token is not valid for broadcast %s", errUnauthorized, broadcastID)
	}
	return nil
}
