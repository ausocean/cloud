/*
DESCRIPTION
  Ocean Bench API handling.

AUTHORS
  Alan Noble <alan@ausocean.org>

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
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
)

type minimalSite struct {
	Skey, Perm int64
	Name       string
	Public     bool
}

// apiHandler handles API requests which take the form:
//
//	/api/operation/property/value
func apiHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()
	p, err := getProfile(w, r)

	req := strings.Split(r.URL.Path, "/")
	if len(req) < 5 {
		writeHttpError(w, http.StatusBadRequest, "invalid length of url path")
		return
	}

	var (
		op   = req[2]
		prop = req[3]
		val  = req[4]
	)
	switch op {
	case "get":
		if err != nil {
			if err != gauth.TokenNotFound {
				log.Printf("authentication error: %v", err)
			}
			writeHttpError(w, http.StatusUnauthorized, "user could not be authenticated: %v", err)
			return
		}

		switch prop {
		case "site":
			skey, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				writeHttpError(w, http.StatusBadRequest, "could not parse site key from url: %v", err)
				return
			}
			site, err := model.GetSite(ctx, settingsStore, skey)
			if err != nil {
				writeHttpError(w, http.StatusInternalServerError, "could not get site with site key: %v: %v", strconv.Itoa(int(skey)), err)
				return
			}
			enc := site.Encode()
			fmt.Fprint(w, string(enc))
			return

		case "devices":
			switch val {
			case "site":
				// Check that the user has at least write access.
				if len(strings.Split(p.Data, ":")) != 2 {
					fmt.Fprint(w, "no site data in profile")
					return
				}
				skey, err := strconv.ParseInt(strings.Split(p.Data, ":")[0], 10, 64)
				if err != nil {
					fmt.Fprintf(w, "invalid site data in profile data: %s", p.Data)
					return
				}
				user, err := model.GetUser(ctx, settingsStore, skey, p.Email)
				if err != nil {
					fmt.Fprintf(w, "unable to get user: %v", err)
					return
				}
				if user.Perm&model.ReadPermission != 0 {
					devs, err := model.GetDevicesBySite(ctx, settingsStore, skey)
					if err != nil {
						fmt.Fprintf(w, "unable to get devices by site: %v", err)
						return
					}
					data, err := json.Marshal(devs)
					if err != nil {
						fmt.Fprintf(w, "unable to marshal devs into json: %v", err)
						return
					}
					w.Write(data)
					return
				}
			}

		case "sites":
			if val == "user" {
			}
			sites, err := model.GetAllSites(ctx, settingsStore)
			if err != nil {
				writeHttpError(w, http.StatusInternalServerError, "could not get all sites: %v", err)
				return
			}
			var s []string
			switch val {
			case "all":
				for _, site := range sites {
					s = append(s, strconv.Itoa(int(site.Skey))+":\""+site.Name+"\"")
				}
			case "public":
				for _, site := range sites {
					if site.Public {
						s = append(s, strconv.Itoa(int(site.Skey))+":\""+site.Name+"\"")
					}
				}
			case "user":
				users, sites, err := model.GetUserSites(ctx, settingsStore, p.Email)
				if err != nil {
					writeHttpError(w, http.StatusInternalServerError, "unable to get sites for user: %v. err: %v", p.Email, err)
					return
				}
				userMap := make(map[int64]int64)
				for _, u := range users {
					userMap[u.Skey] = u.Perm
				}
				var userSites []minimalSite
				for _, site := range sites {
					userSites = append(userSites, minimalSite{site.Skey, userMap[site.Skey], site.Name, site.Public})
				}
				b, err := json.Marshal(userSites)
				if err != nil {
					writeHttpError(w, http.StatusInternalServerError, "unable to marshal user sites")
					return
				}
				w.Write(b)
				return
			}
			output := "{" + strings.Join(s, ",") + "}"
			fmt.Fprint(w, output)
			return

		case "profile":
			switch val {
			case "data":
				fmt.Fprint(w, p.Data)
				return
			}

		case "vars":
			switch val {
			case "site":
				// Check that the user has at least write access.
				if len(strings.Split(p.Data, ":")) != 2 {
					fmt.Fprint(w, "no site data in profile")
					return
				}
				skey, err := strconv.ParseInt(strings.Split(p.Data, ":")[0], 10, 64)
				if err != nil {
					fmt.Fprintf(w, "invalid site data in profile data: %s", p.Data)
					return
				}
				user, err := model.GetUser(ctx, settingsStore, skey, p.Email)
				if err != nil {
					fmt.Fprintf(w, "unable to get user: %v", err)
					return
				}
				if user.Perm&model.ReadPermission != 0 {
					siteVars, err := model.GetVariablesBySite(ctx, settingsStore, skey, "")
					if err != nil {
						fmt.Fprintf(w, "unable to get variables by site: %v", err)
						return
					}

					// Only get device variables (not global or hidden).
					var vars []model.Variable
					for _, v := range siteVars {
						if strings.HasPrefix(v.Name, "_") {
							continue
						}
						s := strings.Split(v.Name, ".")
						if len(s) != 2 {
							continue
						}
						if model.IsMacAddress(s[0]) {
							vars = append(vars, v)
						}

					}

					data, err := json.Marshal(vars)
					if err != nil {
						fmt.Fprintf(w, "unable to marshal variables: %v", err)
						return
					}
					w.Write(data)
					return
				}
			}
		}

	case "set":
		if err != nil {
			if err != gauth.TokenNotFound {
				log.Printf("authentication error: %v", err)
			}
			writeHttpError(w, http.StatusUnauthorized, "user could not be authenticated: %v", err)
			return
		}

		switch prop {
		case "site":
			p := strings.SplitN(val, ":", 2)
			if len(p) != 2 {
				writeHttpError(w, http.StatusBadRequest, "invalid site data, wanted: <sitekey>:<sitename>")
				return
			}
			_, err := strconv.ParseInt(p[0], 10, 64)
			if err != nil {
				writeHttpError(w, http.StatusBadRequest, "could not parse site key from /api/set/site/<sitekey>:<sitename> : %v", err)
				return
			}
			err = putProfileData(w, r, val)
			if err != nil {
				writeHttpError(w, http.StatusInternalServerError, "could not update profile data with site data: %v", err)
				return
			}
			fmt.Fprint(w, "OK")
			return
		}

	case "test":
		// Authorization is not currently required for test operations.
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			writeHttpError(w, http.StatusBadRequest, "could not parse value from /api/test/<prop>/<value> : %v", err)
			return
		}
		body := make([]byte, n)

		// Chunk size is optional.
		chunk := n
		if len(req) == 6 {
			chunk, err = strconv.ParseInt(req[5], 10, 64)
			if err != nil {
				writeHttpError(w, http.StatusBadRequest, "could not parse chunk size from url: %v", err)
				return
			}
		}

		switch prop {
		case "upload":
			// Receive n bytes from the client.
			_, err = io.ReadFull(r.Body, body)
			if err != nil {
				writeError(w, errInvalidBody)
				return
			}
			fmt.Fprint(w, "OK")
			return

		case "download":
			// Send n bytes to the client.
			h := w.Header()
			h.Add("Content-Type", "application/octet-stream")
			h.Add("Content-Disposition", "attachment; filename=\""+val+"\"")
			rand.Read(body)
			var i int64
			for i = 0; i < n; i += chunk {
				w.Write(body[i : i+chunk])
			}
			return
		}

	case "scalar":
		// Authorization is not currently required for scalar operations.
		args, err := splitNumbers(val)
		if err != nil {
			writeHttpError(w, http.StatusInternalServerError, "invalid arg: %v", err)
			return
		}
		if len(args) != 3 {
			writeHttpError(w, http.StatusInternalServerError, "invalid number of args")
			return
		}

		var resp []byte
		switch prop {
		case "put":
			err := model.PutScalar(ctx, mediaStore, &model.Scalar{ID: args[0], Timestamp: args[1], Value: float64(args[2])})
			if err != nil {
				writeHttpError(w, http.StatusInternalServerError, "could not put scalar: %v", err)
				return
			}
		case "get":
			scalars, err := model.GetScalars(ctx, mediaStore, args[0], []int64{args[1], args[2]})
			if err != nil {
				writeHttpError(w, http.StatusInternalServerError, "could not get scalar: %v", err)
				return
			}
			resp, err = json.Marshal(scalars)
			if err != nil {
				writeHttpError(w, http.StatusInternalServerError, "error marshaling scalars: %v", err)
				return
			}
		default:
			writeHttpError(w, http.StatusBadRequest, "invalid scalar request: %s", prop)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(resp)
		return
	}

	writeHttpError(w, http.StatusBadRequest, "invalid url path, expected /get{/site, /sites}, /set/site, /test{/upload, /download}, or /health/site, got: /%v/%v", req[2], req[3])
}

// splitNumbers splits a comma-separated string of numbers, ignoring the decimal part.
func splitNumbers(s string) ([]int64, error) {
	var res []int64
	for _, v := range strings.Split(s, ",") {
		n, err := strconv.ParseInt(strings.TrimRight(v, "."), 10, 64)
		if err != nil {
			return res, err
		}
		res = append(res, n)
	}
	return res, nil
}
