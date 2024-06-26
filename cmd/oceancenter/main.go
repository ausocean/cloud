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

// Ocean Center is a cloud service for remove device management, including:
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

var (
	setupMutex    sync.Mutex
	settingsStore datastore.Store
	debug         bool
	standalone    bool
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
	flag.Parse()

	// Perform one-time setup or bail.
	ctx := context.Background()
	setup(ctx)

	// Serve static files when running locally
	http.Handle("/s/", http.StripPrefix("/s", http.FileServer(http.Dir("s"))))
	http.Handle("/dl/", http.StripPrefix("/dl", http.FileServer(http.Dir("dl"))))

	http.HandleFunc("/install", installHandler)
	http.HandleFunc("/", indexHandler)

	log.Printf("Listening on %s:%d", host, port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil))
}

// indexHandler handles requests for the home page and is here just to
// test that the service is running. Devices do not use this endpoint.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	w.Write([]byte(projectID + " " + version))
}

// setup executes per-instance one-time warmup and is used to
// initialize the datastore. Any errors are considered fatal.
func setup(ctx context.Context) {
	setupMutex.Lock()
	defer setupMutex.Unlock()

	if settingsStore != nil {
		return
	}

	var err error
	if standalone {
		log.Printf("Running in standalone mode")
		settingsStore, err = datastore.NewStore(ctx, "file", "vidgrind", "store")
	} else {
		log.Printf("Running in App Engine mode")
		settingsStore, err = datastore.NewStore(ctx, "cloud", "netreceiver", "")
	}
	if err != nil {
		log.Fatalf("could not set up datastore: %v", err)
	}
	model.RegisterEntities()
}

// installHandler handles all software installation requests originating from a device.
// TODO: Implement this.
func installHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
}

// writeError writes http errors to the response writer, in order to provide more detailed
// response errors in a concise manner.
func writeError(w http.ResponseWriter, code int, msg string, args ...interface{}) {
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
