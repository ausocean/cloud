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
	"context"
	"encoding/json"
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

	"bitbucket.org/ausocean/iotsvc/iotds"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/notify"
)

const (
	projectID          = "oceantv"
	projectURL         = "https://oceantv.appspot.com"
	cronServiceAccount = "oceancron@appspot.gserviceaccount.com"
	senderEmail        = "vidgrindservice@gmail.com" // TODO: Change this.
	locationID         = "Australia/Adelaide"        // TODO: Use site location.
)

var (
	setupMutex    sync.Mutex
	settingsStore iotds.Store
	mediaStore    iotds.Store
	debug         bool
	standalone    bool
	notifier      notify.Notifier
	cronSecret    []byte
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
	flag.Parse()

	// Perform one-time setup or bail.
	setup(context.Background())

	http.HandleFunc("/_ah/warmup", warmupHandler)
	http.HandleFunc("/broadcast/", broadcastHandler)
	http.HandleFunc("/checkbroadcasts", checkBroadcastsHandler)
	http.HandleFunc("/", indexHandler)

	log.Printf("Listening on %s:%d", host, port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil))
}

// warmupHandler handles App Engine warmup requests. It simply ensures that the instance is loaded.
func warmupHandler(w http.ResponseWriter, r *http.Request) {
	indexHandler(w, r)
}

// indexHandler handles requests for the home page and is here just to
// test that the service is running. Devices do not use this endpoint.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	w.Write([]byte("OK"))
}

// setup executes per-instance one-time initialization. Any errors are
// considered fatal.
func setup(ctx context.Context) {
	setupMutex.Lock()
	defer setupMutex.Unlock()

	if settingsStore != nil {
		return
	}

	var err error
	if standalone {
		log.Printf("Running in standalone mode")
		settingsStore, err = iotds.NewStore(ctx, "file", projectID, "store")
		if err != nil {
			mediaStore = settingsStore
		}
	} else {
		log.Printf("Running in App Engine mode")
		settingsStore, err = iotds.NewStore(ctx, "cloud", "netreceiver", "")
		if err != nil {
			mediaStore, err = iotds.NewStore(ctx, "cloud", "vidgrind", "")
		}
	}
	if err != nil {
		log.Fatalf("could not set up datastore: %v", err)
	}
	iotds.RegisterEntities()

	cronSecret, err = gauth.GetHexSecret(ctx, projectID, "cronSecret")
	if err != nil || cronSecret == nil {
		log.Printf("could not get cronSecret: %v", err)
	}

	err = notifier.Init(ctx, projectID, senderEmail, &timeStore{})
	if err != nil {
		log.Fatalf("could not set up email notifier: %v", err)
	}
}

// timeStore implements a notify.TimeStore that uses iotds.Variable for persistence.
type timeStore struct {
}

// Get retrieves a notification time stored in an iotds.Variable.
// We prepend an underscore to keep the variable private.
func (ts *timeStore) Get(skey int64, key string) (time.Time, error) {
	v, err := iotds.GetVariable(context.Background(), settingsStore, skey, "_"+key)
	switch err {
	case nil:
		return v.Updated, nil
	case iotds.ErrNoSuchEntity:
		return time.Time{}, nil // We've never sent this kind of notice previously.
	default:
		return time.Time{}, err // Unexpected datastore error.
	}
}

// Set updates a notification time stored in an iotds.Variable.
func (ts *timeStore) Set(skey int64, key string, t time.Time) error {
	return iotds.PutVariable(context.Background(), settingsStore, skey, "_"+key, "")
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

	err = (&OceanBroadcastManager{}).SaveBroadcast(ctx, &cfg, settingsStore)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	log.Printf("broadcast %s saved", cfg.Name)
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