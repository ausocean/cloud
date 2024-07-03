/*
DESCRIPTION
	Ocean Bench new entity handling.

AUTHORS
	David Sutton <davidsutton@ausocean.org>

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
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
)

// NewData contains all data required by new pages to
// fill the template.
type NewData struct {
	Types []string
	commonData
}

// newHandler handles requests to the /new/ path. Requests to this
// path should take the form: /new/<type>
//
// Where <type> denotes the type of entity to be made.
func newHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	p, err := getProfile(w, r)
	switch {
	case err != nil && !errors.Is(err, gauth.TokenNotFound):
		log.Printf("authentication error: %v", err)
		fallthrough
	case err != nil:
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	setup(ctx)

	// Check URI length
	req := strings.Split(strings.TrimPrefix(r.RequestURI, "/"), "/")
	if len(req) != 2 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Get entity type.
	switch req[1] {
	case strings.ToLower(model.DevTypeController):
	default:
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := NewData{
		commonData: commonData{Pages: pages("new"), Profile: p},
		Types:      devTypes,
	}

	data.Users, err = getUsersForSiteMenu(w, r, ctx, p, data)
	if err != nil {
		writeTemplate(w, r, "new.html", &data, fmt.Sprintf("could not populate site menu: %v", err.Error()))
		return
	}

	writeTemplate(w, r, "new.html", &data, "")
}
