/*
DESCRIPTION
  OceanBench device page handling.

AUTHORS
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2025 the Australian Ocean Lab (AusOcean)

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
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
)

// setDevicesVars handles requests for the variables of the device, returning
// the populated template.
func setDevicesVars(w http.ResponseWriter, r *http.Request) {
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	skey, _ := profileData(profile)

	data := devicesData{
		commonData: commonData{},
		Mac:        r.FormValue("ma"),
		Device:     &model.Device{},
	}

	ctx := r.Context()

	// Return early if no device is selected.
	if data.Mac == "" {
		writeTemplate(w, r, "set/device_vars.html", &data, "")
	}

	data.Device.Mac = model.MacEncode(data.Mac)

	ma := strings.ToLower(strings.ReplaceAll(data.Mac, ":", ""))

	vars, err := model.GetVariablesBySite(ctx, settingsStore, skey, ma)
	if err != nil {
		writeError(w, fmt.Errorf("unable to get vars for device: %w", err))
	}

	data.Vars = vars

	writeTemplate(w, r, "set/device_vars.html", &data, "")
}
