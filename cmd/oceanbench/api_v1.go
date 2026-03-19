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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/gofiber/adaptor/v2"
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
	v1.Get("/media", adaptor.HTTPHandlerFunc(mediaV1Handler))
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

// minimalMediaV1 is the metadata-only DTO for an MtsMedia entity.
// The Clip field is intentionally omitted to keep responses small.
type minimalMediaV1 struct {
	KeyID       uint64    `json:"key_id"`
	MID         int64     `json:"mid"`
	MAC         string    `json:"mac"`
	Pin         string    `json:"pin"`
	Timestamp   int64     `json:"timestamp"`
	DurationSec float64   `json:"duration_sec"`
	Type        string    `json:"type"`
	Geohash     string    `json:"geohash,omitempty"`
	Date        time.Time `json:"date"`
	ClipSize    int       `json:"clip_size_bytes"`
}

// mediaV1Handler dispatches GET and DELETE requests for /api/v1/media.
func mediaV1Handler(w http.ResponseWriter, r *http.Request, p *gauth.Profile) {
	switch r.Method {
	case http.MethodGet:
		getV1MediaHandler(w, r, p)
	case http.MethodDelete:
		deleteV1MediaHandler(w, r, p)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "only GET and DELETE are supported")
	}
}

// getV1MediaHandler handles GET /api/v1/media.
//
// Super-admin only. Returns metadata for all MtsMedia belonging to the
// currently selected site. Clip bytes are never returned.
//
// Optional query parameters:
//
//	mid=<MID>               filter by a specific Media ID
//	from=<unix-timestamp>   filter to media at or after this time
//	to=<unix-timestamp>     filter to media before this time
func getV1MediaHandler(w http.ResponseWriter, r *http.Request, p *gauth.Profile) {
	if !isSuperAdmin(p.Email) {
		writeJSONError(w, http.StatusUnauthorized, "super admin required")
		return
	}

	skey, err := skeyFromProfile(p)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("could not resolve site: %v", err))
		return
	}

	ctx := r.Context()

	devices, err := model.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("could not get devices: %v", err))
		return
	}

	// Optional filters.
	var filterMID int64
	if midStr := r.URL.Query().Get("mid"); midStr != "" {
		filterMID, err = strconv.ParseInt(midStr, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid mid: %v", err))
			return
		}
	}
	var ts []int64
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		fromTS, err := strconv.ParseInt(fromStr, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid from: %v", err))
			return
		}
		ts = append(ts, fromTS)
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		toTS, err := strconv.ParseInt(toStr, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid to: %v", err))
			return
		}
		ts = append(ts, toTS)
	}

	// Collect MID entries for all devices on this site.
	// We deduplicate by MID because some pin types (e.g. A0 and V0) encode
	// to the same MID via model.ToMID, and querying the same MID twice
	// would produce duplicate results.
	type midEntry struct {
		mid int64
		mac string
		pin string
	}
	seenMIDs := make(map[int64]bool)
	var mids []midEntry
	for _, dev := range devices {
		for _, pin := range parsePins(dev.Inputs) {
			mid := model.ToMID(dev.MAC(), pin)
			if filterMID != 0 && mid != filterMID {
				continue
			}
			if seenMIDs[mid] {
				continue
			}
			seenMIDs[mid] = true
			mids = append(mids, midEntry{mid: mid, mac: dev.MAC(), pin: pin})
		}
	}

	var out []minimalMediaV1
	for _, entry := range mids {
		clips, err := model.GetMtsMedia(ctx, mediaStore, entry.mid, nil, ts)
		if err != nil {
			continue // No media for this MID is normal.
		}
		for i := range clips {
			c := &clips[i]
			item := minimalMediaV1{
				MID:         c.MID,
				MAC:         entry.mac,
				Pin:         entry.pin,
				Timestamp:   c.Timestamp,
				DurationSec: model.PTSToSeconds(c.Duration),
				Type:        c.Type,
				Geohash:     c.Geohash,
				Date:        time.Unix(c.Timestamp, 0).UTC(),
				ClipSize:    len(c.Clip),
			}
			if c.Key != nil {
				item.KeyID = uint64(c.Key.ID)
			}
			out = append(out, item)
		}
	}

	// Sort newest first.
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp > out[j].Timestamp
	})

	writeJSON(w, http.StatusOK, out)
}

// deleteV1MediaHandler handles DELETE /api/v1/media.
//
// Super-admin only. Expects JSON body {"key_ids": [<uint64>, ...]}.
// Verifies each key belongs to the current site before deletion.
// At most 500 keys may be deleted per request.
func deleteV1MediaHandler(w http.ResponseWriter, r *http.Request, p *gauth.Profile) {
	if !isSuperAdmin(p.Email) {
		writeJSONError(w, http.StatusUnauthorized, "super admin required")
		return
	}

	skey, err := skeyFromProfile(p)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("could not resolve site: %v", err))
		return
	}

	var body struct {
		KeyIDs []uint64 `json:"key_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		return
	}
	if len(body.KeyIDs) == 0 {
		writeJSONError(w, http.StatusBadRequest, "key_ids must not be empty")
		return
	}
	const maxDelete = 500
	if len(body.KeyIDs) > maxDelete {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("too many key_ids (max %d)", maxDelete))
		return
	}

	ctx := r.Context()

	// Build set of MIDs belonging to this site.
	devices, err := model.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("could not get devices: %v", err))
		return
	}
	siteMIDs := make(map[int64]struct{})
	for _, dev := range devices {
		for _, pin := range parsePins(dev.Inputs) {
			siteMIDs[model.ToMID(dev.MAC(), pin)] = struct{}{}
		}
	}

	// Verify ownership then build keys to delete.
	var validKeys []*datastore.Key
	for _, kid := range body.KeyIDs {
		m, err := model.GetMtsMediaByKey(ctx, mediaStore, kid)
		if err != nil {
			log.Printf("deleteV1MediaHandler: key %d not found, skipping: %v", kid, err)
			continue
		}
		if _, ok := siteMIDs[m.MID]; !ok {
			log.Printf("deleteV1MediaHandler: key %d MID %d not in site %d, skipping", kid, m.MID, skey)
			continue
		}
		validKeys = append(validKeys, mediaStore.IDKey("MtsMedia", int64(kid)))
	}

	if len(validKeys) == 0 {
		writeJSONError(w, http.StatusBadRequest, "no valid key_ids found for current site")
		return
	}

	if err := mediaStore.DeleteMulti(ctx, validKeys); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("could not delete media: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": len(validKeys),
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
