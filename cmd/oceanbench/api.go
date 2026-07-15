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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/utils/nmea"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type minimalSite struct {
	Skey, Perm int64
	Name       string
	Public     bool
}

// setupAPIRoutes registers all Fiber handlers for API endpoints.
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
//	/api/get/site/*id            → Get site by key
//	/api/get/devices/site        → Get devices for the current profile's site
//	/api/get/profile/data        → Get current user's profile data
//	/api/set/site/*data          → Set profile site data
//
// Only some endpoints require authentication. These checks are handled per handler.
// All routes under /api automatically go through the wrapAPI middleware.
func setupAPIRoutes(app *fiber.App) {
	// Create /api group; wrapAPI middleware is applied to every route in the group.
	api := app.Group("/api", wrapAPI)

	// TODO: convert these handlers to fiber handlers instead of just adapting them.
	// New handlers should be fiber handlers.
	api.Get("/get/site/*", getSiteHandler)
	api.Get("/get/devices/site", getDevicesForSiteHandler)
	api.Get("/get/sites/all", getAllSitesHandler)
	api.Get("/get/sites/public", getPublicSitesHandler)
	api.Get("/get/sites/user", getUserSitesHandler)
	api.Get("/get/profile/data", getProfileDataHandler)
	api.Get("/get/profile/tv-overview-config", getProfileTVConfigHandler)
	api.Get("/get/broadcast/config", getBroadcastConfigHandler)
	api.Get("/get/vars/site", getVarsForSiteHandler)
	api.Get("/get/sensor/data/*", getSensorDataHandler)
	api.Get("/get/gpstrail/*", getGPSTrailHandler)

	api.All("/set/site/*", setSiteHandler)
	api.All("/set/log/*", setLogHandler)

	api.All("/test/upload/*", testUploadHandler)
	api.All("/test/download/*", testDownloadHandler)

	// TODO: change these to the form /api/get/scalar and /api/set/scalar.
	api.All("/scalar/put/*", scalarPutHandler)
	api.All("/scalar/get/*", scalarGetHandler)

	setupAPIV1Routes(api)
}

// wrapAPI is Fiber middleware that logs every API request.
func wrapAPI(c *fiber.Ctx) error {
	path := string(c.Request().URI().Path())
	query := string(c.Request().URI().QueryString())
	if query == "" {
		log.Println(path)
	} else {
		log.Println(path + "?" + query)
	}
	return c.Next()
}

func getSiteHandler(c *fiber.Ctx) error {
	// Require authentication.
	if requireProfile(c) == nil {
		return nil
	}

	// Get site key from URL path.
	val, err := getPathValue(c, 4)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": err.Error()})
	}

	skey, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("could not parse site key: %v", err)})
	}

	site, err := model.GetSite(c.UserContext(), settingsStore, skey)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("could not get site with site key: %d: %v", skey, err)})
	}

	enc := site.Encode()
	_, err = fmt.Fprint(c, string(enc))
	return err
}

func getDevicesForSiteHandler(c *fiber.Ctx) error {
	p := requireProfile(c)
	if p == nil {
		return nil
	}

	parts := strings.Split(p.Data, ":")
	if len(parts) != 2 {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": "no site data in profile"})
	}
	skey, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("invalid site key in profile data: %s", p.Data)})
	}

	user, err := model.GetUser(c.UserContext(), settingsStore, skey, p.Email)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to get user: %v", err)})
	}
	if user.Perm&model.ReadPermission == 0 {
		c.Status(fiber.StatusUnauthorized)
		return c.JSON(fiber.Map{"error": "profile does not have read permissions"})
	}

	devices, err := model.GetDevicesBySite(c.UserContext(), settingsStore, skey)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to get devices by site: %v", err)})
	}

	data, err := json.Marshal(devices)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to marshal devices: %v", err)})
	}
	_, err = c.Write(data)
	return err
}

func getAllSitesHandler(c *fiber.Ctx) error {
	// Require authentication.
	if requireProfile(c) == nil {
		return nil
	}

	sites, err := model.GetAllSites(c.UserContext(), settingsStore)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("could not get all sites: %v", err)})
	}

	var s []string
	for _, site := range sites {
		s = append(s, strconv.Itoa(int(site.Skey))+":\""+site.Name+"\"")
	}

	output := "{" + strings.Join(s, ",") + "}"
	_, err = c.WriteString(output)
	return err
}

func getPublicSitesHandler(c *fiber.Ctx) error {
	// Require authentication.
	if requireProfile(c) == nil {
		return nil
	}

	sites, err := model.GetAllSites(c.UserContext(), settingsStore)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("could not get public sites: %v", err)})
	}

	var s []string
	for _, site := range sites {
		if site.Public {
			s = append(s, strconv.Itoa(int(site.Skey))+":\""+site.Name+"\"")
		}
	}

	output := "{" + strings.Join(s, ",") + "}"
	_, err = c.WriteString(output)
	return err
}

func getUserSitesHandler(c *fiber.Ctx) error {
	p := requireProfile(c)
	if p == nil {
		return nil
	}

	users, sites, err := model.GetUserSites(c.UserContext(), settingsStore, p.Email)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to get sites for user: %v. err: %v", p.Email, err)})
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
	return c.JSON(userSites)
}

func getProfileDataHandler(c *fiber.Ctx) error {
	p := requireProfile(c)
	if p == nil {
		return nil
	}

	_, err := c.WriteString(p.Data)
	return err
}

func getProfileTVConfigHandler(c *fiber.Ctx) error {
	p := requireProfile(c)
	if p == nil {
		return nil
	}

	// The request must be for a superadmin.
	if !isSuperAdmin(p.Email) {
		c.Status(fiber.StatusUnauthorized)
		return c.JSON(fiber.Map{"error": "only available to superadmins"})
	}

	// The variable scope of the config is the username of the email
	// with fullstops replaced.
	scope := strings.ReplaceAll(strings.Split(p.Email, "@")[0], ".", "")

	ctx := c.UserContext()
	existingCfg := true
	configVar, err := model.GetVariable(ctx, settingsStore, 0, scope+".tvOverviewConfig")
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		// The user doesn't yet have a configuration, so we will need to make a new one.
		existingCfg = false
	} else if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": "unable to get config"})
	}

	if !existingCfg {
		log.Println("no existing tv overview config found, creating blank config")
		err = model.PutVariable(ctx, settingsStore, 0, scope+".tvOverviewConfig", "{}")
		if err != nil {
			c.Status(fiber.StatusInternalServerError)
			return c.JSON(fiber.Map{"error": "unable to put blank config"})
		}
		configVar = &model.Variable{Name: scope + ".tvOverviewConfig", Value: "{}"}
	}

	_, err = c.WriteString(configVar.Value)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": "unable to write config to response"})
	}
	return nil
}

// getBroadcastConfigHandler handles requests to get the broadcast config for a given broadcast.
// The requester must have admin access to the site on which the broadcast configuration belongs.
//
// The API is expecting the following structure:
//
//	/api/get/broadcast/config
//
// With the following query parameters:
//
//	id: uuid of the broadcast to get
func getBroadcastConfigHandler(c *fiber.Ctx) error {
	p := requireProfile(c)
	if p == nil {
		return nil
	}

	id := c.FormValue("id")
	if id == "" {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": "api/get/broadcast/config got empty id query parameter"})
	}
	if err := uuid.Validate(id); err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("invalid id: %v", err)})
	}

	ctx := c.UserContext()
	broadcastVar, err := model.GetBroadcastVarByUUID(ctx, settingsStore, id)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to get broadcast with UUID (%s): %v", id, err)})
	}

	// Check that the user has admin privileges to the site the broadcast lives on.
	if !isAdmin(ctx, broadcastVar.Skey, p.Email) {
		c.Status(fiber.StatusUnauthorized)
		return c.JSON(fiber.Map{"error": "user does not have admin privileges"})
	}

	_, err = c.Write([]byte(broadcastVar.Value))
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to write broadcast config to response: %v", err)})
	}
	return nil
}

func getVarsForSiteHandler(c *fiber.Ctx) error {
	p := requireProfile(c)
	if p == nil {
		return nil
	}

	// Get site key from profile data.
	parts := strings.Split(p.Data, ":")
	if len(parts) != 2 {
		c.Status(fiber.StatusUnauthorized)
		return c.JSON(fiber.Map{"error": "no site data in profile"})
	}
	skey, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("invalid site key in profile data: %s", p.Data)})
	}

	// Check for read permission.
	user, err := model.GetUser(c.UserContext(), settingsStore, skey, p.Email)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to get user: %v", err)})
	}
	if user.Perm&model.ReadPermission == 0 {
		c.Status(fiber.StatusUnauthorized)
		return c.JSON(fiber.Map{"error": "profile does not have read permissions"})
	}

	// Get variables for the site.
	siteVars, err := model.GetVariablesBySite(c.UserContext(), settingsStore, skey, "")
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to get variables by site: %v", err)})
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

	return c.JSON(filtered)
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
func getSensorDataHandler(c *fiber.Ctx) error {
	mac := model.MacEncode(c.FormValue("ma"))
	if mac == 0 {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("invalid MAC supplied, wanted in form XX:XX:XX:XX:XX:XX, got: %s", c.FormValue("ma"))})
	}

	pin := c.FormValue("pn")

	ctx := context.Background()
	sensor, err := model.GetSensorV2(ctx, settingsStore, mac, pin)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to get sensor: %v", err)})
	}

	start, err := strconv.ParseInt(c.FormValue("start"), 10, 64)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to parse start time as unix timestamp: %v", err)})
	}

	finish, err := strconv.ParseInt(c.FormValue("finish"), 10, 64)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to parse finish time as unix timestamp: %v", err)})
	}

	if start > finish {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": "start time must be before finish time"})
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
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to get scalars for sensor: %v", err)})
	}

	type timedValue struct {
		Value     float64
		Timestamp int64
	}

	var output []timedValue
	for _, s := range scalars {
		transformed, err := sensor.Transform(s.Value)
		if err != nil {
			c.Status(fiber.StatusInternalServerError)
			return c.JSON(fiber.Map{"error": fmt.Sprintf("error whilst transforming scalar value: %v", err)})
		}
		output = append(output, timedValue{Value: transformed, Timestamp: s.Timestamp})
	}

	// Allow scripts to access this data.
	// TODO: restrict based on needs and authentication.
	c.Set("Access-Control-Allow-Origin", "*")

	return c.JSON(output)
}

// getGPSTrailHandler returns recent GPS points for a device/pin from Text storage (raw NMEA).
// Endpoint: GET /api/get/gpstrail/?ma=<MAC>&pn=<pin>&start=<unix>&finish=<unix>
func getGPSTrailHandler(c *fiber.Ctx) error {
	ctx := context.Background()

	macStr := c.FormValue("ma")
	mac := model.MacEncode(macStr)
	if mac == 0 {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("invalid MAC supplied, wanted in form XX:XX:XX:XX:XX:XX, got: %s", macStr)})
	}

	// Parse pin (default T1).
	pin := c.FormValue("pn")
	if pin == "" {
		pin = "T1"
	}

	var startTS, finishTS int64
	if sv := c.FormValue("start"); sv != "" {
		v, err := strconv.ParseInt(sv, 10, 64)
		if err != nil {
			c.Status(fiber.StatusBadRequest)
			return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to parse start time as unix timestamp: %v", err)})
		}
		startTS = v
	}
	if fv := c.FormValue("finish"); fv != "" {
		v, err := strconv.ParseInt(fv, 10, 64)
		if err != nil {
			c.Status(fiber.StatusBadRequest)
			return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to parse finish time as unix timestamp: %v", err)})
		}
		finishTS = v
	}
	if startTS != 0 && finishTS != 0 && startTS > finishTS {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": "start time must be before finish time"})
	}

	// Build MID and fetch texts.
	mid := model.ToMID(model.MacDecode(mac), pin)

	texts, err := model.GetText(ctx, mediaStore, mid, []int64{startTS, finishTS})
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("unable to get text for gps: %v", err)})
	}

	type gpsOut struct {
		Lat  float64 `json:"lat"`
		Lon  float64 `json:"lon"`
		Time string  `json:"time"` // RFC3339 UTC
		TS   int64   `json:"ts"`   // unix seconds
	}

	points := make([]gpsOut, 0, len(texts))
	for _, t := range texts {
		fix, err := nmea.Parse(t.Data, t.Date.UTC())
		if err != nil || !fix.Valid || (fix.Lat == 0 && fix.Lon == 0) {
			continue
		}
		points = append(points, gpsOut{
			Lat:  fix.Lat,
			Lon:  fix.Lon,
			Time: fix.Time.Format(time.RFC3339),
			TS:   fix.Time.Unix(),
		})

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

	c.Set("Access-Control-Allow-Origin", "*")
	return c.JSON(resp)
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
func setSiteHandler(c *fiber.Ctx) error {
	p := requireProfile(c)
	if p == nil {
		return nil
	}

	// Get site data from path.
	val, err := getPathValue(c, 4) // /api/set/site/<sitekey>:<sitename>
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": err.Error()})
	}

	// Validate format: <sitekey>:<sitename>
	parts := strings.SplitN(val, ":", 2)
	if len(parts) != 2 {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": "invalid site data, wanted: <sitekey>:<sitename>"})
	}

	// Validate site key.
	if _, err := strconv.ParseInt(parts[0], 10, 64); err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("could not parse site key from /api/set/site/<sitekey>:<sitename> : %v", err)})
	}

	// Update profile.
	if err := putProfileDataFiber(c, val); err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("could not update profile data with site data: %v", err)})
	}

	_, err = c.WriteString("OK")
	return err
}

// testUploadHandler handles test upload requests.
// Reads exactly <value> bytes from the request body and responds "OK".
// Used for testing upload throughput or validation.
func testUploadHandler(c *fiber.Ctx) error {
	// Get byte count from path.
	val, err := getPathValue(c, 4) // /api/test/upload/<value>
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": err.Error()})
	}

	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("could not parse value from /api/test/upload/<value>: %v", err)})
	}

	body := make([]byte, n)
	buf := bytes.NewBuffer(c.Body())
	_, err = io.ReadFull(buf, body)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		writeErrorFiber(c, errInvalidBody)
		return nil
	}

	_, err = c.WriteString("OK")
	return err
}

// testDownloadHandler handles test download requests.
//
// Expected path format:
//
//	/api/test/download/<n>[/<chunk>]
//
// Responds with <n> bytes of random data, optionally sent in chunks of size <chunk>.
// Used for testing download speed and behavior.
func testDownloadHandler(c *fiber.Ctx) error {
	req := strings.Split(c.Path(), "/")
	if len(req) < 5 {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": "invalid length of url path"})
	}

	n, err := strconv.ParseInt(req[4], 10, 64)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("could not parse value from /api/test/download/<n>: %v", err)})
	}

	chunk := n // Default: whole payload in one write
	if len(req) == 6 {
		chunk, err = strconv.ParseInt(req[5], 10, 64)
		if err != nil {
			c.Status(fiber.StatusBadRequest)
			return c.JSON(fiber.Map{"error": fmt.Sprintf("could not parse chunk size from url: %v", err)})
		}
	}

	body := make([]byte, n)
	_, err = rand.Read(body)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("could not generate random data: %v", err)})
	}

	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Disposition", "attachment; filename=\""+req[4]+"\"")

	var i int64
	for i = 0; i < n; i += chunk {
		end := i + chunk
		if end > n {
			end = n
		}
		c.Write(body[i:end])
	}

	return nil
}

// scalarPutHandler handles scalar data ingestion.
//
// Expected path format:
//
//	/api/scalar/put/<id>,<timestamp>,<value>
//
// Parses and stores a single scalar. No authentication required.
func scalarPutHandler(c *fiber.Ctx) error {
	val, err := getPathValue(c, 4) // /api/scalar/put/<id>,<timestamp>,<value>
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": err.Error()})
	}

	args, err := splitNumbers(val)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("invalid arg: %v", err)})
	}
	if len(args) != 3 {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": "invalid number of args"})
	}

	err = model.PutScalar(c.UserContext(), mediaStore, &model.Scalar{
		ID:        args[0],
		Timestamp: args[1],
		Value:     float64(args[2]),
	})
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("could not put scalar: %v", err)})
	}
	return nil
}

// scalarGetHandler handles scalar data retrieval.
//
// Expected path format:
//
//	/api/scalar/get/<id>,<start>,<end>
//
// Returns a JSON-encoded array of scalars for the given ID and time range.
// No authentication required.
func scalarGetHandler(c *fiber.Ctx) error {
	val, err := getPathValue(c, 4) // /api/scalar/get/<id>,<start>,<end>
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": err.Error()})
	}

	args, err := splitNumbers(val)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("invalid arg: %v", err)})
	}
	if len(args) != 3 {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": "invalid number of args"})
	}

	scalars, err := model.GetScalars(c.UserContext(), mediaStore, args[0], []int64{args[1], args[2]})
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("could not get scalar: %v", err)})
	}

	data, err := json.Marshal(scalars)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("error marshaling scalars: %v", err)})
	}
	_, err = c.Write(data)
	return err
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
func getPathValue(c *fiber.Ctx, index int) (string, error) {
	parts := strings.Split(c.Path(), "/")

	if index < 0 {
		return "", fmt.Errorf("invalid index %d: must be non-negative", index)
	}

	if len(parts) <= index {
		return "", fmt.Errorf("invalid URL path %q: expected at least %d segments, got %d", c.Path(), index+1, len(parts))
	}

	val := parts[index]
	if val == "" {
		return "", fmt.Errorf("empty path value at index %d in URL %q", index, c.Path())
	}

	return val, nil
}

// requireProfile ensures the request is from an authenticated user.
// It returns the profile or writes an error and returns nil if auth fails.
func requireProfile(c *fiber.Ctx) *gauth.Profile {
	p, err := getProfileFiber(c)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		c.Status(fiber.StatusUnauthorized)
		c.JSON(fiber.Map{"error": fmt.Errorf("user could not be authenticated: %v", err)})
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
