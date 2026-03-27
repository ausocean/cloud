/*
DESCRIPTION
  Ocean Bench media manager page handling.

AUTHORS
  Trek Hopton <trek@ausocean.org>

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
  in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"net/http"

	"github.com/ausocean/cloud/gauth"
)

// mediaManagerHandler handles media manager page requests.
func mediaManagerHandler(w http.ResponseWriter, r *http.Request, profile *gauth.Profile) {
	data := monitorData{commonData: commonData{Pages: pages("media manager"), Profile: profile}}
	writeTemplate(w, r, "media-manager.html", &data, "")
}
