/*
NAME
  Ocean Bench - a cloud service for analyzing ocean data.

AUTHORS
  Alan Noble <alan@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2018-2026 the Australian Ocean Lab (AusOcean)

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
  in gpl.txt.  If not, see <http://www.gnu.org/licenses/>.
*/

// Ocean Bench is a cloud service for analyzing ocean data.
//
// Data can be accessed and played.
//
//	/search to search for any data.
//	/play to play audio or video daa
//
// Ocean Bench can also be run in standalone mode without App Engine:
//
//	./oceanbench -standalone
//
// Other command-line flags available in standalone mode:
//
//	[-debug]        enables verbose output for debugging.
//	[-host string]  host name we're running on (localhost by default).
//	[-port int]     host port we're listening on (8080 by default).
//	[-gps string]   GPS receiver serial port and enables GPS mode, e.g. COM4 or /dev/ttyUSB.
//	[-baudRate int] serial device baud rate (9600 by default).
//	[-loc string]   latitude,longitude of the GPS receiver in decimal degrees format.
//	[-alt float]    altitude of the GPS receiver. Negative numbers signify depths (0 by default).
//
// The PORT environment variable can be used to set the default port number.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/ausocean/cloud/backend"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/utils/sliceutils"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
)

const (
	version     = "v0.36.4"
	localSite   = "localhost"
	localDevice = "localdevice"
	localEmail  = "localuser@localhost"
)

const (
	projectID          = "oceanbench"
	oauthClientID      = "802166617157-v67emnahdpvfuc13ijiqb7qm3a7sf45b.apps.googleusercontent.com"
	oauthMaxAge        = 60 * 60 * 24 * 7 // 7 days
	tvServiceURL       = "https://tv.cloudblue.org"
	cronServiceURL     = "https://cron.cloudblue.org"
	cronServiceAccount = "oceancron@appspot.gserviceaccount.com"
)

// page defines one page of the web app.
type page struct {
	Name     string
	URL      string
	Level    int
	Selected bool
	Group    bool
	Perm     int
}

// commonData defines the commonly used template data.
type commonData struct {
	Standalone bool
	Debug      bool
	Version    string
	Msg        string
	Pages      []page
	PageData   interface{}
	Profile    *gauth.Profile
	SuperAdmin bool
	LoginURL   string
	LogoutURL  string
	Users      []model.User
	Footer     template.HTML
}

// TestDataConfig defines the structure for injecting standalone datastore test configurations via JSON.
type TestDataConfig struct {
	Sites    []*model.Site     `json:"sites"`
	Users    []*model.User     `json:"users"`
	Devices  []*model.Device   `json:"devices"`
	MtsMedia []*model.MtsMedia `json:"mtsmedia"`
}

var (
	setupMutex    sync.Mutex
	templates     *template.Template
	setTemplates  *template.Template
	dataHost      = "https://bench.cloudblue.org"
	mediaStore    datastore.Store
	settingsStore datastore.Store
	debug         bool
	standalone    bool
	auth          *gauth.UserAuth
	tvURL         = tvServiceURL
	storePath     string
	testDataFile  string
)

var (
	errInvalidBody = errors.New("invalid body")
	errInvalidJSON = errors.New("invalid JSON")
)

var (
	cronScheduler proxyScheduler
	cronSecret    []byte
)

// templateFuncs defines custom template functions.
var templateFuncs = template.FuncMap{
	"macdecode":     model.MacDecode,
	"split":         strings.Split,
	"part":          sliceutils.StringPart,
	"float":         parseFloat,
	"localdate":     formatLocalDate,
	"localtime":     formatLocalTime,
	"localdatetime": formatLocalDateTime,
	"json":          toJSON,
}

func main() {
	defaultPort := 8080
	v := os.Getenv("PORT")
	if v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			defaultPort = i
		}
	}
	exp := strings.Split(os.Getenv("OCEANBENCH_EXPERIMENTS"), ",")
	if ok, i := sliceutils.ContainsStringPrefix(exp, "DATA_HOST="); ok {
		v := exp[i][len("DATA_HOST="):]
		log.Printf("Experiment DATA_HOST enabled: %s", v)
		dataHost = v
	}

	var alt float64
	var baud int
	var gps string
	var host string
	var loc string
	var port int
	var cronURL string
	flag.BoolVar(&debug, "debug", false, "Run in debug mode.")
	flag.BoolVar(&standalone, "standalone", false, "Run in standalone mode.")
	flag.Float64Var(&alt, "alt", 0, "Altitude (negative for depth)")
	flag.IntVar(&baud, "baud", 9600, "Baud rate of GPS receiver")
	flag.StringVar(&gps, "gps", "", "GPS receiver serial port, e.g., /dev/ttyUSB")
	flag.StringVar(&host, "host", "localhost", "Host we run on in standalone mode")
	flag.StringVar(&loc, "loc", "", "Latitude,longitude pair in decimal degrees.")
	flag.IntVar(&port, "port", defaultPort, "Port we listen on in standalone mode")
	flag.StringVar(&cronURL, "cronurl", cronServiceURL, "Cron service URL")
	flag.StringVar(&tvURL, "tvurl", tvServiceURL, "TV service URL")
	flag.StringVar(&storePath, "filestore", "store", "File store path")
	flag.StringVar(&testDataFile, "testdata", "", "Path to a JSON file to populate the datastore")
	flag.Parse()

	// Perform one-time setup or bail.
	ctx := context.Background()
	setup(ctx)

	// Build the Fiber application.
	app := fiber.New()

	// Serve static files from the "s" directory.
	app.Static("/s", "./s")
	// Except for favicon.ico.
	app.Get("/favicon.ico", adaptor.HTTPHandlerFunc(faviconHandler))

	// Get shared cronSecret.
	var err error
	cronSecret, err = gauth.GetHexSecret(ctx, "oceancron", "cronSecret")
	if err != nil {
		log.Printf("could not get cronSecret: %v", err)
	}

	// Warmup handler.
	app.Get("/_ah/warmup", func(c *fiber.Ctx) error {
		log.Println("warmup request received, version: " + version)
		return nil
	})

	// User requests.
	// TODO: convert these handlers to fiber handlers instead of just adapting them.
	// New handlers should be fiber handlers.
	app.All("/search", adaptor.HTTPHandlerFunc(searchHandler))
	app.All("/play/audiorequest", adaptor.HTTPHandlerFunc(filterHandler))
	app.All("/play", adaptor.HTTPHandlerFunc(playHandler))
	app.All("/learn/mooring", adaptor.HTTPHandlerFunc(mooringHandler))
	app.All("/upload", adaptor.HTTPHandlerFunc(uploadHandler))
	app.All("/set/devices/edit/var", adaptor.HTTPHandlerFunc(editVarHandler))
	app.All("/set/devices/edit/sensor", adaptor.HTTPHandlerFunc(editSensorHandler))
	app.All("/set/devices/edit/actuator", adaptor.HTTPHandlerFunc(editActuatorHandler))
	app.All("/set/devices/edit/calibrate", adaptor.HTTPHandlerFunc(calibrateDevicesHandler))
	app.All("/set/devices/edit", adaptor.HTTPHandlerFunc(editDevicesHandler))
	app.All("/set/devices/vars", adaptor.HTTPHandlerFunc(setDevicesVars))
	app.All("/set/devices/*", adaptor.HTTPHandlerFunc(setDevicesHandler))
	app.All("/set/crons/edit", adaptor.HTTPHandlerFunc(editCronsHandler))
	app.All("/set/crons/*", adaptor.HTTPHandlerFunc(setCronsHandler))
	app.All("/get", adaptor.HTTPHandlerFunc(getHandler))
	app.All("/test/*", adaptor.HTTPHandlerFunc(testHandler))
	app.All("/login", adaptor.HTTPHandlerFunc(loginHandler))
	app.All("/logout", adaptor.HTTPHandlerFunc(logoutHandler))
	app.All("/oauth2callback", adaptor.HTTPHandlerFunc(oauthCallbackHandler))
	app.All("/live/*", adaptor.HTTPHandlerFunc(liveHandler))
	app.All("/monitor", adaptor.HTTPHandlerFunc(monitorHandler))
	app.All("/admin/site/add", adaptor.HTTPHandlerFunc(adminHandler))
	app.All("/admin/site/update", adaptor.HTTPHandlerFunc(adminHandler))
	app.All("/admin/site/delete", adaptor.HTTPHandlerFunc(adminHandler))
	app.All("/admin/user/add", adaptor.HTTPHandlerFunc(adminHandler))
	app.All("/admin/user/update", adaptor.HTTPHandlerFunc(adminHandler))
	app.All("/admin/user/delete", adaptor.HTTPHandlerFunc(adminHandler))
	app.All("/admin/site", adaptor.HTTPHandlerFunc(adminHandler))
	app.All("/admin/broadcast", adaptor.HTTPHandlerFunc(adminHandler))
	app.All("/admin/tv-overview", adaptor.HTTPHandlerFunc(tvOverviewHandler))
	app.All("/admin/missioncontrol", adaptor.HTTPHandlerFunc(adminHandler))
	app.All("/admin/mediamanager", adaptor.HTTPHandlerFunc(adminHandler))
	app.All("/admin/sandbox/configure", adaptor.HTTPHandlerFunc(configDevicesHandler))
	app.All("/admin/sandbox", adaptor.HTTPHandlerFunc(sandboxHandler))
	app.All("/admin/utils", adaptor.HTTPHandlerFunc(adminHandler))
	app.All("/data/*", adaptor.HTTPHandlerFunc(dataHandler))
	app.All("/throughputs", adaptor.HTTPHandlerFunc(throughputsHandler))
	app.All("/logs", adaptor.HTTPHandlerFunc(logPageHandler))
	app.All("/", adaptor.HTTPHandlerFunc(indexHandler))

	// Setup routes for the API, ie. /api requests.
	setupAPIRoutes(app)

	if standalone {
		// Location and GPS only apply in standalone mode.
		if loc != "" {
			latlng := strings.Split(loc, ",")
			if len(latlng) < 2 {
				log.Fatal("Invalid location")
			}
			lat, err := strconv.ParseFloat(latlng[0], 64)
			if err != nil {
				log.Fatal("Invalid latitude")
			}
			lng, err := strconv.ParseFloat(latlng[1], 64)
			if err != nil {
				log.Fatal("Invalid longitude")
			}
			setLocation(lat, lng, alt)
		}
		if gps != "" {
			// Poll for NMEA GPS messages.
			go pollGPS(gps, baud, alt)
		}
		dataHost = "http://" + host + ":" + strconv.Itoa(port)

	} else {
		log.Printf("Initializing OAuth2")
		auth = &gauth.UserAuth{ProjectID: projectID, ClientID: oauthClientID, MaxAge: oauthMaxAge}
		auth.Init(backend.NewNetHandler(nil, nil, nil))
		host = "" // Host is determined by App Engine.
	}

	cronScheduler = proxyScheduler{url: cronURL}
	log.Printf("Listening on %s:%d", host, port)
	log.Printf("Sending cron requests to %s", cronURL)
	log.Printf("Sending TV requests to %s", tvURL)
	log.Fatal(app.Listen(fmt.Sprintf("%s:%d", host, port)))
}

// setup executes per-instance one-time warmup and is used to
// initialize datastores.
func setup(ctx context.Context) {
	setupMutex.Lock()
	defer setupMutex.Unlock()

	if mediaStore != nil {
		return
	}

	var err error
	settingsStore, mediaStore, err = model.SetupDatastore(standalone, storePath, ctx)
	if err == nil && standalone {
		err = setupLocal(ctx, settingsStore)
	}
	if err != nil {
		log.Fatalf("could not set up datastore: %v", err)
	}

	templateDir := "cmd/oceanbench/t"
	if standalone || os.Getenv("GAE_ENV") == "" {
		templateDir = "t"
	}
	templates, err = template.New("").Funcs(templateFuncs).ParseGlob(templateDir + "/*.html")
	if err != nil {
		log.Fatalf("error parsing templates: %v", err)
	}
	setTemplates, err = template.New("").Funcs(templateFuncs).ParseGlob(templateDir + "/set/*.html")
	if err != nil {
		log.Fatalf("error parsing set templates: %v", err)
	}
}

// setupLocal creates a local site, user and device for use in standalone mode.
// In standalone mode all data is associated with site 1.
func setupLocal(ctx context.Context, store datastore.Store) error {
	if testDataFile != "" {
		data, err := os.ReadFile(testDataFile)
		if err != nil {
			return fmt.Errorf("could not read testdata file: %w", err)
		}
		var config TestDataConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("could not parse testdata file: %w", err)
		}

		for _, site := range config.Sites {
			if err := model.PutSite(ctx, store, site); err != nil {
				return err
			}
		}
		for _, user := range config.Users {
			if err := model.PutUser(ctx, store, user); err != nil {
				return err
			}
		}
		for _, device := range config.Devices {
			if err := model.PutDevice(ctx, store, device); err != nil {
				return err
			}
		}
		for _, media := range config.MtsMedia {
			key := mediaStore.IDKey("MtsMedia", datastore.IDKey(media.MID, media.Timestamp, 0))
			if _, err := mediaStore.Put(ctx, key, media); err != nil {
				return fmt.Errorf("could not put MtsMedia: %w", err)
			}
		}
		// Set the active site to the first site in the config so adminHandler
		// can resolve a valid skey from standaloneData.
		if len(config.Sites) > 0 {
			first := config.Sites[0]
			standaloneData = strconv.FormatInt(first.Skey, 10) + ":" + first.Name
		}
		log.Printf("Successfully populated datastore from %s", testDataFile)
		return nil
	}

	standaloneData = "1:" + localSite
	err := model.PutSite(ctx, store, &model.Site{Skey: 1, Name: localSite, Enabled: true})
	if err != nil {
		return err
	}
	err = model.PutUser(ctx, store, &model.User{Skey: 1, Email: localEmail, Perm: model.ReadPermission | model.WritePermission | model.AdminPermission})
	if err != nil {
		return err
	}
	err = model.PutDevice(ctx, store, &model.Device{Skey: 1, Mac: 1, Dkey: 0, Name: localDevice, Inputs: "A0,V0,S0", MonitorPeriod: 60, Enabled: true})

	return err
}

// faviconHandler serves favicon.ico.
func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "favicon.ico")
}

// indexHandler handles requests for the home page and unimplemented pages.
// Signed-in users are presented with a list of their sites.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	if r.URL.Path != "/" {
		// Redirect all invalid URLs to the root homepage.
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	profile, err := getProfile(w, r)
	data := commonData{
		Pages:   pages("home"),
		Profile: profile,
	}
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		writeTemplate(w, r, "index.html", &data, "")
		return
	}

	writeTemplate(w, r, "index.html", &data, "")
}

// warmupHandler handles warmup requests. It is a no-op that simply ensures that the intance is loaded.
func warmupHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	w.Write([]byte{})
}

// getHandler handles media and text requests, depending on the pin type.
// Requires read permission for the requested media, otherwise permission is denied.
// The user need not be logged in to access public sites.
// When no output is specified, media data is downloaded to the client.
func getHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	p, _ := getProfile(w, r) // Ignore errors, since users need not be logged in.

	q := r.URL.Query()
	id := q.Get("id")
	mid, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		writeError(w, errInvalidMID)
		return
	}

	t := q.Get("ts")
	var ts []int64
	if t != "" {
		ts, err = splitTimestamps(t, false)
		if err != nil {
			writeError(w, errInvalidTimestamp)
			return
		}
	}

	k := q.Get("ky")
	var ky []uint64
	if k != "" {
		ky, err = splitUints(k)
		if err != nil {
			writeError(w, errInvalidKey)
			return
		}
	}

	ctx := r.Context()
	setup(ctx)

	ok, err := hasPermission(ctx, p, mid, model.ReadPermission)
	if err != nil {
		writeError(w, err)
		return
	}
	if !ok {
		writeError(w, errPermissionDenied)
		return
	}

	var content []byte
	var mime, name string

	_, pin := model.FromMID(mid)
	switch pin[0] {
	case 'V', 'S':
		content, mime, err = getMedia(w, r, mid, ts, ky)
		if err != nil {
			writeError(w, fmt.Errorf("could not get media: %w", err))
			return
		}

		if mime == "video/mp2t" {
			name = "media.ts" // Could contain video or audio.
			break
		}

		split := strings.Split(mime, "/")
		if len(split) > 1 {
			name = split[0] + "." + split[1]
			break
		}

		name = split[0] + "." + split[0]

	case 'T':
		content, mime, err = getText(r, mid, ts, ky)
		if err != nil {
			writeError(w, fmt.Errorf("could not get text: %w", err))
			return
		}

		if mime == "application/json" {
			name = "data.json"
			break
		}

		name = "data.txt"

	default:
		writeError(w, fmt.Errorf("unknown pin type: %v", pin[0]))
	}

	writeData(w, content, mime, name)
}

// hasPermission returns true if the user has the requested media
// permission or false otherwise. This requires, first, looking up the
// device associated with the media and, second, looking up its
// site. All users have access to public sites. For private sites, the
// user must be logged in and have a user record with the requested
// permission.
func hasPermission(ctx context.Context, p *gauth.Profile, mid, perm int64) (bool, error) {
	if standalone {
		return true, nil
	}
	ma, _ := model.FromMID(mid)
	dev, err := model.GetDevice(ctx, settingsStore, model.MacEncode(ma))
	if err != nil {
		if err != datastore.ErrNoSuchEntity {
			return false, fmt.Errorf("error getting device: %w", err)
		}
		return false, nil
	}
	site, err := model.GetSite(ctx, settingsStore, dev.Skey)
	if err != nil {
		return false, fmt.Errorf("error getting site: %w", err)
	}
	if site.Public {
		return perm == model.ReadPermission, nil
	}
	if p == nil {
		return false, nil // User not logged in.
	}
	user, err := model.GetUser(ctx, settingsStore, dev.Skey, p.Email)
	if err != nil {
		return false, fmt.Errorf("error getting user: %w", err)
	}
	return perm&user.Perm != 0, nil
}

// writeTemplate writes the given template with the supplied data,
// populating some common properties.
func writeTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}, msg string) {
	profile, _ := getProfile(w, r)
	v := reflect.Indirect(reflect.ValueOf(data))
	p := v.FieldByName("Standalone")
	if p.IsValid() {
		p.SetBool(standalone)
	}
	p = v.FieldByName("Debug")
	if p.IsValid() {
		p.SetBool(debug)
	}
	p = v.FieldByName("Version")
	if p.IsValid() {
		p.SetString(version)
	}
	p = v.FieldByName("Msg")
	if p.IsValid() {
		p.SetString(msg)
	}
	p = v.FieldByName("Profile")
	if p.IsValid() {
		p.Set(reflect.ValueOf(profile))
	}
	p = v.FieldByName("SuperAdmin")
	if p.IsValid() {
		if profile == nil {
			p.SetBool(false)
		} else {
			p.SetBool(isSuperAdmin(profile.Email))
		}
	}
	p = v.FieldByName("LoginURL")
	if p.IsValid() {
		p.Set(reflect.ValueOf("/login?redirect=" + r.URL.RequestURI()))
	}
	p = v.FieldByName("LogoutURL")
	if p.IsValid() {
		p.Set(reflect.ValueOf("/logout?redirect=" + r.URL.RequestURI()))
	}

	const footer = "footer.html"
	var b bytes.Buffer
	err := templates.ExecuteTemplate(&b, footer, data)
	if err != nil {
		log.Fatalf("ExecuteTemplate failed on %s: %v", footer, err)
	}
	p = v.FieldByName("Footer")
	p.Set(reflect.ValueOf(template.HTML(b.String())))

	if strings.HasPrefix(name, "set/") {
		err = setTemplates.ExecuteTemplate(w, name[4:], data)
	} else {
		err = templates.ExecuteTemplate(w, name, data)
	}
	if err != nil {
		log.Fatalf("ExecuteTemplate failed on %s: %v", name, err)
	}
}

// pages returns a copy of the app's pages, selecting the one that matches selected.
func pages(selected string) []page {
	pages := []page{
		{
			Name: "home",
			URL:  "/",
			Perm: model.ReadPermission,
		},
		{
			Name: "search",
			URL:  "/search",
			Perm: model.ReadPermission,
		},
		{
			Name: "monitor",
			URL:  "/monitor",
			Perm: model.ReadPermission,
		},
		{
			Name: "play",
			URL:  "/play",
			Perm: model.ReadPermission,
		},
		{
			Name: "upload",
			URL:  "/upload",
			Perm: model.WritePermission,
		},
		{
			Name:  "settings",
			Group: true,
			Perm:  model.WritePermission,
		},
		{
			Name:  "devices",
			URL:   "/set/devices",
			Level: 1,
			Perm:  model.WritePermission,
		},
		{
			Name:  "crons",
			URL:   "/set/crons",
			Level: 1,
			Perm:  model.WritePermission,
		},
		{
			Name:  "admin",
			Group: true,
			Perm:  model.AdminPermission,
		},
		{
			Name:  "site",
			URL:   "/admin/site",
			Level: 1,
			Perm:  model.AdminPermission,
		},
		{
			Name:  "broadcast",
			URL:   "/admin/broadcast",
			Level: 1,
			Perm:  model.AdminPermission,
		},
		{
			Name:  "mission control",
			URL:   "/admin/missioncontrol",
			Level: 1,
			Perm:  model.AdminPermission,
		},
		{
			Name:  "media manager",
			URL:   "/admin/mediamanager",
			Level: 1,
			Perm:  model.AdminPermission,
		},
		{
			Name:  "configuration",
			URL:   "/admin/sandbox",
			Level: 1,
			Perm:  model.AdminPermission,
		},
		{
			Name:  "utilities",
			URL:   "/admin/utils",
			Level: 1,
			Perm:  model.AdminPermission,
		},
		{
			Name:  "logs",
			URL:   "/logs",
			Level: 1,
			Perm:  model.AdminPermission,
		},
	}
	for i := range pages {
		if pages[i].Name == selected {
			pages[i].Selected = true
		}
	}
	return pages
}

// configJSON generates JSON for a config request response given a device, varsum, and device key.
func configJSON(dev *model.Device, vs int64, dk string) (string, error) {
	config := struct {
		MAC           string `json:"ma"`
		Wifi          string `json:"wi"`
		Inputs        string `json:"ip"`
		Outputs       string `json:"op"`
		MonitorPeriod int    `json:"mp"`
		ActPeriod     int    `json:"ap"`
		Version       string `json:"cv"`
		Vs            int64  `json:"vs"`
		DK            string `json:"dk,omitempty"`
	}{
		MAC:           dev.MAC(),
		Wifi:          dev.Wifi,
		Inputs:        dev.Inputs,
		Outputs:       dev.Outputs,
		MonitorPeriod: int(dev.MonitorPeriod),
		ActPeriod:     int(dev.ActPeriod),
		Version:       dev.Version,
		Vs:            vs,
		DK:            dk,
	}

	jsonBytes, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// testHandler handles test operations:
//
//	/test/operation/operand
//
// Users need not be signed in.
func testHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()

	req := strings.Split(r.URL.Path, "/")
	if len(req) < 5 {
		writeHttpError(w, http.StatusBadRequest, "invalid length of url path")
		return
	}

	switch req[2] {
	case "create":
		switch req[3] {
		case "device":
			switch req[4] {
			case "1":
				err := model.PutDevice(ctx, settingsStore, &model.Device{Skey: 1, Mac: 1, Dkey: 10000001, Name: "TestDevice", Inputs: "V0", Enabled: true})
				if err != nil {
					writeHttpErrorf(w, http.StatusInternalServerError, "could not put devices: %v", err)
					return
				}
				fmt.Fprint(w, "OK")
				return
			}
		}
	}

	writeHttpError(w, http.StatusBadRequest, "invalid url path, does not exist")
}

// logRequest logs a request if in debug mode and standalone mode.
// It does nothing in App Engine mode as App Engine logs requests
// automatically.
func logRequest(r *http.Request) {
	if !(debug || standalone) {
		return
	}
	if r.URL.RawQuery == "" {
		log.Println(r.URL.Path)
		return
	}
	log.Println(r.URL.Path + "?" + r.URL.RawQuery)
}

// writeError writes an error in JSON format.
func writeError(w http.ResponseWriter, err error) {
	w.Header().Add("Content-Type", "application/json")
	err2 := json.NewEncoder(w).Encode(map[string]string{"er": err.Error()})
	if err2 != nil {
		log.Printf("failed to write error (%v): %v", err, err2)
		return
	}
	if debug {
		log.Println("Wrote error: " + err.Error())
	}
}

// writeHttpError writes an HTTP error response with the given status code and plain message.
func writeHttpError(w http.ResponseWriter, code int, msg string) {
	http.Error(w, fmt.Sprintf("%s: %s", http.StatusText(code), msg), code)
}

// writeHttpErrorf writes an HTTP error response with the given status code and a formatted message.
func writeHttpErrorf(w http.ResponseWriter, code int, format string, args ...interface{}) {
	http.Error(w, fmt.Sprintf("%s: ", http.StatusText(code))+fmt.Sprintf(format, args...), code)
}
