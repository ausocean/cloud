/*
DESCRIPTION
  Ocean Bench API v1 handling.

AUTHORS
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2019-2024 the Australian Ocean Lab (AusOcean)

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
	"net/http"

	"github.com/ausocean/cloud/model"
)

func setupAPIV1Routes() {
	http.HandleFunc("/api/v1/sites/all", wrapAPI(getV1AllSitesHandler))
	http.HandleFunc("/api/v1/sites/public", wrapAPI(getV1PublicSitesHandler))
	http.HandleFunc("/api/v1/sites/user", wrapAPI(getV1UserSitesHandler))
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"error":"encode failure"}`, http.StatusInternalServerError)
	}
}

// /api/v1/sites/all → []minimalSite.
func getV1AllSitesHandler(w http.ResponseWriter, r *http.Request) {
	if requireProfile(w, r) == nil {
		return
	}
	sites, err := model.GetAllSites(r.Context(), settingsStore)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "could not get all sites: %v", err)
		return
	}
	out := make([]minimalSite, 0, len(sites))
	for _, s := range sites {
		out = append(out, minimalSite{
			Skey:   s.Skey,
			Name:   s.Name,
			Public: s.Public,
			// Perm left as 0 for “all sites.”.
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// /api/v1/sites/public → []minimalSite.
func getV1PublicSitesHandler(w http.ResponseWriter, r *http.Request) {
	if requireProfile(w, r) == nil {
		return
	}
	sites, err := model.GetAllSites(r.Context(), settingsStore)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "could not get public sites: %v", err)
		return
	}
	out := make([]minimalSite, 0, len(sites))
	for _, s := range sites {
		if s.Public {
			out = append(out, minimalSite{
				Skey:   s.Skey,
				Name:   s.Name,
				Public: true,
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// /api/v1/sites/user → []minimalSite with Perm set.
func getV1UserSitesHandler(w http.ResponseWriter, r *http.Request) {
	p := requireProfile(w, r)
	if p == nil {
		return
	}
	users, sites, err := model.GetUserSites(r.Context(), settingsStore, p.Email)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to get sites for user: %v. err: %v", p.Email, err)
		return
	}
	perms := make(map[int64]int64, len(users))
	for _, u := range users {
		perms[u.Skey] = u.Perm
	}
	out := make([]minimalSite, 0, len(sites))
	for _, s := range sites {
		out = append(out, minimalSite{
			Skey:   s.Skey,
			Perm:   perms[s.Skey],
			Name:   s.Name,
			Public: s.Public,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
