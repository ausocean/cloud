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

func createLogHandler(w http.ResponseWriter, r *http.Request) {

	ctx := context.Background()

	// Validate the user is logged in and should be allowed to create the log.
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	skey, _ := profileData(profile)

	user, err := model.GetUser(ctx, settingsStore, skey, profile.Email)
	if errors.Is(err, datastore.ErrNoSuchEntity) || (err == nil && user.Perm&model.WritePermission == 0) {
		log.Println("user does not have write permissions")
		w.WriteHeader(http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Printf("failed to get permission for user: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Parse the fields to be put into the log.
	skStr := r.FormValue("sk")
	maStr := r.FormValue("ma")
	ld := r.FormValue("lg")

	sk, _ := strconv.ParseInt(skStr, 10, 64)
	ma, _ := strconv.ParseInt(maStr, 10, 64)

	// Use the datastore functions that you created to _put_ the new log.
	err = model.PutLog(ctx, settingsStore, &model.Log{Skey: sk, DeviceMAC: ma, Note: ld})

	// Return any errors, or success code
	if err != nil {
		log.Printf("failed to put Log: %v", err)
	}
}
