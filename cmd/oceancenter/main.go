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
// - device enabling and disabling
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
	projectID = "oceancenter"
	version   = "v0.1.0"
)

// service defines the properties of our web service.
type service struct {
	setupMutex    sync.Mutex
	settingsStore datastore.Store
	debug         bool
	standalone    bool
}

// app is an instance of our service.
var app *service = &service{}

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
	flag.BoolVar(&app.debug, "debug", false, "Run in debug mode.")
	flag.BoolVar(&app.standalone, "standalone", false, "Run in standalone mode.")
	flag.StringVar(&host, "host", "localhost", "Host we run on in standalone mode")
	flag.IntVar(&port, "port", defaultPort, "Port we listen on in standalone mode")
	flag.Parse()

	// Perform one-time setup or bail.
	ctx := context.Background()
	app.setup(ctx)

	// Serve static files when running locally
	http.Handle("/s/", http.StripPrefix("/s", http.FileServer(http.Dir("s"))))
	http.Handle("/dl/", http.StripPrefix("/dl", http.FileServer(http.Dir("dl"))))

	http.HandleFunc("/", app.indexHandler)

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
	if svc.standalone {
		log.Printf("Running in standalone mode")
		svc.settingsStore, err = datastore.NewStore(ctx, "file", "vidgrind", "store")
	} else {
		log.Printf("Running in App Engine mode")
		svc.settingsStore, err = datastore.NewStore(ctx, "cloud", "netreceiver", "")
	}
	if err != nil {
		log.Fatalf("could not set up datastore: %v", err)
	}
	model.RegisterEntities()
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
