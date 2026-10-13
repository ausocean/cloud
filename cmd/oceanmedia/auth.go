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
	"strings"

	"github.com/ausocean/cloud/gauth"
	"github.com/gofiber/fiber/v2"
)

var (
	errMissingToken = errors.New("missing token")
	errInvalidToken = errors.New("invalid token")
	errUnauthorized = errors.New("unauthorized")
)

// requestToken returns the JWT supplied with the request, either as the
// "token" query parameter or as a Bearer token in the Authorization header.
// The "Bearer " prefix, if present, is stripped.
func requestToken(c *fiber.Ctx) string {
	token := c.Query("token")
	if token == "" {
		token = c.Get(fiber.HeaderAuthorization)
	}
	return strings.TrimPrefix(token, "Bearer ")
}

// requestBaseURL returns the base URL (scheme://host) the request was made
// to, honouring the X-Forwarded-Proto header set by the App Engine proxy.
func requestBaseURL(c *fiber.Ctx) string {
	scheme := c.Protocol()
	if p := c.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	}
	return scheme + "://" + c.Hostname()
}

// writeAuthError writes an error response for an authentication failure.
func writeAuthError(c *fiber.Ctx, err error) error {
	status := fiber.StatusForbidden
	if errors.Is(err, errMissingToken) || errors.Is(err, errInvalidToken) {
		status = fiber.StatusUnauthorized
	}
	return c.Status(status).JSON(fiber.Map{"error": err.Error()})
}

// tokenClaims validates the request's JWT and returns its claims. Required
// claims are "sub" (the broadcast ID) and "exp".
func (s *server) tokenClaims(c *fiber.Ctx) (map[string]interface{}, error) {
	token := requestToken(c)
	if token == "" {
		return nil, errMissingToken
	}

	claims, err := gauth.GetClaims(token, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidToken, err)
	}
	if _, ok := claims["exp"]; !ok {
		return nil, fmt.Errorf("%w: missing exp claim", errInvalidToken)
	}
	if sub, ok := claims["sub"].(string); !ok || sub == "" {
		return nil, fmt.Errorf("%w: missing sub claim", errInvalidToken)
	}
	return claims, nil
}

// authenticate validates the request's JWT and ensures that it authorises
// access to the given broadcast.
func (s *server) authenticate(c *fiber.Ctx, broadcastID string) error {
	claims, err := s.tokenClaims(c)
	if err != nil {
		return err
	}
	if sub, _ := claims["sub"].(string); sub != broadcastID {
		return fmt.Errorf("%w: token is not valid for broadcast %s", errUnauthorized, broadcastID)
	}
	return nil
}
