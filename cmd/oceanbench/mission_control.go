/*
DESCRIPTION
  Ocean Bench site mission control handling.

AUTHORS
  Russell Stanley <russell@ausocean.org>
  David Sutton <davidsutton@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2022-2024 the Australian Ocean Lab (AusOcean)

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
	"log"
	"net/http"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

// missionControlHandler handles mission control page requests.
func missionControlHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}

	data := monitorData{commonData: commonData{Pages: pages("monitor"), Profile: profile}}

	ctx := r.Context()

	skey, _ := profileData(profile)

	// Check if user has write permissions to link to devices page.
	user, err := model.GetUser(ctx, settingsStore, skey, profile.Email)
	if err == nil && user.Perm&model.WritePermission != 0 {
		data.WritePerm = true
	} else if err != nil && err != datastore.ErrNoSuchEntity {
		log.Println("failed getting user permissions", err)
	}

	site, err := model.GetSite(ctx, settingsStore, skey)
	if err != nil {
		reportMonitorError(w, r, &data, "could not get devices: %v", err)
		return
	}
	data.Timezone = site.Timezone
	data.SiteLat = site.Latitude
	data.SiteLon = site.Longitude

	writeTemplate(w, r, "mission-control.html", &data, "")
}
