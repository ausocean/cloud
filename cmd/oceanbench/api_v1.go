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
)

func setupAPIV1Routes() {
	http.HandleFunc("/api/v1/sites/all", wrapAPI(withProfileJSON(getV1AllSitesHandler)))
	http.HandleFunc("/api/v1/sites/public", wrapAPI(withProfileJSON(getV1PublicSitesHandler)))
	http.HandleFunc("/api/v1/sites/user", wrapAPI(withProfileJSON(getV1UserSitesHandler)))
}

// withProfileJSON is middleware that authenticates the request and passes the
// profile to the handler. If authentication fails, it writes a JSON error
// response. This keeps auth boilerplate out of individual handlers.
func withProfileJSON(handler func(http.ResponseWriter, *http.Request, *gauth.Profile)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := getProfile(w, r)
		if err != nil {
			if err != gauth.TokenNotFound {
				log.Printf("authentication error: %v", err)
			}
			writeJSONError(w, http.StatusUnauthorized, fmt.Sprintf("user could not be authenticated: %v", err))
			return
		}
		if p == nil {
			writeJSONError(w, http.StatusUnauthorized, "user could not be authenticated")
			return
		}
		handler(w, r, p)
	}
}

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
func getV1AllSitesHandler(w http.ResponseWriter, r *http.Request, p *gauth.Profile) {
	if !isSuperAdmin(p.Email) {
		writeJSONError(w, http.StatusUnauthorized, "super admin required")
		return
	}

	sites, err := model.GetAllSites(r.Context(), settingsStore)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("could not get all sites: %v", err))
		return
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

	writeJSON(w, http.StatusOK, out)
}

// /api/v1/sites/public → []minimalSiteV1.
func getV1PublicSitesHandler(w http.ResponseWriter, r *http.Request, _ *gauth.Profile) {
	sites, err := model.GetAllSites(r.Context(), settingsStore)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("could not get public sites: %v", err))
		return
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
	writeJSON(w, http.StatusOK, out)
}

// /api/v1/sites/user → []minimalSiteV1 with Perm set.
func getV1UserSitesHandler(w http.ResponseWriter, r *http.Request, p *gauth.Profile) {
	users, sites, err := model.GetUserSites(r.Context(), settingsStore, p.Email)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("unable to get sites for user: %v, err: %v", p.Email, err))
		return
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
	writeJSON(w, http.StatusOK, out)
}
