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
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	"bitbucket.org/ausocean/iotsvc/gauth"
	"bitbucket.org/ausocean/iotsvc/iotds"
)

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
			site, err := iotds.GetSite(ctx, settingsStore, skey)
			if err != nil {
				writeHttpError(w, http.StatusInternalServerError, "could not get site with site key: %v: %v", strconv.Itoa(int(skey)), err)
				return
			}
			enc := site.Encode()
			fmt.Fprint(w, string(enc))
			return

		case "sites":
			sites, err := iotds.GetAllSites(ctx, settingsStore)
			if err != nil {
				writeHttpError(w, http.StatusInternalServerError, "could not get all sites: %v", err)
				return
			}
			var s []string
			for _, site := range sites {
				if val == "all" || (val == "public" && site.Public) {
					s = append(s, strconv.Itoa(int(site.Skey))+":\""+site.Name+"\"")
				}
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

	case "health":
		// Authorization is not currently required for health operations.
		switch prop {
		case "site":
			// Device ID is optional.
			id := ""
			if len(req) == 6 {
				id = req[5]
			}

			h, err := siteHealthStatus(ctx, val, id)
			if err != nil {
				writeHttpError(w, http.StatusBadRequest, "could not get site health: %v", err)
				return
			}

			resp, err := json.Marshal(h)
			if err != nil {
				writeHttpError(w, http.StatusInternalServerError, "could not marshal JSON to response: %v", err)
				return
			}
			w.Write(resp)

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
			err := iotds.PutScalar(ctx, mediaStore, &iotds.Scalar{ID: args[0], Timestamp: args[1], Value: float64(args[2])})
			if err != nil {
				writeHttpError(w, http.StatusInternalServerError, "could not put scalar: %v", err)
				return
			}
		case "get":
			scalars, err := iotds.GetScalars(ctx, mediaStore, args[0], []int64{args[1], args[2]})
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

// siteHealthStatus collects health status for devices at the given site. If id is not
// empty, only the device with the specified ID is queried. If any queried device
// is not healthy an email notification is sent to $OPS_EMAIL.
func siteHealthStatus(ctx context.Context, site, id string) (h map[string]health, err error) {
	skey, err := strconv.ParseInt(site, 10, 64)
	if err != nil {
		return nil, errors.New("bad request")
	}
	devices, err := iotds.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		return nil, errors.New("bad request")
	}

	h = make(map[string]health)
	healthy := true
	for _, dev := range devices {
		if id == "" || id == dev.Name {
			h[dev.Name] = deviceHealthStatus(ctx, dev)
			if h[dev.Name] == healthStatusBad {
				healthy = false
			}
		}
	}

	if !healthy {
		var msg string
		if id == "" {
			msg = fmt.Sprintf("Site %d is unhealthy", skey)
		} else {
			msg = fmt.Sprintf("Device %d/%s is unhealthy", skey, id)
		}
		log.Print(msg)
		err := notifyOps(ctx, skey, "health", msg)
		if err != nil {
			log.Printf("unable to notify ops: %v", err)
		}
	}

	return h, nil
}

// deviceHealthStatus returns the status of a device: 1 for reporting, 0 for not reporting, or -1 if unknown.
func deviceHealthStatus(ctx context.Context, dev iotds.Device) health {
	v, err := iotds.GetVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".uptime")
	if err != nil {
		return healthStatusUnknown
	}
	if time.Since(v.Updated) < time.Duration(2*dev.MonitorPeriod)*time.Second {
		return healthStatusGood
	}
	return healthStatusBad
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
