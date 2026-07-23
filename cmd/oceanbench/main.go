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
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"reflect"
	rtdebug "runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/ausocean/cloud/backend"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/utils/sliceutils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/encryptcookie"
	"github.com/gofiber/template/html/v3"
	"github.com/joho/godotenv"
)

const (
	version     = "v0.38.0"
	localSite   = "localhost"
	localDevice = "localdevice"
	localEmail  = "localuser@localhost"
)

const (
	skeyParamKey = "skey" // Parameter key used for site key in url path.
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
	Standalone     bool
	Debug          bool
	Version        string
	CommitHash     string
	Msg            string
	Pages          []page
	PageData       interface{}
	Profile        *gauth.Profile
	SuperAdmin     bool
	LoginURL       string
	LogoutURL      string
	Users          []model.User
	CurrentSiteKey int64
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
	errInvalidBody   = errors.New("invalid body")
	errInvalidJSON   = errors.New("invalid JSON")
	errInvalidFormat = errors.New("invalid format")
)

var (
	cronScheduler proxyScheduler
	cronSecret    []byte
	commitHash    string
)

func init() {
	if info, ok := rtdebug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				commitHash = setting.Value
				if len(commitHash) > 7 {
					commitHash = commitHash[:7]
				}
				break
			}
		}
	}
	if commitHash == "" {
		commitHash = os.Getenv("COMMIT_HASH")
	}
}

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
	// Load environment variables from .env file if present.
	// This is a no-op if the file doesn't exist.
	_ = godotenv.Load()

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

	// Setup template rendering.
	engine := setupTemplating()

	// Build the Fiber application.
	app := fiber.New(fiber.Config{Views: engine})

	encryptCookies(ctx, app)

	// Serve static files from the "s" directory.
	app.Static("/s", "./s")

	// Except for favicon.ico.
	app.Get("/favicon.ico", func(c *fiber.Ctx) error {
		return c.SendFile("./favicon.ico")
	})

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

	// Setup routes for the API, ie. /api requests.
	setupAPIRoutes(app)

	// User requests.
	app.All("/search", searchHandler)
	app.Post("/play/audiorequest", filterHandler)
	app.All("/play", playHandler)
	app.All("/learn/mooring", mooringHandler)
	app.All("/upload", uploadHandler)
	app.All("/set/devices/edit/var", editVarHandler)
	app.All("/set/devices/edit/sensor", editSensorHandler)
	app.All("/set/devices/edit/actuator", editActuatorHandler)
	app.All("/set/devices/edit/calibrate", calibrateDevicesHandler)
	app.All("/set/devices/edit", editDevicesHandler)
	app.All("/set/devices/vars", setDevicesVars)
	app.All("/set/devices/*", setDevicesHandler)
	app.All("/set/crons/edit", editCronsHandler)
	app.All("/set/crons/*", setCronsHandler)
	app.All("/get", getHandler)
	app.All("/test/*", testHandler)
	app.All("/login", loginHandler)
	app.All("/logout", logoutHandler)
	app.All("/oauth2callback", oauthCallbackHandler)
	app.All("/live/:broadcastName", liveHandler)
	app.All("/monitor", monitorHandler)
	app.All("/admin/site/add", adminHandler)
	app.All("/admin/site/update", adminHandler)
	app.All("/admin/site/delete", adminHandler)
	app.All("/admin/user/add", adminHandler)
	app.All("/admin/user/update", adminHandler)
	app.All("/admin/user/delete", adminHandler)
	app.All("/admin/site", adminHandler)
	app.All("/admin/broadcast", adminHandler)
	app.All("/admin/tv-overview", tvOverviewHandler)
	app.All("/admin/missioncontrol", adminHandler)
	app.All("/admin/mediamanager", adminHandler)
	app.All("/admin/sandbox/configure", configDevicesHandler)
	app.All("/admin/sandbox", sandboxHandler)
	app.All("/admin/utils", adminHandler)
	app.All("/data/*", dataHandler)
	app.All("/throughputs", throughputsHandler)
	app.All("/logs", logPageHandler)

	// Handle paths with prefixed site keys.
	app.Group("/:"+skeyParamKey).
		All("/*", indexHandler)

	app.All("/*", indexHandler)

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
		auth.Init(backend.NewFiberHandler(nil))

		// If we are running in app engine mode locally, we want to request data
		// from the local instance.
		if os.Getenv("GAE_ENV") == "" {
			dataHost = "http://" + host + ":" + strconv.Itoa(port)
		} else {
			host = "" // Host is determined by App Engine.
		}
	}

	cronScheduler = proxyScheduler{url: cronURL}
	log.Printf("Listening on %s:%d", host, port)
	log.Printf("Sending cron requests to %s", cronURL)
	log.Printf("Sending TV requests to %s", tvURL)
	log.Fatal(app.Listen(fmt.Sprintf("%s:%d", host, port)))
}

func setupTemplating() *html.Engine {
	templateDir := "cmd/oceanbench/t"
	if standalone || os.Getenv("GAE_ENV") == "" {
		templateDir = "t"
	}
	engine := html.New(templateDir, ".html")
	engine.AddFuncMap(templateFuncs)
	return engine
}

// encryptCookies encrypts all cookies in the fiber app with the project's
// secret key. This must be called before any middleware or handlers that access
// cookies are setup.
//
// All errors are considered fatal.
func encryptCookies(ctx context.Context, app *fiber.App) {
	key, err := gauth.GetSecret(ctx, projectID, "sessionKey")
	if err != nil {
		log.Fatalf("unable to get sessionKey secret: %v", err)
	}
	app.Use(encryptcookie.New(encryptcookie.Config{
		Key: key[0:32],
	}))
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

// indexHandler handles requests for the home page and unimplemented pages.
// Signed-in users are presented with a list of their sites.
//
// Requests without signed in user are redirected to "/" and rendered.
// Requests to / are redirected with prefixed default skey
// Invalid requests are redirected to index with prefixed default skey
func indexHandler(c *fiber.Ctx) error {
	logRequest(c)

	profile, err := getProfile(c)
	data := commonData{
		Pages:   pages("home"),
		Profile: profile,
	}
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		if c.Path() != "/" {
			// Clear the URL if not the root URL.
			return c.Redirect("/", fiber.StatusSeeOther)
		}
		writeTemplate(c, "index.html", &data, "")
		return nil
	}

	skey, err := c.ParamsInt(skeyParamKey)
	if err != nil {
		// Get the default skey and redirect.
		skey, err := getDefaultSkey(c.UserContext(), profile)
		if err != nil {
			// This should never happen, and if it does we likely can't recover.
			log.Panicf("unable to get default skey: %v", err)
		}
		return c.Redirect(fmt.Sprintf("/%d", skey), fiber.StatusSeeOther)
	}

	if c.Params("*") != "" {
		// Redirect to /:skey
		return c.Redirect(fmt.Sprintf("/%d", skey), fiber.StatusSeeOther)
	}

	writeTemplate(c, "index.html", &data, "")
	return nil
}

// getHandler handles media and text requests, depending on the pin type.
// Requires read permission for the requested media, otherwise permission is denied.
// The user need not be logged in to access public sites.
// When no output is specified, media data is downloaded to the client.
func getHandler(c *fiber.Ctx) error {
	logRequest(c)

	p, _ := getProfile(c) // Ignore errors, since users need not be logged in.

	id := c.Query("id")
	mid, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		writeError(c, errInvalidMID)
		return err
	}

	t := c.Query("ts")
	var ts []int64
	if t != "" {
		ts, err = splitTimestamps(t, false)
		if err != nil {
			writeError(c, errInvalidTimestamp)
			return err
		}
	}

	k := c.Query("ky")
	var ky []uint64
	if k != "" {
		ky, err = splitUints(k)
		if err != nil {
			writeError(c, errInvalidKey)
			return err
		}
	}

	ctx := c.UserContext()
	setup(ctx)

	ok, err := hasPermission(ctx, p, mid, model.ReadPermission)
	if err != nil {
		writeError(c, err)
		return err
	}
	if !ok {
		writeError(c, errPermissionDenied)
		return err
	}

	var content []byte
	var mime, name string

	_, pin := model.FromMID(mid)
	switch pin[0] {
	case 'V', 'S':
		content, mime, err = getMedia(c, mid, ts, ky)
		if err != nil {
			writeError(c, fmt.Errorf("could not get media: %w", err))
			return err
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
		content, mime, err = getText(c, mid, ts, ky)
		if err != nil {
			writeError(c, fmt.Errorf("could not get text: %w", err))
			return err
		}

		if mime == "application/json" {
			name = "data.json"
			break
		}

		name = "data.txt"

	default:
		err := fmt.Errorf("unknown pin type: %v", pin[0])
		writeError(c, err)
		return err
	}

	writeData(c, content, mime, name)
	return nil
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
func writeTemplate(c *fiber.Ctx, name string, data interface{}, msg string) error {
	profile, _ := getProfile(c)
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
	p = v.FieldByName("CommitHash")
	if p.IsValid() {
		p.SetString(commitHash)
	}
	p = v.FieldByName("Msg")
	if p.IsValid() {
		p.SetString(msg)
	}
	p = v.FieldByName("Profile")
	if p.IsValid() {
		p.Set(reflect.ValueOf(profile))
	}
	skey, _ := requestSiteData(c, profile)
	p = v.FieldByName("CurrentSiteKey")
	if p.IsValid() {
		p.SetInt(skey)
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
		p.Set(reflect.ValueOf("/login?redirect=" + c.OriginalURL()))
	}
	p = v.FieldByName("LogoutURL")
	if p.IsValid() {
		p.Set(reflect.ValueOf("/logout?redirect=" + c.OriginalURL()))
	}

	name, _ = strings.CutSuffix(name, ".html")
	return c.Render(name, data)
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
func testHandler(c *fiber.Ctx) error {
	logRequest(c)
	ctx := c.UserContext()

	req := strings.Split(c.Path(), "/")
	if len(req) < 5 {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Errorf("invalid length of url path")})
	}

	switch req[2] {
	case "create":
		switch req[3] {
		case "device":
			switch req[4] {
			case "1":
				err := model.PutDevice(ctx, settingsStore, &model.Device{Skey: 1, Mac: 1, Dkey: 10000001, Name: "TestDevice", Inputs: "V0", Enabled: true})
				if err != nil {
					c.Status(fiber.StatusInternalServerError)
					return c.JSON(fiber.Map{"error": fmt.Errorf("could not put devices: %v", err)})
				}
				_, err = fmt.Fprint(c, "OK")
				return err
			}
		}
	}

	c.Status(fiber.StatusBadRequest)
	return c.JSON(fiber.Map{"error": fmt.Errorf("invalid url path, does not exist")})
}

// logRequest logs a request if in debug mode and standalone mode.
// It does nothing in App Engine mode as App Engine logs requests
// automatically.
func logRequest(c *fiber.Ctx) {
	if !(debug || standalone) {
		return
	}
	log.Println(c.OriginalURL())
}

// writeError c JSON format.
func writeError(c *fiber.Ctx, err error) {
	err2 := c.JSON(fiber.Map{"er": err.Error()})
	if err2 != nil {
		log.Printf("failed to write error (%v): %v", err, err2)
		return
	}
	if debug {
		log.Println("Wrote error: " + err.Error())
	}
}
