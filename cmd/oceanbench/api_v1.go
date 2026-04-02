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
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/datastore"
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
	v1.Get("/media", getMediaKeysHandler)
	v1.Delete("/media", deleteV1MediaHandler)
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

// getMediaKeysHandler handles GET /api/v1/media.
//
// Super-admin only. Returns an array of MtsMedia key IDs for the currently
// selected site for the past month.
func getMediaKeysHandler(c *fiber.Ctx) error {
	p := c.Locals(profileKey).(*gauth.Profile)
	if !isSuperAdmin(p.Email) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "super admin required"})
	}

	// Get site key.
	// TODO: Change this to be part of the query, profile may move away from containing the selected site.
	skey, err := skeyFromProfile(p)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("could not resolve site: %v", err)})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
	defer cancel()

	devices, err := model.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("could not get devices: %v", err)})
	}

	var ts []int64
	if fromStr := c.Query("from"); fromStr != "" {
		if fromTS, err := strconv.ParseInt(fromStr, 10, 64); err == nil {
			ts = append(ts, fromTS)
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if toTS, err := strconv.ParseInt(toStr, 10, 64); err == nil {
			ts = append(ts, toTS)
		}
	}

	if len(ts) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "time range (ts) is required"})
	}

	deviceFilterStr := c.Query("device")
	var deviceFilter int64 = -1
	if deviceFilterStr != "" {
		if d, err := strconv.ParseInt(deviceFilterStr, 10, 64); err == nil {
			deviceFilter = d
		}
	}

	pinFilterStr := c.Query("pin")

	limit := 50000
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l >= 0 {
			limit = l
		}
	}

	location := time.UTC
	if tzStr := c.Query("tz"); tzStr != "" {
		if loc, err := time.LoadLocation(tzStr); err == nil {
			location = loc
		} else {
			log.Printf("getMediaKeysHandler: failed to load location %q: %v", tzStr, err)
		}
	}

	start := time.Now()
	log.Printf("getMediaKeysHandler: Fetching media keys for site %d with timestamp filter %v", skey, ts)

	// Collect unique MIDs for all devices on this site.
	seenMIDs := make(map[int64]bool)
	var mids []int64
	for _, dev := range devices {
		if deviceFilter != -1 && dev.Mac != deviceFilter {
			continue
		}
		for _, pin := range parsePins(dev.Inputs) {
			if pinFilterStr != "" && !strings.EqualFold(pin, pinFilterStr) {
				continue
			}
			mid := model.ToMID(dev.MAC(), pin)
			if seenMIDs[mid] {
				continue
			}
			seenMIDs[mid] = true
			mids = append(mids, mid)
		}
	}

	type mediaSummaryV1 struct {
		Keys    []string       `json:"keys"`
		Summary map[string]int `json:"summary"`
	}

	out := mediaSummaryV1{
		Keys:    make([]string, 0),
		Summary: make(map[string]int),
	}

	for _, mid := range mids {
		midStart := time.Now()
		keys, err := model.GetMtsMediaKeysLimit(ctx, mediaStore, mid, nil, ts, limit)
		if err != nil {
			log.Printf("getMediaKeysHandler: Error fetching keys for MID %d: %v", mid, err)
			continue // No media keys for this MID is normal.
		}
		log.Printf("getMediaKeysHandler: Found %d keys for MID %d in %v", len(keys), mid, time.Since(midStart))
		for _, k := range keys {
			out.Keys = append(out.Keys, strconv.FormatUint(uint64(k.ID), 10))
			_, tsec, _ := datastore.SplitIDKey(k.ID)
			dateStr := time.Unix(tsec, 0).In(location).Format("2006-01-02")
			out.Summary[dateStr]++
		}
	}

	log.Printf("getMediaKeysHandler: Finished. Found %d total keys in %v", len(out.Keys), time.Since(start))

	return c.JSON(out)
}

// deleteV1MediaHandler handles DELETE /api/v1/media.
//
// Super-admin only. Expects JSON body {"key_ids": [<uint64>, ...]}.
// Verifies each key belongs to the current site before deletion.
// At most 500 keys may be deleted per request.
func deleteV1MediaHandler(c *fiber.Ctx) error {
	p := c.Locals(profileKey).(*gauth.Profile)
	if !isSuperAdmin(p.Email) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "super admin required"})
	}

	skey, err := skeyFromProfile(p)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("could not resolve site: %v", err)})
	}

	var body struct {
		KeyIDs []string `json:"key_ids"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("invalid JSON body: %v", err)})
	}
	if len(body.KeyIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "key_ids must not be empty"})
	}
	const maxDelete = 500
	if len(body.KeyIDs) > maxDelete {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("too many key_ids (max %d)", maxDelete)})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Minute)
	defer cancel()

	// Build set of MIDs belonging to this site.
	devices, err := model.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("could not get devices: %v", err)})
	}
	siteMIDs := make(map[int64]struct{})
	for _, dev := range devices {
		for _, pin := range parsePins(dev.Inputs) {
			siteMIDs[model.ToMID(dev.MAC(), pin)] = struct{}{}
		}
	}

	// Extract the lower 32 bits of the site's MIDs for fast key ownership validation
	siteLowerMIDs := make(map[int64]struct{})
	for mid := range siteMIDs {
		lower32 := int64(uint64(mid) & 0xffffffff)
		siteLowerMIDs[lower32] = struct{}{}
	}

	// Verify ownership securely using bitwise properties of the Key ID,
	// eliminating thousands of manual datastore lookups.
	var validKeys []*datastore.Key

	log.Printf("deleteV1MediaHandler: Memory-validating %d keys...", len(body.KeyIDs))
	validStart := time.Now()

	for _, kidStr := range body.KeyIDs {
		kid, err := strconv.ParseUint(kidStr, 10, 64)
		if err != nil {
			log.Printf("deleteV1MediaHandler: invalid key %q, skipping", kidStr)
			continue
		}

		// The datastore ID encodes the lower 32 bits of the MID.
		lower32, _, _ := datastore.SplitIDKey(int64(kid))

		if _, ok := siteLowerMIDs[lower32]; !ok {
			log.Printf("deleteV1MediaHandler: key %d lower_mid %d not in site %d, skipping", kid, lower32, skey)
			continue
		}

		validKeys = append(validKeys, mediaStore.IDKey("MtsMedia", int64(kid)))
	}
	log.Printf("deleteV1MediaHandler: Validated %d keys cleanly in %v", len(validKeys), time.Since(validStart))

	if len(validKeys) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no valid key_ids found for current site"})
	}

	deleteStart := time.Now()
	log.Printf("deleteV1MediaHandler: Deleting %d keys from datastore in sub-batches...", len(validKeys))

	const subBatchSize = 50
	deleted := 0
	for i := 0; i < len(validKeys); i += subBatchSize {
		end := i + subBatchSize
		if end > len(validKeys) {
			end = len(validKeys)
		}
		batch := validKeys[i:end]

		batchStart := time.Now()
		if err := mediaStore.DeleteMulti(ctx, batch); err != nil {
			log.Printf("deleteV1MediaHandler: DeleteMulti sub-batch %d-%d failed after %v: %v", i, end, time.Since(batchStart), err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   fmt.Sprintf("could not delete media: %v", err),
				"deleted": deleted,
			})
		}
		deleted += len(batch)
		log.Printf("deleteV1MediaHandler: Sub-batch %d-%d (%d keys) deleted in %v", i, end, len(batch), time.Since(batchStart))
	}
	log.Printf("deleteV1MediaHandler: Finished deleting %d keys in %v", deleted, time.Since(deleteStart))

	return c.JSON(fiber.Map{
		"deleted": deleted,
	})
}

// skeyFromProfile extracts the site key from the profile's Data field ("skey:name").
func skeyFromProfile(p *gauth.Profile) (int64, error) {
	parts := strings.SplitN(p.Data, ":", 2)
	if len(parts) < 2 {
		return 0, fmt.Errorf("no site selected in profile")
	}
	return strconv.ParseInt(parts[0], 10, 64)
}

// parsePins splits a comma-separated pin string (e.g. "A0,V0,S0") into a slice of pin names.
func parsePins(inputs string) []string {
	if inputs == "" {
		return nil
	}
	var pins []string
	for _, p := range strings.Split(inputs, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			pins = append(pins, p)
		}
	}
	return pins
}
