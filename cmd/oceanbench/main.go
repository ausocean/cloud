/*
NAME
  Ocean Bench - a cloud service for analyzing ocean data.

AUTHORS
  Alan Noble <alan@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2018-2024 the Australian Ocean Lab (AusOcean)

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
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"bitbucket.org/ausocean/iotsvc/gauth"
	"bitbucket.org/ausocean/iotsvc/iotds"
	"bitbucket.org/ausocean/utils/sliceutils"
)

const (
	version      = "0.18.0"
	ptsTolerance = 12000 // 133ms
	localSite    = "localhost"
	localDevice  = "localdevice"
	localEmail   = "localuser@localhost"
	apiSeed      = 845681267
	version138   = 138
)

const (
	oauthClientID      = "802166617157-v67emnahdpvfuc13ijiqb7qm3a7sf45b.apps.googleusercontent.com"
	oauthMaxAge        = 60 * 60 * 24 * 7 // 7 days
	tvServiceURL       = "https://oceantv.appspot.com"
	cronServiceURL     = "https://oceancron.appspot.com"
	cronServiceAccount = "oceancron@appspot.gserviceaccount.com"
	locationID         = "Australia/Adelaide" // TODO: Use site location.
)

// Device health statuses.
type health int

const (
	healthStatusUnknown health = iota - 1
	healthStatusBad
	healthStatusGood
)

// Device state statuses.
const (
	deviceStatusOK = iota
	deviceStatusUpdate
	deviceStatusReboot
	deviceStatusDebug
	deviceStatusUpgrade
	deviceStatusAlarm
	deviceStatusTest
	deviceStatusShutdown
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
	LoginURL   string
	LogoutURL  string
	Users      []iotds.User
	Footer     template.HTML
}

var (
	projectID     = "oceanbench"
	setupMutex    sync.Mutex
	templates     = template.Must(template.New("").Funcs(templateFuncs).ParseGlob("t/*.html"))
	setTemplates  = template.Must(template.New("").Funcs(templateFuncs).ParseGlob("t/set/*.html"))
	rtpEndpoint   string
	trimMTS       bool
	dataHost      = "https://bench.cloudblue.org"
	mediaStore    iotds.Store
	settingsStore iotds.Store
	debug         bool
	standalone    bool
	auth          *gauth.UserAuth
	tvURL         = tvServiceURL
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
	"macdecode":     iotds.MacDecode,
	"split":         strings.Split,
	"part":          sliceutils.StringPart,
	"float":         parseFloat,
	"localdate":     formatLocalDate,
	"localtime":     formatLocalTime,
	"localdatetime": formatLocalDateTime,
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
	exp := strings.Split(os.Getenv("VIDGRIND_EXPERIMENTS"), ",")
	if ok, i := sliceutils.ContainsStringPrefix(exp, "RTP_ENDPOINT="); ok {
		v := exp[i][len("RTP_ENDPOINT="):]
		log.Printf("Experiment RTP_ENDPOINT enabled: %s", v)
		rtpEndpoint = v
	}
	if sliceutils.ContainsString(exp, "TRIM_MTS") {
		log.Printf("Experiment TRIM_MTS enabled")
		trimMTS = true
	}
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
	flag.Parse()

	// Perform one-time setup or bail.
	ctx := context.Background()
	setup(ctx)

	// Serve static files from the "s" directory.
	http.Handle("/s/", http.StripPrefix("/s", http.FileServer(http.Dir("s"))))
	// Except for favicon.ico.
	http.HandleFunc("/favicon.ico", faviconHandler)

	// Get shared cronSecret.
	var err error
	cronSecret, err = gauth.GetHexSecret(ctx, "oceancron", "cronSecret")
	if err != nil {
		log.Printf("could not get cronSecret: %v", err)
	}

	// Device requests.
	// TODO: Remove these once all clients sending to data.cloudblue.org.
	http.HandleFunc("/recv", recvHandler)
	http.HandleFunc("/config", configHandler)
	http.HandleFunc("/poll", pollHandler)
	http.HandleFunc("/act", actHandler)
	http.HandleFunc("/vars", varsHandler)

	// TODO: Remove following once oceantv factored out.
	http.HandleFunc("/broadcast/save", broadcastSaveHandler)

	// User requests.
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/play", playHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/set/devices/edit/var", editVarHandler)
	http.HandleFunc("/set/devices/edit/sensor", editSensorHandler)
	http.HandleFunc("/set/devices/edit/actuator", editActuatorHandler)
	http.HandleFunc("/set/devices/edit", editDevicesHandler)
	http.HandleFunc("/set/devices/", setDevicesHandler)
	http.HandleFunc("/set/crons/edit", editCronsHandler)
	http.HandleFunc("/set/crons/", setCronsHandler)
	http.HandleFunc("/get", getHandler)
	http.HandleFunc("/api/", apiHandler)
	http.HandleFunc("/test/", testHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/oauth2callback", oauthCallbackHandler)
	http.HandleFunc("/checkbroadcasts", checkBroadcastsHandler)
	http.HandleFunc("/live/", liveHandler)
	http.HandleFunc("/monitor", monitorHandler)
	http.HandleFunc("/play/audiorequest", filterHandler)
	http.HandleFunc("/admin/site/add", adminHandler)
	http.HandleFunc("/admin/site/update", adminHandler)
	http.HandleFunc("/admin/site/delete", adminHandler)
	http.HandleFunc("/admin/user/add", adminHandler)
	http.HandleFunc("/admin/user/update", adminHandler)
	http.HandleFunc("/admin/user/delete", adminHandler)
	http.HandleFunc("/admin/site", adminHandler)
	http.HandleFunc("/admin/broadcast", adminHandler)
	http.HandleFunc("/admin/utils", adminHandler)
	http.HandleFunc("/data/", dataHandler)
	http.HandleFunc("/", indexHandler)

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
		auth.Init()
		host = "" // Host is determined by App Engine.
	}

	cronScheduler = proxyScheduler{url: cronURL}
	log.Printf("Listening on %s:%d", host, port)
	log.Printf("Sending cron requests to %s", cronURL)
	log.Printf("Sending TV requests to %s", tvURL)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil))
}

// setup executes per-instance one-time warmup and is used to
// initialize datastores. In standalone mode we use a file store for
// storing both media and settings. In App Engine mode we use
// the netreceiver datastore for settings and the vidgrind datastore for
// media.
//
// In standalone mode all data is associated with site 1.
func setup(ctx context.Context) {
	setupMutex.Lock()
	defer setupMutex.Unlock()

	if mediaStore != nil {
		return
	}
	rand.Seed(apiSeed)

	var err error
	if standalone {
		log.Printf("Running in standalone mode")
		mediaStore, err = iotds.NewStore(ctx, "file", "vidgrind", "store")
		if err == nil {
			settingsStore = mediaStore
			err = setupLocal(ctx, settingsStore)
		}
	} else {
		log.Printf("Running in App Engine mode")
		mediaStore, err = iotds.NewStore(ctx, "cloud", "vidgrind", "")
		if err == nil {
			settingsStore, err = iotds.NewStore(ctx, "cloud", "netreceiver", "")
		}
	}
	if err != nil {
		log.Fatalf("setup failed due to iotds.NewStore error: %v", err)
	}

	iotds.RegisterEntities()
}

// setupLocal creates a local site, user and device for use in standalone mode.
func setupLocal(ctx context.Context, store iotds.Store) error {
	standaloneData = "1:" + localSite
	err := iotds.PutSite(ctx, store, &iotds.Site{Skey: 1, Name: localSite, Enabled: true})
	if err != nil {
		return err
	}
	err = iotds.PutUser(ctx, store, &iotds.User{Skey: 1, Email: localEmail, Perm: iotds.ReadPermission | iotds.WritePermission | iotds.AdminPermission})
	if err != nil {
		return err
	}
	err = iotds.PutDevice(ctx, store, &iotds.Device{Skey: 1, Mac: 1, Dkey: 0, Name: localDevice, Inputs: "A0,V0,S0", MonitorPeriod: 60, Enabled: true})

	return err
}

// faviconHandler serves favicon.ico.
func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "favicon.ico")
}

// indexHandler handles requests for the home page and unimplemented pages.
// Signed-in users are presented with a list of their NetReceiver sites.
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

	ctx := r.Context()
	setup(ctx)
	data.Users, err = getUsersForSiteMenu(w, r, ctx, profile, data)
	if err != nil {
		writeTemplate(w, r, "index.html", &data, fmt.Sprintf("could not populate site menu: %v", err.Error()))
		return
	}

	writeTemplate(w, r, "index.html", &data, "")
}

func getUsersForSiteMenu(w http.ResponseWriter, r *http.Request, ctx context.Context, profile *gauth.Profile, data interface{}) ([]iotds.User, error) {
	users, err := iotds.GetUsers(ctx, settingsStore, profile.Email)
	if err != nil {
		return nil, fmt.Errorf("could not get users: %w", err)
	}

	// Keep track of site keys added.
	added := map[int64]bool{}
	for _, u := range users {
		added[u.Skey] = true
	}

	// Get all public sites, if a public site hasn't been added yet, add it.
	publicSites, err := iotds.GetPublicSites(ctx, settingsStore)
	if err != nil {
		return nil, fmt.Errorf("could not get public sites: %w", err)
	}
	for _, s := range publicSites {
		if !added[s.Skey] {
			users = append(users, iotds.User{Skey: s.Skey, Perm: iotds.ReadPermission})
		}
	}
	return users, nil
}

// warmupHandler handles warmup requests. It is a no-op that simply ensures that the intance is loaded.
func warmupHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	w.Write([]byte{})
}

// cronIndexHandler renders the one and only page served in cron mode.
func cronIndexHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	if r.URL.Path != "/" {
		// Redirect all invalid URLs to the home page.
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	data := commonData{}

	writeTemplate(w, r, "cron-index.html", &data, "")
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

	ok, err := hasPermission(ctx, p, mid, iotds.ReadPermission)
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

	_, pin := iotds.FromMID(mid)
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
	ma, _ := iotds.FromMID(mid)
	dev, err := iotds.GetDevice(ctx, settingsStore, iotds.MacEncode(ma))
	if err != nil {
		if err != iotds.ErrNoSuchEntity {
			return false, fmt.Errorf("error getting device: %w", err)
		}
		return false, nil
	}
	site, err := iotds.GetSite(ctx, settingsStore, dev.Skey)
	if err != nil {
		return false, fmt.Errorf("error getting site: %w", err)
	}
	if site.Public {
		return perm == iotds.ReadPermission, nil
	}
	if p == nil {
		return false, nil // User not logged in.
	}
	user, err := iotds.GetUser(ctx, settingsStore, dev.Skey, p.Email)
	if err != nil {
		return false, fmt.Errorf("error getting user: %w", err)
	}
	return perm&user.Perm != 0, nil
}

// writeTemplate writes the given template with the supplied data,
// populating some common properties.
func writeTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}, msg string) {
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
		profile, _ := getProfile(w, r)
		p.Set(reflect.ValueOf(profile))
	}
	p = v.FieldByName("LoginURL")
	if p.IsValid() {
		p.Set(reflect.ValueOf("/login?redirect=" + r.URL.RequestURI()))
	}
	p = v.FieldByName("LogoutURL")
	if p.IsValid() {
		p.Set(reflect.ValueOf("/logout?redirect=" + r.URL.RequestURI()))
	}

	b, err := os.ReadFile("s/footer.html")
	if err != nil {
		log.Fatalf("could not load footer")
	}
	f := v.FieldByName("Footer")
	f.Set(reflect.ValueOf(template.HTML(string(b))))

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
			Perm: iotds.ReadPermission,
		},
		{
			Name: "search",
			URL:  "/search",
			Perm: iotds.ReadPermission,
		},
		{
			Name: "monitor",
			URL:  "/monitor",
			Perm: iotds.ReadPermission,
		},
		{
			Name: "play",
			URL:  "/play",
			Perm: iotds.ReadPermission,
		},
		{
			Name: "upload",
			URL:  "/upload",
			Perm: iotds.WritePermission,
		},
		{
			Name:  "settings",
			Group: true,
			Perm:  iotds.WritePermission,
		},
		{
			Name:  "devices",
			URL:   "/set/devices",
			Level: 1,
			Perm:  iotds.WritePermission,
		},
		{
			Name:  "crons",
			URL:   "/set/crons",
			Level: 1,
			Perm:  iotds.WritePermission,
		},
		{
			Name:  "admin",
			Group: true,
			Perm:  iotds.AdminPermission,
		},
		{
			Name:  "site",
			URL:   "/admin/site",
			Level: 1,
			Perm:  iotds.AdminPermission,
		},
		{
			Name:  "broadcast",
			URL:   "/admin/broadcast",
			Level: 1,
			Perm:  iotds.AdminPermission,
		},
		{
			Name:  "utilities",
			URL:   "/admin/utils",
			Level: 1,
			Perm:  iotds.AdminPermission,
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
func configJSON(dev *iotds.Device, vs int64, dk string) (string, error) {
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

// configHandler handles configuration requests for a given device.
func configHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()

	q := r.URL.Query()
	ma := q.Get("ma")
	dk := q.Get("dk")
	vn := q.Get("vn")
	ut := q.Get("ut")
	la := q.Get("la")
	vt := q.Get("vt")

	// Extract var types from the body if vt is present.
	var varTypes map[string]string
	if vt != "" {
		n, err := strconv.Atoi(vt)
		if err != nil {
			writeError(w, errInvalidSize)
			return
		}
		body := make([]byte, n)
		_, err = io.ReadFull(r.Body, body)
		if err != nil {
			writeError(w, errInvalidBody)
			return
		}
		err = json.Unmarshal(body, &varTypes)
		if err != nil {
			writeError(w, errInvalidJSON)
			return
		}
	}

	// Is this request for a valid device?
	setup(ctx)
	dev, err := iotds.CheckDevice(ctx, settingsStore, ma, dk)

	var dkey int64
	switch err {
	case nil, iotds.ErrInvalidDeviceKey:
		dkey, _ = strconv.ParseInt(dk, 10, 64) // Can't fail.
	case iotds.ErrMissingDeviceKey:
		// Device key defaults to zero.
	case iotds.ErrNoSuchEntity:
		log.Printf("/config from unknown device %s", ma)
		writeError(w, iotds.ErrDeviceNotFound)
		return
	default:
		writeDeviceError(w, dev, err)
		return
	}

	// NB: Only reveal the device key if it has changed.
	dk = ""

	if dev.Status == deviceStatusOK {
		// Device is configured, so check the device key matches.
		if dkey != dev.Dkey {
			// We should not get here. A known, configured device is using the wrong key,
			// so we return an error rather than forcing the device to reconfigure.
			log.Printf("/config from device %s with invalid device key %d", ma, dkey)
			writeError(w, iotds.ErrInvalidDeviceKey)
			return
		}

	} else {
		// Device is not configured
		log.Printf("/config from unconfigured device %s", ma)
		if dkey != dev.Dkey {
			// Inform the device of its new key.
			dk = strconv.FormatInt(dev.Dkey, 10)
		}
		dev.Status = deviceStatusOK
	}

	vs, _ := iotds.GetVarSum(ctx, settingsStore, dev.Skey, dev.Hex())
	resp, err := configJSON(dev, vs, dk)
	if err != nil {
		log.Printf("could not generate config response JSON for device with MAC %v: %v", ma, err)
		writeError(w, err)
		return
	}
	fmt.Fprint(w, resp)

	// Update the device.
	dev.Updated = time.Now()
	dev.Protocol = vn
	iotds.PutDevice(ctx, settingsStore, dev)

	// Update the system variables for this device with the client's uptime, local address and var types.
	if ut != "" {
		iotds.PutVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".uptime", ut)
	}
	if la != "" {
		iotds.PutVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".localaddr", la)
	}
	if varTypes != nil {
		for k, v := range varTypes {
			iotds.PutVariable(ctx, settingsStore, dev.Skey, "_type."+k, v)
		}
	}
}

// pollHandler handles poll requests.
func pollHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()

	q := r.URL.Query()
	ma := q.Get("ma")
	dk := q.Get("dk")
	ut := q.Get("ut")
	vn := q.Get("vn")

	// Is this request for a valid device?
	setup(ctx)
	dev, err := iotds.CheckDevice(ctx, settingsStore, ma, dk)
	if err != nil {
		writeDeviceError(w, dev, err)
		return
	}
	// Update the client protocol version number if it has changed.
	if vn != "" && vn != dev.Protocol {
		log.Printf("netsender %s updated to protocol %s", ma, vn)
		dev.Protocol = vn
		err := iotds.PutDevice(ctx, settingsStore, dev)
		if err != nil {
			log.Printf("error putting device %s: %v", ma, err)
		}
	}
	vs, err := iotds.GetVarSum(ctx, settingsStore, dev.Skey, dev.Hex())
	if err != nil {
		log.Printf("error getting varsum: %v", err)
	}

	for _, pin := range strings.Split(dev.Inputs, ",") {
		// Get numeric value for pin, if present.
		v := q.Get(pin)
		if v == "" {
			continue
		}
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			writeError(w, errInvalidValue)
			break
		}

		switch pin[0] {
		case 'A', 'D', 'X':
			err = writeScalar(r, ma, pin, n)

		case 'B':
			// Not implemented.

		case 'S', 'V':
			// Handled by /recv.

		case 'T':
			err = writeText(r, ma, pin, int(n))

		default:
			err = errInvalidPin
		}

		if err != nil {
			writeError(w, err)
			return
		}
	}

	respMap := map[string]interface{}{"ma": ma, "vs": int(vs)}
	if dev.Status != deviceStatusOK {
		respMap["rc"] = int(dev.Status)
	}

	err = processActuators(ctx, dev, respMap)
	if err != nil {
		writeError(w, err)
		return
	}

	// Update the system variable for this device with the client's uptime.
	err = iotds.PutVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".uptime", ut)
	if err != nil {
		log.Printf("error putting variable %s: %v", "_"+dev.Hex()+".uptime", err)
	}

	resp, err := json.Marshal(respMap)
	if err != nil {
		writeError(w, fmt.Errorf("could not marshal response map %w", err))
		return
	}
	w.Write(resp)
}

// processActuators updates the response map with actuator values, if any.
func processActuators(ctx context.Context, dev *iotds.Device, respMap map[string]interface{}) error {
	acts, err := iotds.GetActuatorsV2(ctx, settingsStore, dev.Mac)
	if err != nil {
		return fmt.Errorf("failed to get actuators for device %d: %w", dev.Mac, err)
	}
	for _, act := range acts {
		// Ignore defunct actuators.
		if !sliceutils.ContainsString(dev.OutputList(), act.Pin) {
			continue
		}

		// Actuator var names are relative to their device.
		val, err := iotds.GetVariable(ctx, settingsStore, dev.Skey, dev.Hex()+"."+act.Var)
		if err != nil {
			return fmt.Errorf("failed to get actuator by %s.%s: %w", dev.Hex(), act.Pin, err)
		}

		n, err := toInt(val.Value)
		if err != nil {
			return fmt.Errorf("could not convert variable value to int: %w", err)
		}
		respMap[act.Pin] = n
	}
	return nil
}

// toInt returns 1 for "true", 0 for "false", or otherwise attempts to parse the string as an integer.
func toInt(s string) (int64, error) {
	s = strings.ToLower(s)
	switch s {
	case "true":
		return 1, nil
	case "false":
		return 0, nil
	default:
		return strconv.ParseInt(s, 10, 64)
	}
}

// writeScalar writes a scalar value.
func writeScalar(r *http.Request, ma, pin string, n float64) error {
	id := iotds.ToSID(ma, pin)
	ts := time.Now().Unix()
	return iotds.PutScalar(r.Context(), mediaStore, &iotds.Scalar{ID: id, Timestamp: ts, Value: n})
}

// writeText writes text data.
func writeText(r *http.Request, ma, pin string, n int) error {
	data := make([]byte, n)
	n_, err := io.ReadFull(r.Body, data)
	if err != nil {
		return err
	}
	if n != n_ {
		return errInvalidSize
	}

	mid := iotds.ToMID(ma, pin)
	ts := time.Now().Unix()
	tt := r.Header.Get("Content-Type")
	return iotds.WriteText(r.Context(), mediaStore, &iotds.Text{MID: mid, Timestamp: ts, Data: string(data), Type: tt})
}

// actHandler handles act requests.
func actHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()
	q := r.URL.Query()
	ma := q.Get("ma")
	dk := q.Get("dk")

	// Is this request for a valid device?
	setup(ctx)
	dev, err := iotds.CheckDevice(ctx, settingsStore, ma, dk)
	if err != nil {
		writeDeviceError(w, dev, err)
		return
	}

	respMap := map[string]interface{}{"ma": ma}

	// If status is not okay.
	if dev.Status != deviceStatusOK {
		respMap["rc"] = int(dev.Status)
	} else {
		vsInt, err := iotds.GetVarSum(ctx, settingsStore, dev.Skey, dev.Hex())
		if err != nil {
			writeError(w, fmt.Errorf("could not get var sum: %w", err))
			return
		}

		respMap["vs"] = int(vsInt)
	}

	err = processActuators(ctx, dev, respMap)
	if err != nil {
		writeError(w, err)
		return
	}

	err = iotds.PutVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".uptime", "")
	if err != nil {
		log.Printf("error putting variable %s: %v", "_"+dev.Hex()+".uptime", err)
	}

	resp, err := json.Marshal(respMap)
	if err != nil {
		writeError(w, fmt.Errorf("could not marshal response map %w", err))
		return
	}

	w.Write(resp)
}

// varsHandler returns vars for a given device (except for system variables).
// NB: Format vs as a string, not an int.
func varsHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()

	q := r.URL.Query()
	ma := q.Get("ma")
	dk := q.Get("dk")
	md := q.Get("md")
	er := q.Get("er")

	// Is this request for a valid device?
	setup(ctx)
	dev, err := iotds.CheckDevice(ctx, settingsStore, ma, dk)
	if err != nil {
		writeDeviceError(w, dev, err)
		return
	}

	if md != "" {
		iotds.PutVariable(ctx, settingsStore, dev.Skey, dev.Hex()+".mode", md)
		iotds.PutVariable(ctx, settingsStore, dev.Skey, dev.Hex()+".error", er)
	}
	vars, err := iotds.GetVariablesBySite(ctx, settingsStore, dev.Skey, dev.Hex())
	if err != nil {
		writeError(w, err)
		return
	}

	resp := `{"id":"` + dev.Hex() + `",`
	for _, v := range vars {
		if v.IsSystemVariable() {
			continue
		}
		resp += `"` + v.Name + `":"` + v.Value + `",`

	}
	vs := iotds.ComputeVarSum(vars)
	resp += `"vs":"` + strconv.Itoa(int(vs)) + `"}`
	fmt.Fprint(w, resp)
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
				err := iotds.PutDevice(ctx, settingsStore, &iotds.Device{Skey: 1, Mac: 1, Dkey: 10000001, Name: "TestDevice", Inputs: "V0", Enabled: true})
				if err != nil {
					writeHttpError(w, http.StatusInternalServerError, "could not put devices: %v", err)
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
	writeDeviceError(w, nil, err)
}

// writeDeviceError writes an error in JSON format with an optional update response code for device key errors.
func writeDeviceError(w http.ResponseWriter, dev *iotds.Device, err error) {
	var rc string
	switch err {
	case iotds.ErrMalformedDeviceKey, iotds.ErrInvalidDeviceKey:
		if dev != nil {
			log.Printf("bad request from %s: %v", dev.MAC(), err)
		}
		fallthrough
	case iotds.ErrMissingDeviceKey:
		rc = `,"rc":` + strconv.Itoa(deviceStatusUpdate)
	}
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprint(w, `{"er":"`+err.Error()+`"`+rc+`}`)
	if debug {
		log.Println("Wrote error: " + err.Error())
	}
}

// httpError writes http errors to the response writer, in order to provide more detailed
// response errors in a concise manner.
func writeHttpError(w http.ResponseWriter, code int, msg string, args ...interface{}) {
	errorMsg := "%s: "
	if msg != "" {
		errorMsg += msg
	}
	if len(args) > 0 {
		errorMsg += ": "
		errorMsg = fmt.Sprintf(errorMsg, http.StatusText(code), args)
	} else {
		errorMsg = fmt.Sprintf(errorMsg, http.StatusText(code))
	}
	http.Error(w, errorMsg, code)
}
