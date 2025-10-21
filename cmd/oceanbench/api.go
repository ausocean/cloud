/*
DESCRIPTION
  Ocean Bench API handling.

AUTHORS
  Alan Noble <alan@ausocean.org>
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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adrianmo/go-nmea"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
)

type minimalSite struct {
	Skey, Perm int64
	Name       string
	Public     bool
}

// setupAPIRoutes registers all HTTP handlers for API endpoints.
//
// API requests follow the form:
//
//	/api/<operation>/<property>[/<value>]
//
// Where:
//   - <operation> is one of: get, set, test
//   - <property> depends on the operation (e.g., "site", "devices", "upload")
//   - <value> may be a numeric ID, string key, or omitted for some routes
//
// Example routes:
//
//	/api/get/site/123            → Get site by key
//	/api/get/devices/site        → Get devices for the current profile’s site
//	/api/get/profile/data        → Get current user’s profile data
//	/api/set/site/123:MySite     → Set profile site data
//
// Only some endpoints require authentication. These checks are handled per handler.
func setupAPIRoutes() {
	http.HandleFunc("/api/get/site/", wrapAPI(getSiteHandler))
	http.HandleFunc("/api/get/devices/site", wrapAPI(getDevicesForSiteHandler))
	http.HandleFunc("/api/get/sites/all", wrapAPI(getAllSitesHandler))
	http.HandleFunc("/api/get/sites/public", wrapAPI(getPublicSitesHandler))
	http.HandleFunc("/api/get/sites/user", wrapAPI(getUserSitesHandler))
	http.HandleFunc("/api/get/profile/data", wrapAPI(getProfileDataHandler))
	http.HandleFunc("/api/get/vars/site", wrapAPI(getVarsForSiteHandler))
	http.HandleFunc("/api/get/sensor/data/", wrapAPI(getSensorDataHandler))
	http.HandleFunc("/api/get/gpstrail/", wrapAPI(getGPSTrailHandler))

	http.HandleFunc("/api/set/site/", wrapAPI(setSiteHandler))

	http.HandleFunc("/api/test/upload/", wrapAPI(testUploadHandler))
	http.HandleFunc("/api/test/download/", wrapAPI(testDownloadHandler))

	// TODO: change these to the form /api/get/scalar and /api/set/scalar.
	http.HandleFunc("/api/scalar/put/", wrapAPI(scalarPutHandler))
	http.HandleFunc("/api/scalar/get/", wrapAPI(scalarGetHandler))
}

// wrapAPI does things that are common for all api requests, such as log the request.
func wrapAPI(handler func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		handler(w, r)
	}
}

func getSiteHandler(w http.ResponseWriter, r *http.Request) {
	// Require authentication.
	if requireProfile(w, r) == nil {
		return
	}

	// Get site key from URL path.
	val, err := getPathValue(r, 4)
	if err != nil {
		writeHttpError(w, http.StatusBadRequest, err.Error())
		return
	}

	skey, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		writeHttpError(w, http.StatusBadRequest, "could not parse site key: %v", err)
		return
	}

	site, err := model.GetSite(r.Context(), settingsStore, skey)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "could not get site with site key: %d: %v", skey, err)
		return
	}

	enc := site.Encode()
	fmt.Fprint(w, string(enc))
}

func getDevicesForSiteHandler(w http.ResponseWriter, r *http.Request) {
	p := requireProfile(w, r)
	if p == nil {
		return
	}

	parts := strings.Split(p.Data, ":")
	if len(parts) != 2 {
		writeHttpError(w, http.StatusBadRequest, "no site data in profile")
		return
	}
	skey, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeHttpError(w, http.StatusBadRequest, "invalid site key in profile data: %s", p.Data)
		return
	}

	user, err := model.GetUser(r.Context(), settingsStore, skey, p.Email)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to get user: %v", err)
		return
	}
	if user.Perm&model.ReadPermission == 0 {
		writeHttpError(w, http.StatusUnauthorized, "profile does not have read permissions")
		return
	}

	devices, err := model.GetDevicesBySite(r.Context(), settingsStore, skey)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to get devices by site: %v", err)
		return
	}

	data, err := json.Marshal(devices)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to marshal devices: %v", err)
		return
	}
	w.Write(data)
}

func getAllSitesHandler(w http.ResponseWriter, r *http.Request) {
	// Require authentication.
	if requireProfile(w, r) == nil {
		return
	}

	sites, err := model.GetAllSites(r.Context(), settingsStore)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "could not get all sites: %v", err)
		return
	}

	var s []string
	for _, site := range sites {
		s = append(s, strconv.Itoa(int(site.Skey))+":\""+site.Name+"\"")
	}

	output := "{" + strings.Join(s, ",") + "}"
	fmt.Fprint(w, output)
}

func getPublicSitesHandler(w http.ResponseWriter, r *http.Request) {
	// Require authentication.
	if requireProfile(w, r) == nil {
		return
	}

	sites, err := model.GetAllSites(r.Context(), settingsStore)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "could not get public sites: %v", err)
		return
	}

	var s []string
	for _, site := range sites {
		if site.Public {
			s = append(s, strconv.Itoa(int(site.Skey))+":\""+site.Name+"\"")
		}
	}

	output := "{" + strings.Join(s, ",") + "}"
	fmt.Fprint(w, output)
}

func getUserSitesHandler(w http.ResponseWriter, r *http.Request) {
	p := requireProfile(w, r)
	if p == nil {
		return
	}

	users, sites, err := model.GetUserSites(r.Context(), settingsStore, p.Email)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to get sites for user: %v. err: %v", p.Email, err)
		return
	}

	// Build permission map.
	userMap := make(map[int64]int64)
	for _, u := range users {
		userMap[u.Skey] = u.Perm
	}

	var userSites []minimalSite
	for _, site := range sites {
		userSites = append(userSites, minimalSite{
			Skey:   site.Skey,
			Perm:   userMap[site.Skey],
			Name:   site.Name,
			Public: site.Public,
		})
	}

	data, err := json.Marshal(userSites)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to marshal user sites")
		return
	}
	w.Write(data)
}

func getProfileDataHandler(w http.ResponseWriter, r *http.Request) {
	p := requireProfile(w, r)
	if p == nil {
		return
	}

	fmt.Fprint(w, p.Data)
}

func getVarsForSiteHandler(w http.ResponseWriter, r *http.Request) {
	p := requireProfile(w, r)
	if p == nil {
		return
	}

	// Get site key from profile data.
	parts := strings.Split(p.Data, ":")
	if len(parts) != 2 {
		writeHttpError(w, http.StatusUnauthorized, "no site data in profile")
		return
	}
	skey, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeHttpError(w, http.StatusBadRequest, "invalid site key in profile data: %s", p.Data)
		return
	}

	// Check for read permission.
	user, err := model.GetUser(r.Context(), settingsStore, skey, p.Email)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to get user: %v", err)
		return
	}
	if user.Perm&model.ReadPermission == 0 {
		writeHttpError(w, http.StatusUnauthorized, "profile does not have read permissions")
		return
	}

	// Get variables for the site.
	siteVars, err := model.GetVariablesBySite(r.Context(), settingsStore, skey, "")
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to get variables by site: %v", err)
		return
	}

	// Filter to only include device-specific variables (not global or hidden).
	var filtered []model.Variable
	for _, v := range siteVars {
		if strings.HasPrefix(v.Name, "_") {
			continue
		}
		parts := strings.Split(v.Name, ".")
		if len(parts) != 2 {
			continue
		}
		if model.IsMacAddress(parts[0]) {
			filtered = append(filtered, v)
		}
	}

	data, err := json.Marshal(filtered)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to marshal variables: %v", err)
		return
	}
	w.Write(data)
}

// getSensorDataHandler handles requests to get data for a given sensor. The API transforms
// the scalar data based on the sensor transform function and returns it.
//
// Expected path format:
//
//	/api/get/sensor/data
//
// Expected query params:
//
//	ma: MAC address of associated device in the form XX:XX:XX:XX:XX:XX
//	pn: Pin name, ie: X50, A0, ...
//	start: unix timestamp to start query
//	finish: unix timestamp to finish query
//
// This currently does not require authentication, and so requests are limited to 10 minute
// periods. If the requested period is more than 10 minutes, the finish time will be changed
// to be only 10 minutes after the start time.
func getSensorDataHandler(w http.ResponseWriter, r *http.Request) {
	mac := model.MacEncode(r.FormValue("ma"))
	if mac == 0 {
		writeHttpError(w, http.StatusBadRequest, "invalid MAC supplied, wanted in form XX:XX:XX:XX:XX:XX, got: %s", r.FormValue("ma"))
		return
	}

	pin := r.FormValue("pn")

	ctx := context.Background()
	sensor, err := model.GetSensorV2(ctx, settingsStore, mac, pin)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to get sensor: %v", err)
		return
	}

	start, err := strconv.ParseInt(r.FormValue("start"), 10, 64)
	if err != nil {
		writeHttpError(w, http.StatusBadRequest, "unable to parse start time as unix timestamp: %v", err)
		return
	}

	finish, err := strconv.ParseInt(r.FormValue("finish"), 10, 64)
	if err != nil {
		writeHttpError(w, http.StatusBadRequest, "unable to parse finish time as unix timestamp: %v", err)
		return
	}

	if start > finish {
		writeHttpError(w, http.StatusBadRequest, "start time must be before finish time")
		return
	}

	// For now we will limit requests to 10 minutes at a time.
	// TODO: Update once we have authentication.
	const maxPeriod = 60 * 10
	if finish-start >= maxPeriod {
		// Adjust the finish time to be 10 minutes after the start time.
		finish = start + maxPeriod
	}

	// Get the data for the sensor.
	scalars, err := model.GetScalars(ctx, mediaStore, model.ToSID(model.MacDecode(mac), pin), []int64{start, finish})
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to get scalars for sensor: %v", err)
	}

	type timedValue struct {
		Value     float64
		Timestamp int64
	}

	var output []timedValue
	for _, s := range scalars {
		transformed, err := sensor.Transform(s.Value)
		if err != nil {
			writeHttpError(w, http.StatusInternalServerError, "error whilst transforming scalar value: %v", err)
			return
		}
		output = append(output, timedValue{Value: transformed, Timestamp: s.Timestamp})
	}

	// Allow scripts to access this data.
	// TODO: restrict based on needs and authentication.
	w.Header().Set("Access-Control-Allow-Origin", "*")

	data, err := json.Marshal(output)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to marshal response data to json: %v", err)
		return
	}
	w.Write(data)
}

// getGPSTrailHandler returns recent GPS points for a device/pin from Text storage (raw NMEA).
// Endpoint: GET /api/get/gpstrail/?ma=<MAC>&pn=<pin>&start=<unix>&finish=<unix>
func getGPSTrailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	macStr := r.FormValue("ma")
	mac := model.MacEncode(macStr)
	if mac == 0 {
		writeHttpError(w, http.StatusBadRequest, "invalid MAC supplied, wanted in form XX:XX:XX:XX:XX:XX, got: %s", macStr)
		return
	}

	// Parse pin (default T1).
	pin := r.FormValue("pn")
	if pin == "" {
		pin = "T1"
	}

	var startTS, finishTS int64
	if sv := r.FormValue("start"); sv != "" {
		v, err := strconv.ParseInt(sv, 10, 64)
		if err != nil {
			writeHttpError(w, http.StatusBadRequest, "unable to parse start time as unix timestamp: %v", err)
			return
		}
		startTS = v
	}
	if fv := r.FormValue("finish"); fv != "" {
		v, err := strconv.ParseInt(fv, 10, 64)
		if err != nil {
			writeHttpError(w, http.StatusBadRequest, "unable to parse finish time as unix timestamp: %v", err)
			return
		}
		finishTS = v
	}
	if startTS != 0 && finishTS != 0 && startTS > finishTS {
		writeHttpError(w, http.StatusBadRequest, "start time must be before finish time")
		return
	}

	mid := model.ToMID(model.MacDecode(mac), pin)

	texts, err := model.GetText(ctx, mediaStore, mid, []int64{startTS, finishTS})
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to get text for gps: %v", err)
		return
	}

	type gpsOut struct {
		Lat  float64 `json:"lat"`
		Lon  float64 `json:"lon"`
		Time string  `json:"time"` // RFC3339 UTC
		TS   int64   `json:"ts"`   // unix seconds
	}

	points := make([]gpsOut, 0, len(texts))

	for _, t := range texts {
		// Some devices might batch multiple sentences in one Text row; handle that.
		for _, line := range splitLines(t.Data) {
			if line == "" || line[0] != '$' {
				continue
			}
			s, err := nmea.Parse(line)
			if err != nil {
				continue // skip malformed lines.
			}

			var (
				lat, lon float64
				ok       bool
				ts       = t.Timestamp
			)

			switch v := s.(type) {
			case nmea.RMC:
				if v.Validity != "A" {
					break
				}
				lat, lon = v.Latitude, v.Longitude
				// Prefer RMC time+date if present.
				if v.Time.Valid && v.Date.Valid {
					ts = time.Date(
						v.Date.YY,
						time.Month(v.Date.MM),
						v.Date.DD,
						v.Time.Hour, v.Time.Minute, v.Time.Second,
						v.Time.Millisecond*1e6,
						time.UTC,
					).Unix()
				}
				ok = true

			case nmea.GGA:
				// FixQuality "0" = invalid.
				if v.FixQuality == "0" {
					break
				}
				lat, lon = v.Latitude, v.Longitude
				// GGA has time only → combine with server date from Text.Timestamp.
				if v.Time.Valid {
					day := time.Unix(t.Timestamp, 0).UTC()
					ts = time.Date(
						day.Year(), day.Month(), day.Day(),
						v.Time.Hour, v.Time.Minute, v.Time.Second,
						v.Time.Millisecond*1e6,
						time.UTC,
					).Unix()
				}
				ok = true

			case nmea.GLL:
				if v.Validity != "A" {
					break
				}
				lat, lon = v.Latitude, v.Longitude
				if v.Time.Valid {
					day := time.Unix(t.Timestamp, 0).UTC()
					ts = time.Date(
						day.Year(), day.Month(), day.Day(),
						v.Time.Hour, v.Time.Minute, v.Time.Second,
						v.Time.Millisecond*1e6,
						time.UTC,
					).Unix()
				}
				ok = true

			default:
				// Ignore other sentences.
			}

			if !ok {
				continue
			}
			if lat == 0 && lon == 0 {
				continue // null island
			}

			points = append(points, gpsOut{
				Lat:  lat,
				Lon:  lon,
				Time: time.Unix(ts, 0).UTC().Format(time.RFC3339),
				TS:   ts,
			})
		}
	}

	resp := struct {
		MAC    string   `json:"mac"`
		Pin    string   `json:"pin"`
		Count  int      `json:"count"`
		Points []gpsOut `json:"points"`
	}{
		MAC:    macStr,
		Pin:    pin,
		Count:  len(points),
		Points: points,
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(resp); err != nil {
		writeHttpError(w, http.StatusInternalServerError, "unable to encode response: %v", err)
		return
	}
}

func splitLines(s string) []string {
	// Handles \n, \r\n, stray \r.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	parts := strings.Split(s, "\n")
	// Trim spaces.
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// setSiteHandler handles API requests to update the user's current site selection.
//
// Expected path format:
//
//	/api/set/site/<sitekey>:<sitename>
//
// Example:
//
//	/api/set/site/123:MySite
//
// This stores the selected site information (key and name) in the user's profile data.
// The value is passed unchanged to putProfileData, which performs the actual update.
func setSiteHandler(w http.ResponseWriter, r *http.Request) {
	p := requireProfile(w, r)
	if p == nil {
		return
	}

	// Get site data from path.
	val, err := getPathValue(r, 4) // /api/set/site/<sitekey>:<sitename>
	if err != nil {
		writeHttpError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate format: <sitekey>:<sitename>
	parts := strings.SplitN(val, ":", 2)
	if len(parts) != 2 {
		writeHttpError(w, http.StatusBadRequest, "invalid site data, wanted: <sitekey>:<sitename>")
		return
	}

	// Validate site key.
	if _, err := strconv.ParseInt(parts[0], 10, 64); err != nil {
		writeHttpError(w, http.StatusBadRequest, "could not parse site key from /api/set/site/<sitekey>:<sitename> : %v", err)
		return
	}

	// Update profile.
	if err := putProfileData(w, r, val); err != nil {
		writeHttpError(w, http.StatusInternalServerError, "could not update profile data with site data: %v", err)
		return
	}

	fmt.Fprint(w, "OK")
}

// testUploadHandler handles test upload requests.
// Reads exactly <value> bytes from the request body and responds "OK".
// Used for testing upload throughput or validation.
func testUploadHandler(w http.ResponseWriter, r *http.Request) {
	// Get byte count from path.
	val, err := getPathValue(r, 4) // /api/test/upload/<value>
	if err != nil {
		writeHttpError(w, http.StatusBadRequest, err.Error())
		return
	}

	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		writeHttpError(w, http.StatusBadRequest, "could not parse value from /api/test/upload/<value>: %v", err)
		return
	}

	body := make([]byte, n)
	_, err = io.ReadFull(r.Body, body)
	if err != nil {
		writeError(w, errInvalidBody)
		return
	}

	fmt.Fprint(w, "OK")
}

// testDownloadHandler handles test download requests.
//
// Expected path format:
//
//	/api/test/download/<n>[/<chunk>]
//
// Responds with <n> bytes of random data, optionally sent in chunks of size <chunk>.
// Used for testing download speed and behavior.
func testDownloadHandler(w http.ResponseWriter, r *http.Request) {
	req := strings.Split(r.URL.Path, "/")
	if len(req) < 5 {
		writeHttpError(w, http.StatusBadRequest, "invalid length of url path")
		return
	}

	n, err := strconv.ParseInt(req[4], 10, 64)
	if err != nil {
		writeHttpError(w, http.StatusBadRequest, "could not parse value from /api/test/download/<n>: %v", err)
		return
	}

	chunk := n // Default: whole payload in one write
	if len(req) == 6 {
		chunk, err = strconv.ParseInt(req[5], 10, 64)
		if err != nil {
			writeHttpError(w, http.StatusBadRequest, "could not parse chunk size from url: %v", err)
			return
		}
	}

	body := make([]byte, n)
	_, err = rand.Read(body)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "could not generate random data: %v", err)
		return
	}

	h := w.Header()
	h.Add("Content-Type", "application/octet-stream")
	h.Add("Content-Disposition", "attachment; filename=\""+req[4]+"\"")

	var i int64
	for i = 0; i < n; i += chunk {
		end := i + chunk
		if end > n {
			end = n
		}
		w.Write(body[i:end])
	}
}

// scalarPutHandler handles scalar data ingestion.
//
// Expected path format:
//
//	/api/scalar/put/<id>,<timestamp>,<value>
//
// Parses and stores a single scalar. No authentication required.
func scalarPutHandler(w http.ResponseWriter, r *http.Request) {
	val, err := getPathValue(r, 4) // /api/scalar/put/<id>,<timestamp>,<value>
	if err != nil {
		writeHttpError(w, http.StatusBadRequest, err.Error())
		return
	}

	args, err := splitNumbers(val)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "invalid arg: %v", err)
		return
	}
	if len(args) != 3 {
		writeHttpError(w, http.StatusInternalServerError, "invalid number of args")
		return
	}

	err = model.PutScalar(r.Context(), mediaStore, &model.Scalar{
		ID:        args[0],
		Timestamp: args[1],
		Value:     float64(args[2]),
	})
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "could not put scalar: %v", err)
		return
	}
}

// scalarGetHandler handles scalar data retrieval.
//
// Expected path format:
//
//	/api/scalar/get/<id>,<start>,<end>
//
// Returns a JSON-encoded array of scalars for the given ID and time range.
// No authentication required.
func scalarGetHandler(w http.ResponseWriter, r *http.Request) {
	val, err := getPathValue(r, 4) // /api/scalar/get/<id>,<start>,<end>
	if err != nil {
		writeHttpError(w, http.StatusBadRequest, err.Error())
		return
	}

	args, err := splitNumbers(val)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "invalid arg: %v", err)
		return
	}
	if len(args) != 3 {
		writeHttpError(w, http.StatusInternalServerError, "invalid number of args")
		return
	}

	scalars, err := model.GetScalars(r.Context(), mediaStore, args[0], []int64{args[1], args[2]})
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "could not get scalar: %v", err)
		return
	}

	data, err := json.Marshal(scalars)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "error marshaling scalars: %v", err)
		return
	}
	w.Write(data)
}

// getPathValue extracts a segment from the URL path at the given zero-based index.
// The URL path is split using "/" as the delimiter.
// Because leading slashes in the path produce an empty first element (""), the parts array
// will have an empty string at index 0.
//
// For example, for the URL "/api/get/site/123", splitting on "/" produces:
//
//	["", "api", "get", "site", "123"]
//
// Therefore, index 4 corresponds to "123".
//
// Returns an error if the path does not have enough parts, or if the extracted part is empty.
func getPathValue(r *http.Request, index int) (string, error) {
	parts := strings.Split(r.URL.Path, "/")

	if index < 0 {
		return "", fmt.Errorf("invalid index %d: must be non-negative", index)
	}

	if len(parts) <= index {
		return "", fmt.Errorf("invalid URL path %q: expected at least %d segments, got %d", r.URL.Path, index+1, len(parts))
	}

	val := parts[index]
	if val == "" {
		return "", fmt.Errorf("empty path value at index %d in URL %q", index, r.URL.Path)
	}

	return val, nil
}

// requireProfile ensures the request is from an authenticated user.
// It returns the profile or writes an error and returns nil if auth fails.
func requireProfile(w http.ResponseWriter, r *http.Request) *gauth.Profile {
	p, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		writeHttpError(w, http.StatusUnauthorized, "user could not be authenticated: %v", err)
		return nil
	}
	return p
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
