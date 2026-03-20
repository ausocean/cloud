/*
DESCRIPTION
  Ocean Bench API v1 handling.

AUTHORS
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2019-2026 the Australian Ocean Lab (AusOcean)

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
  in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// profileKey is the Locals key used to pass a *gauth.Profile through Fiber middleware.
const profileKey = "profile"

// setupAPIV1Routes registers /api/v1 routes on the provided Fiber Router.
// The withProfileJSON middleware is applied to the whole /v1 group so every
// route automatically requires authentication and has the profile available.
func setupAPIV1Routes(api fiber.Router) {
	v1 := api.Group("/v1", withProfileJSON)
	v1.Get("/sites/all", getV1AllSitesHandler)
	v1.Get("/sites/public", getV1PublicSitesHandler)
	v1.Get("/sites/user", getV1UserSitesHandler)
}

// withProfileJSON is Fiber middleware that authenticates the request and stores
// the *gauth.Profile in c.Locals(profileKey). If authentication fails it writes
// a JSON error response and aborts the chain.
func withProfileJSON(c *fiber.Ctx) error {
	// Build a *http.Request from the Fiber/fasthttp context so we can reuse the
	// existing getProfile helper which relies on net/http cookies.
	var r http.Request
	if err := fasthttpadaptor.ConvertRequest(c.Context(), &r, true); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not read request"})
	}

	// A noop ResponseWriter is sufficient here: getProfile only reads from the
	// request (cookies), it does not write to w.
	p, err := getProfile(noopResponseWriter{}, &r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": fmt.Sprintf("user could not be authenticated: %v", err)})
	}
	if p == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user could not be authenticated"})
	}
	c.Locals(profileKey, p)
	return c.Next()
}

// noopResponseWriter satisfies http.ResponseWriter but discards all writes.
// It is only used when calling getProfile, which reads cookies but never writes.
type noopResponseWriter struct{}

func (noopResponseWriter) Header() http.Header         { return http.Header{} }
func (noopResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (noopResponseWriter) WriteHeader(int)             {}

type minimalSiteV1 struct {
	Skey   int64  `json:"Skey"`
	Perm   int64  `json:"Perm,omitempty"`
	Name   string `json:"Name"`
	Public bool   `json:"Public"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"error":"encode failure"}`, http.StatusInternalServerError)
	}
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// /api/v1/sites/all → []minimalSiteV1 (SUPER ADMIN ONLY).
func getV1AllSitesHandler(c *fiber.Ctx) error {
	p := c.Locals(profileKey).(*gauth.Profile)
	if !isSuperAdmin(p.Email) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "super admin required"})
	}

	sites, err := model.GetAllSites(c.Context(), settingsStore)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("could not get all sites: %v", err)})
	}

	out := make([]minimalSiteV1, 0, len(sites))
	for _, s := range sites {
		out = append(out, minimalSiteV1{
			Skey:   s.Skey,
			Name:   s.Name,
			Public: s.Public,
			// Perm intentionally 0 for "all sites".
		})
	}

	return c.JSON(out)
}

// /api/v1/sites/public → []minimalSiteV1.
func getV1PublicSitesHandler(c *fiber.Ctx) error {
	sites, err := model.GetAllSites(c.Context(), settingsStore)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("could not get public sites: %v", err)})
	}
	out := make([]minimalSiteV1, 0, len(sites))
	for _, s := range sites {
		if s.Public {
			out = append(out, minimalSiteV1{
				Skey:   s.Skey,
				Name:   s.Name,
				Public: true,
			})
		}
	}
	return c.JSON(out)
}

// /api/v1/sites/user → []minimalSiteV1 with Perm set.
func getV1UserSitesHandler(c *fiber.Ctx) error {
	p := c.Locals(profileKey).(*gauth.Profile)
	users, sites, err := model.GetUserSites(c.Context(), settingsStore, p.Email)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("unable to get sites for user: %v, err: %v", p.Email, err)})
	}
	perms := make(map[int64]int64, len(users))
	for _, u := range users {
		perms[u.Skey] = u.Perm
	}
	out := make([]minimalSiteV1, 0, len(sites))
	for _, s := range sites {
		out = append(out, minimalSiteV1{
			Skey:   s.Skey,
			Perm:   perms[s.Skey],
			Name:   s.Name,
			Public: s.Public,
		})
	}
	return c.JSON(out)
}
