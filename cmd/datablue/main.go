/*
LICENSE
  Copyright (C) 2024-2025 the Australian Ocean Lab (AusOcean)

  This file is part of Data Blue. This is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Data Blue is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with Data Blue in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

// Data Blue is the device data handler for Cloud Blue, the AusOcean ocean data cloud.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

const (
	version   = "v0.8.1"
	projectID = "datablue"
)

var (
	setupMutex    sync.Mutex
	mediaStore    datastore.Store
	settingsStore datastore.Store
	debug         bool
	standalone    bool
	storePath     string
)

func main() {
	defaultPort := 8083
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

	// Perform one-time setup.
	setup(context.Background())

	// Device requests.
	http.HandleFunc("/config", configHandler)
	http.HandleFunc("/poll", pollHandler)
	http.HandleFunc("/act", actHandler)
	http.HandleFunc("/vars", varsHandler)
	http.HandleFunc("/mts", mtsHandler)
	http.HandleFunc("/recv", mtsHandler) // For backwards compatibility.
	http.HandleFunc("/api", apiHandler)
	http.HandleFunc("/api/", apiHandler)

	// Other requests
	http.HandleFunc("/_ah/warmup", warmupHandler)
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
	w.Write([]byte(projectID + " " + version))
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

// setupLocal creates a local site and device for use in standalone mode.
// In standalone mode all data is associated with site 1.
func setupLocal(ctx context.Context, store datastore.Store) error {
	err := model.PutSite(ctx, store, &model.Site{Skey: 1, Name: "localhost", Enabled: true})
	if err != nil {
		return err
	}
	err = model.PutDevice(ctx, store, &model.Device{Skey: 1, Mac: 1, Dkey: 0, Name: "localdevice", Inputs: "A0,V0,S0", MonitorPeriod: 60, Enabled: true})
	return err
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
