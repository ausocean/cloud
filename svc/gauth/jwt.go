/*
AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2023 the Australian Ocean Lab (AusOcean)

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
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// PutClaims digitally signs JSON Web Token (JWT) claims using the
// supplied secret by means of the HMAC-SHA-256 signing method.
func PutClaims(claims map[string]interface{}, secret []byte) (string, error) {
	if secret == nil {
		return "", errors.New("missing secret")
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(claims))
	tokString, err := tok.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("error signing token: %w", err)
	}
	return tokString, nil
}

// GetClaims retrieves JWT claims from a token string using the supplied secret.
// Any "Bearer" string prefix will be ignored.
func GetClaims(tokString string, secret []byte) (map[string]interface{}, error) {
	tokString = strings.TrimPrefix(tokString, "Bearer ")
	if tokString == "" {
		return nil, errors.New("missing token")
	}
	if secret == nil {
		return nil, errors.New("missing secret")
	}
	tok, err := jwt.Parse(tokString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not parse token: %w", err)
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !tok.Valid || !ok {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
