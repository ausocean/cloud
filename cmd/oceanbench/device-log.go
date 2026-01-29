/*
DESCRIPTION
  device-log.go provides a handler for logs.

AUTHOR
  Deborah Baker <deborah@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

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
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
)

// setLogHandler creates a new log for the given device MAC and sitekey. The request parameters are:
//
//	sk: site key
//	ma: device MAC address (encoded as int64)
//	ld: log data.
func setLogHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Validate the user is logged in.
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Parse the fields to be put into the log.
	skStr := r.FormValue("sk")
	maStr := r.FormValue("ma")
	ld := r.FormValue("lg")

	// Convert the site key and device MAC to int64.
	sk, err := strconv.ParseInt(skStr, 10, 64)
	if err != nil {
		log.Printf("failed to parse site key: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ma, err := strconv.ParseInt(maStr, 10, 64)
	if err != nil {
		log.Printf("failed to parse device MAC: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Check the user has at least admin permissions for the site they are trying to create a log for.
	user, err := model.GetUser(ctx, settingsStore, sk, profile.Email)
	if errors.Is(err, datastore.ErrNoSuchEntity) || (err == nil && user.Perm&model.AdminPermission == 0) {
		log.Println("user does not have admin permissions")
		w.WriteHeader(http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Printf("failed to get permission for user: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Put the new Log into the datastore.
	err = model.PutLog(ctx, settingsStore, &model.Log{Skey: sk, DeviceMAC: ma, Note: ld})

	// Return any errors from putting the log.
	if err != nil {
		log.Printf("failed to put Log: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Redirect to the log page.
	http.Redirect(w, r, "/logs", http.StatusSeeOther)
}

// logPageHandler handles requests for the log page.
func logPageHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	if r.URL.Path != "/logs" {
		// Redirect all invalid URLs to the root homepage.
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	profile, err := getProfile(w, r)
	skey, _ := profileData(profile)
	data := adminData{
		commonData: commonData{
			Pages:   pages("logs"),
			Profile: profile,
		},
		Skey: skey,
	}
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		writeTemplate(w, r, "log.html", &data, "")
		return
	}

	writeTemplate(w, r, "log.html", &data, "")
}
