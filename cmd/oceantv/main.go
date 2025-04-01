/*
LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with Ocean TV in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

// Ocean TV is a cloud service for managing YouTube broadcasts.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/openfish"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/notify"
	"github.com/ausocean/cloud/utils"
	"github.com/ausocean/openfish/datastore"
)

const (
	projectID          = "oceantv"
	version            = "v0.9.1"
	projectURL         = "https://oceantv.appspot.com"
	cronServiceAccount = "oceancron@appspot.gserviceaccount.com"
	locationID         = "Australia/Adelaide" // TODO: Use site location.
)

var (
	setupMutex sync.Mutex
	store      *CompositeStore
	debug      bool
	standalone bool
	notifier   notify.Notifier
	cronSecret []byte
	storePath  string
)

func main() {
	defaultPort := 8082
	v := os.Getenv("PORT")
	if v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			defaultPort = i
		}
	}

	var host string
	var port int
	flag.BoolVar(&debug, "debug", false, "Run in debug mode.")
	flag.BoolVar(&standalone, "standalone", false, "Run in standalone mode.")
	flag.StringVar(&host, "host", "localhost", "Host we run on in standalone mode")
	flag.IntVar(&port, "port", defaultPort, "Port we listen on in standalone mode")
	flag.StringVar(&storePath, "filestore", "store", "File store path")
	flag.Parse()

	// Perform one-time setup or bail.
	setup(context.Background())

	secrets, err := gauth.GetSecrets(context.Background(), projectID, nil)
	if err != nil {
		log.Fatalf("could not get secrets: %v", err)
	}

	publicKey, ok := secrets["mailjetPublicKey"]
	if !ok {
		log.Fatalf("could not get mailjetPublicKey, can't send panic recovery notification")
	}

	privateKey, ok := secrets["mailjetPrivateKey"]
	if !ok {
		log.Fatalf("could not get mailjetPrivateKey, can't send panic recovery notification")
	}

	mux := utils.NewRecoverableServeMux(
		utils.NewConfigurableRecoveryHandler(
			// Only consider handled if we can get a notification off.
			utils.WithHandledConditions(utils.HandledConditions{HandledOnNotification: true}),
			utils.WithLogOutput(log.Println),
			utils.WithNotification(func(msg string) error { return sendPanicNotification(publicKey, privateKey, msg) }),
			utils.WithHttpError(http.StatusInternalServerError),
			utils.WithHandlers(errNoGlobalNotifierHandler(secrets)),
		),
	)

	otv, err := newOceanTVService(
		withEventHooks(
			func(e event, cfg *Cfg) {
				// Only continue if we have a finished event.
				if _, ok := e.(finishedEvent); !ok {
					return
				}

				if !cfg.RegisterOpenFish {
					return
				}

				// Register stream with openfish so we can annotate the video.
				cs, err := strconv.Atoi(cfg.OpenFishCaptureSource)
				if err != nil {
					log.Printf("could not parse OpenFish capture source: %v", err)
					return
				}

				ofsvc, err := openfish.New()
				if err != nil {
					log.Printf("could not setup openfish service: %v", err)
					return
				}

				err = ofsvc.RegisterStream(cfg.SID, cs, cfg.Start, cfg.End)
				if err != nil {
					log.Printf("could not register stream with OpenFish: %v", err)
					return
				}
			},
		),
		withStateHooks(
			func(s state, cfg *Cfg) {

				// We're deactivating this webhook for the time being until it's been
				// properly configured.
				// Remove return to re-enable.
				log.Println("AusOceanTV webhook disabled")
				return

				// Only continue if we have a directLive state.
				// NOTE this can be removed if we wish to webhook for all states.
				if _, ok := s.(*directLive); !ok {
					return
				}

				data := struct {
					ID    string `json:"id"`
					State string `json:"state"`
				}{
					ID:    cfg.Name,
					State: stateToString(s),
				}
				const ausoceanTVWebHookDest = "https://www.ausocean.org/ausocean-tv/tvwebhook"
				err := sendWebhook(ausoceanTVWebHookDest, data)
				if err != nil {
					log.Printf("could not send AusOceanTV webhook: %v", err)
					return
				}
			},
		),
	)
	if err != nil {
		log.Fatalf("could not create oceanTV service: %v", err)
	}

	mux.HandleFunc("/_ah/warmup", warmupHandler)
	mux.HandleFunc("/broadcast/", broadcastHandler)
	mux.HandleFunc("/checkbroadcasts", otv.checkBroadcastsHandler)
	mux.HandleFunc("/", indexHandler)

	log.Printf("Listening on %s:%d", host, port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), mux))
}

func sendWebhook(url string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer func(b io.ReadCloser) {
		if err := b.Close(); err != nil {
			log.Printf("failed to close webhook response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusCreated {
		return nil
	}

	return fmt.Errorf("webhook request failed with status: %s", resp.Status)
}

func sendPanicNotification(publicKey, privateKey, msg string) error {
	const (
		sender   = "vidgrindservice@gmail.com"
		opsEmail = "ops@ausocean.org"
	)
	err := notify.Send(
		publicKey,
		privateKey,
		sender,
		[]string{opsEmail},
		"URGENT: Ocean TV Panic Recovery",
		msg,
	)
	if err != nil {
		return fmt.Errorf("could not send panic recovery email: %v", err)
	}
	return nil
}

func errNoGlobalNotifierHandler(secrets map[string]string) utils.RecoveryHandler {
	return func(w http.ResponseWriter, panicErr any) bool {
		err, ok := panicErr.(error)
		if !ok {
			return false
		}
		if errors.Is(err, errNoGlobalNotifier) {
			notifier, err = notify.NewMailjetNotifier(
				notify.WithSecrets(secrets),
				notify.WithRecipientLookup(tvRecipients),
				notify.WithStore(notify.NewStore(store)),
			)
			if err != nil {
				log.Printf("could not remediate missing global notifier: %v", err)
				return false
			}
			return true
		}
		return false
	}
}

// warmupHandler handles App Engine warmup requests. It simply ensures that the instance is loaded.
func warmupHandler(w http.ResponseWriter, r *http.Request) {
	indexHandler(w, r)
}

// indexHandler handles requests for the home page and is here just to
// test that the service is running. Clients do not use this endpoint.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	w.Write([]byte(projectID + " " + version))
}

// setup executes per-instance one-time initialization. Any errors are
// considered fatal.
func setup(ctx context.Context) {
	setupMutex.Lock()
	defer setupMutex.Unlock()

	if store != nil {
		return
	}

	var (
		err                       error
		settingsStore, mediaStore datastore.Store
	)
	if standalone {
		log.Printf("Running in standalone mode")
		settingsStore, err = datastore.NewStore(ctx, "file", "vidgrind", storePath)
		if err != nil {
			mediaStore = settingsStore
		}
	} else {
		log.Printf("Running in App Engine mode")
		settingsStore, err = datastore.NewStore(ctx, "cloud", "netreceiver", "")
		if err == nil {
			mediaStore, err = datastore.NewStore(ctx, "cloud", "vidgrind", "")
		}
	}
	if err != nil {
		log.Fatalf("could not set up datastore: %v", err)
	}
	model.RegisterEntities()

	store = ausOceanCompositeStore(settingsStore, mediaStore)

	cronSecret, err = gauth.GetHexSecret(ctx, projectID, "cronSecret")
	if err != nil || cronSecret == nil {
		log.Printf("could not get cronSecret: %v", err)
	}

	secrets, err := gauth.GetSecrets(ctx, projectID, nil)
	if err != nil {
		log.Fatalf("could not get secrets: %v", err)
	}

	notifier, err = notify.NewMailjetNotifier(
		notify.WithSecrets(secrets),
		notify.WithRecipientLookup(tvRecipients),
		notify.WithStore(notify.NewStore(store)),
	)
	if err != nil {
		log.Fatalf("could not set up email notifier: %v", err)
	}
}

// tvRecipients looks up the email addresses and notification period
// for the given site,
func tvRecipients(skey int64, kind notify.Kind) ([]string, time.Duration, error) {
	ctx := context.Background()
	site, err := model.GetSite(ctx, store, skey)
	if err != nil {
		return nil, 0, fmt.Errorf("error getting site: %w", err)
	}
	if site.OpsEmail == "" {
		log.Printf("OpsEmail not defined for site %s", site.Name)
	}
	recipients := []string{site.OpsEmail}
	switch kind {
	case broadcastHardware, broadcastNetwork, broadcastConfiguration:
		if site.YouTubeEmail == "" {
			log.Printf("YouTubeEmail not defined for site %s", site.Name)
			break
		}
		recipients = append(recipients, site.YouTubeEmail)
	default:
		// Skip YouTubeEmail notifications for other kinds.
	}
	return recipients, time.Duration(site.NotifyPeriod) * time.Hour, nil
}

// broadcastHandler handles broadcast save requests from broadcast clients.
// These take the form: /broadcast/op
// TODO: Add JWT signing
func broadcastHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	ctx := r.Context()
	setup(ctx)

	req := strings.Split(r.URL.Path, "/")
	if len(req) != 3 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid URL length"))
		return
	}

	op := req[2]
	if op != "save" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid operation: %s", op))
		return
	}

	ct := r.Header.Get("Content-Type")
	if ct != "application/json" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unexpected Content-Type: %s", ct))
		return
	}

	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var cfg BroadcastConfig
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	log := func(msg string, args ...interface{}) {
		logForBroadcast(&cfg, log.Println, msg, args...)
	}

	// Use the broadcast manager to save the broadcast.
	// We can provide a nil BroadcastService given that Save
	// won't need this.
	err = newOceanBroadcastManager(nil, &cfg, store, log).Save(ctx, func(_cfg *Cfg) {
		// Update only the fields that can be updated via the UI.
		// NOTE: This needs to be kept in sync with the UI. To aid this, the fields
		// have been updated in the same order which they're currently being updated on oceanbench.

		// Values parsed initially from the form submission.
		_cfg.SKey = cfg.SKey
		_cfg.Name = cfg.Name
		_cfg.ID = cfg.ID
		_cfg.StreamName = cfg.StreamName
		_cfg.Description = cfg.Description
		_cfg.LivePrivacy = cfg.LivePrivacy
		_cfg.PostLivePrivacy = cfg.PostLivePrivacy
		_cfg.Resolution = cfg.Resolution
		_cfg.StartTimestamp = cfg.StartTimestamp
		_cfg.EndTimestamp = cfg.EndTimestamp
		_cfg.RTMPVar = cfg.RTMPVar
		_cfg.RTMPKey = cfg.RTMPKey
		_cfg.VidforwardHost = cfg.VidforwardHost
		_cfg.CameraMac = cfg.CameraMac
		_cfg.ControllerMAC = cfg.ControllerMAC
		_cfg.OnActions = cfg.OnActions
		_cfg.OffActions = cfg.OffActions
		_cfg.ShutdownActions = cfg.ShutdownActions
		_cfg.SendMsg = cfg.SendMsg
		_cfg.UsingVidforward = cfg.UsingVidforward
		_cfg.CheckingHealth = cfg.CheckingHealth
		_cfg.Enabled = cfg.Enabled
		_cfg.InFailure = cfg.InFailure
		_cfg.RegisterOpenFish = cfg.RegisterOpenFish
		_cfg.OpenFishCaptureSource = cfg.OpenFishCaptureSource
		_cfg.BatteryVoltagePin = cfg.BatteryVoltagePin

		// Values that are parsed into floats from their form values.
		_cfg.RequiredStreamingVoltage = cfg.RequiredStreamingVoltage
		_cfg.VoltageRecoveryTimeout = cfg.VoltageRecoveryTimeout

		// Calculated based on the Start and End timestamps.
		_cfg.Start = cfg.Start
		_cfg.End = cfg.End

		// Only updated if coming out of failure.
		if cfg.HardwareState == "hardwareOff" {
			_cfg.HardwareState = cfg.HardwareState
		}

		// Either stays the same or is updated via a authentication pipeline.
		_cfg.Account = cfg.Account

		// Currently not updated via the UI.
		// Active
		// Slate
		// Issues
		// SensorList
		// AttemptingToStart
		// Events
		// Unhealthy
		// BroadcastState
		// StartFailures
		// Transitioning
		// StateData
		// HardwareStateData
		// RecoveringVoltage
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	log("broadcast saved")
	w.WriteHeader(http.StatusOK)
}

// writeError writes HTTP errors to the response writer.
func writeError(w http.ResponseWriter, code int, err error) {
	log.Printf(err.Error())
	http.Error(w, http.StatusText(code)+":"+err.Error(), code)
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
