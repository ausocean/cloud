/*
AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Center. Ocean Center is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean Center is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with Ocean Center in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

// Ocean Center is a cloud service for remote device management, including:
//
// - device software installation
// - device software upgrades
// - device enabling and disabling (TODO)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/notify"
	"github.com/ausocean/openfish/datastore"
	"github.com/ausocean/utils/totp"
)

// Project constants.
const (
	projectID = "oceancenter"
	version   = "v0.3.0"
)

// Site/device defaults.
const (
	sandboxSite   = 3
	sandboxName   = "Sandbox"
	sandboxOrg    = "AusOcean"
	sandboxOwner  = "david@ausocean.org"
	sandboxOps    = "ops@ausocean.org"
	sandboxTz     = 9.5
	sandboxPeriod = 1 * time.Hour
	sandboxLoc    = "Australia/Adelaide"
	devPeriod     = 60 * time.Second
	localEmail    = "localuser@localhost"
)

// TOTP constants.
const (
	totpSecretKey   = "totpSecret"
	totpDigits      = 16
	totpGracePeriod = 2 * time.Minute
)

// Misc constants.
const (
	notifyNewDevice notify.Kind = "new-device"
)

// service defines the properties of our web service.
type service struct {
	setupMutex    sync.Mutex
	settingsStore datastore.Store
	debug         bool
	standalone    bool
	notifier      notify.Notifier
	totpSecret    []byte
	storePath     string
}

// app is an instance of our service.
var app *service = &service{}

func main() {
	defaultPort := 8084
	v := os.Getenv("PORT")
	if v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			defaultPort = i
		}
	}

	var host string
	var port int
	flag.BoolVar(&app.debug, "debug", false, "Run in debug mode.")
	flag.BoolVar(&app.standalone, "standalone", false, "Run in standalone mode.")
	flag.StringVar(&host, "host", "localhost", "Host we run on in standalone mode")
	flag.IntVar(&port, "port", defaultPort, "Port we listen on in standalone mode")
	flag.StringVar(&app.storePath, "filestore", "store", "File store path")
	flag.Parse()

	// Perform one-time setup or bail.
	ctx := context.Background()
	app.setup(ctx)

	// Serve static files when running locally
	http.Handle("/s/", http.StripPrefix("/s", http.FileServer(http.Dir("s"))))
	http.Handle("/dl/", http.StripPrefix("/dl", http.FileServer(http.Dir("dl"))))

	http.HandleFunc("/", app.indexHandler)
	http.HandleFunc("/install", app.installHandler)

	log.Printf("Listening on %s:%d", host, port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil))
}

// indexHandler handles requests for the home page and is here just to
// test that the service is running. Devices do not use this endpoint.
func (svc *service) indexHandler(w http.ResponseWriter, r *http.Request) {
	svc.logRequest(r)
	w.Write([]byte(projectID + " " + version))
}

// setup executes per-instance one-time warmup and is used to
// initialize the service. Any errors are considered fatal.
func (svc *service) setup(ctx context.Context) {
	svc.setupMutex.Lock()
	defer svc.setupMutex.Unlock()

	if svc.settingsStore != nil {
		return
	}

	var err error
	svc.settingsStore, _, err = model.SetupDatastore(svc.standalone, svc.storePath, ctx)
	if err != nil {
		log.Fatalf("could not set up datastore: %v", err)
	}

	// Get or create sandbox site and its admin user if it doesn't exist.
	site, err := model.GetSite(ctx, svc.settingsStore, sandboxSite)
	switch {
	case err == nil:
		// Nothing to do.
	case errors.Is(err, datastore.ErrNoSuchEntity):
		owner := sandboxOwner
		if svc.standalone {
			owner = localEmail
		}
		site = &model.Site{
			Skey:         sandboxSite,
			Name:         sandboxName,
			OrgID:        sandboxOrg,
			OwnerEmail:   owner,
			OpsEmail:     sandboxOps,
			NotifyPeriod: int64(sandboxPeriod.Hours()),
			Timezone:     sandboxTz,
			Created:      time.Now(),
			Enabled:      true,
		}
		err := model.PutSite(ctx, svc.settingsStore, site)
		if err != nil {
			log.Fatalf("could not put sandbox site: %v", err)
		}
		user := &model.User{
			Skey:  sandboxSite,
			Email: owner,
			Perm:  model.ReadPermission | model.WritePermission | model.AdminPermission,
		}
		err = model.PutUser(ctx, svc.settingsStore, user)
		if err != nil {
			log.Fatalf("could not put sandbox user: %v", err)
		}

	default:
		log.Fatalf("could not get sandbox site: %v", err)
	}
	log.Printf("set up datatore")

	// Set up email notifier.
	secrets, err := gauth.GetSecrets(ctx, projectID, nil)
	if err != nil {
		log.Fatalf("could not get secrets: %v", err)
	}
	svc.notifier, err = notify.NewMailjetNotifier(
		notify.WithSecrets(secrets),
		notify.WithRecipient(site.OpsEmail),
		notify.WithStore(notify.NewStore(svc.settingsStore)),
		notify.WithPeriod(time.Duration(site.NotifyPeriod)*time.Hour),
	)
	if err != nil {
		log.Fatalf("could not set up email notifier: %v", err)
	}
	log.Printf("set up notifier")

	// Get secret required for TOTP.
	secret, ok := secrets[totpSecretKey]
	if !ok {
		log.Fatalf("could not get %s", totpSecretKey)
	}
	svc.totpSecret = []byte(secret)
	log.Printf("set up TOTP")
}

// installHandler handles installation requests from new devices.
// Three parameters are expected:
//
// - wi: WiFi MAC address (not used for configuration).
// - ma: MAC address (used for configuration).
// - dk: device key (a valid TOTP generated by totpgen).
//
// If a request is received from an unknown device, and the device key
// is a valid TOTP, a device is created and placed in the sandbox
// site. Ops for the sandbox site is notified of the new device and
// responsible for assigning the client type (ct). A device
// configuration is not considered complete until its client type has
// been assigned. Clients are therefore expected to periodically call
// this method until the the device configuration has been completed
// by the operator.

// The response is in netsender.conf format, i.e., with one
// parameter per line.
//
//	ma <MAC-address>
//	dk <device-key>
//	ct <client-type> (omitted when empty)
//
// In the case of an error, the following is returned without any
// device configuration information. The error message is also logged.
//
//	er:<error-message>
func (svc *service) installHandler(w http.ResponseWriter, r *http.Request) {
	svc.logRequest(r)
	ctx := r.Context()

	wi := r.FormValue("wi")
	ma := r.FormValue("ma")
	dk := r.FormValue("dk")

	// Validate request params.
	if wi == "" {
		writeError(w, http.StatusBadRequest, "missing wi param")
		return
	}
	if model.MacEncode(wi) == 0 {
		writeError(w, http.StatusBadRequest, "wi invalid MAC address")
		return
	}
	if ma == "" {
		writeError(w, http.StatusBadRequest, "missing ma param")
		return
	}
	mac := model.MacEncode(ma)
	if mac == 0 {
		writeError(w, http.StatusBadRequest, "ma invalid MAC address")
		return
	}
	if dk == "" {
		writeError(w, http.StatusBadRequest, "missing dk param")
		return
	}
	dkey, err := strconv.ParseInt(dk, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not parse device key")
		return
	}

	// Attempt to get this device.
	dev, err := model.GetDevice(ctx, svc.settingsStore, mac)
	if err == nil {
		// Device already exists => inform client what we know about it.
		writeDeviceConfig(w, dev)
		return
	}

	if !errors.Is(err, datastore.ErrNoSuchEntity) {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not get device: %v", err))
		return
	}

	// We've detected a new device.
	// Check if device key is a recently-generated TOTP.
	ok, err := totp.CheckTOTP(dk, time.Now(), totpGracePeriod, totpDigits, svc.totpSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not check TOTP: %v", err))
		return
	}
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid device key")
		return
	}

	// Provision a new device with a temporary name that includes the current time.
	now := time.Now()
	loc, err := time.LoadLocation(sandboxLoc)
	if err != nil {
		panic(fmt.Errorf("could not load location %s: %w", sandboxLoc, err))
	}
	localTime := now.In(loc)
	name := fmt.Sprintf("New device detected at %s", localTime.Format("2006-01-02 15:04:05"))

	dev = &model.Device{
		Skey:          sandboxSite,
		Dkey:          dkey,
		Mac:           mac,
		Name:          name,
		MonitorPeriod: int64(devPeriod.Seconds()),
		ActPeriod:     int64(devPeriod.Seconds()),
		Status:        model.DeviceStatusUpdate,
		Updated:       now,
	}
	err = model.PutDevice(ctx, svc.settingsStore, dev)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not put device: %v", err))
		return
	}

	// Inform the client.
	// NB: This will be an incomplete config, since the client type is not yet known.
	writeDeviceConfig(w, dev)

	// Notify ops of the new device.
	// TODO: Use a template once the notifier supports templates.
	msg := fmt.Sprintf("New device with MAC addresses %s, %s and device key %s detected at %s.\nConfigure at https://bench.cloudblue.org/set/devices/?ma=%s&sk=auto",
		wi, ma, dk, localTime.Format("2006-01-02 15:04:05"), ma)
	err = svc.notifier.Send(ctx, sandboxSite, notifyNewDevice, msg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not send notification: %v", err))
		return
	}
}

// writeDeviceConfig writes a minimal device configuration in CSV
// format that can be used by clients to write netsender.conf.
// The client type (ct) param is omitted when it is empty.
func writeDeviceConfig(w http.ResponseWriter, dev *model.Device) {
	var resp string
	if dev.Type == "" {
		resp = fmt.Sprintf("ma %s\ndk %d", dev.MAC(), dev.Dkey)
	} else {
		resp = fmt.Sprintf("ma %s\ndk %d\nct %s", dev.MAC(), dev.Dkey, dev.Type)
	}
	log.Printf("Replying with %s", strings.ReplaceAll(resp, "\n", " "))
	w.Write([]byte(resp))
}

// writeError writes HTTP errors, with a client-friendly error message
// prefixed er:.
// NB: This is to make parsing easier for dumb clients, such as shell scripts.
func writeError(w http.ResponseWriter, status int, msg string) {
	log.Printf("%s", msg)
	http.Error(w, "er:"+msg, status)
}

// logRequest logs a request if in debug mode and standalone mode.
// It does nothing in App Engine mode as App Engine logs requests
// automatically.
func (svc *service) logRequest(r *http.Request) {
	if !(svc.debug || svc.standalone) {
		return
	}
	if r.URL.RawQuery == "" {
		log.Println(r.URL.Path)
		return
	}
	log.Println(r.URL.Path + "?" + r.URL.RawQuery)
}
