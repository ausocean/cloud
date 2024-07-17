/*
LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Cron. Ocean Cron is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean Cron is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with Ocean Cron in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

// Ocean Cron is a cloud service running cron jobs.
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

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/notify"
	"github.com/ausocean/openfish/datastore"
)

const (
	projectID          = "oceancron"
	version            = "v0.1.0"
	cronServiceURL     = "https://oceancron.appspot.com"
	cronServiceAccount = "oceancron@appspot.gserviceaccount.com"
)

var (
	setupMutex    sync.Mutex
	settingsStore datastore.Store
	debug         bool
	standalone    bool
	auth          *gauth.UserAuth
	cronScheduler *scheduler
	cronSecret    []byte
	notifier      notify.Notifier
)

func main() {
	defaultPort := 8081
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

	http.HandleFunc("/_ah/warmup", warmupHandler)
	http.HandleFunc("/cron/", cronHandler)
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
// initialize the datastore, set up the cron scheduler, and set up the
// notifier. Any errors are considered fatal.
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

	cronSecret, err = gauth.GetHexSecret(ctx, projectID, "cronSecret")
	if err != nil || cronSecret == nil {
		log.Printf("could not get cronSecret: %v", err)
	}

	err = setupCronScheduler(ctx)
	if err != nil {
		log.Fatalf("could not set up cron scheduler: %v", err)
	}

	secrets, err := gauth.GetSecrets(ctx, projectID, nil)
	if err != nil {
		log.Fatalf("could not get secrets: %v", err)
	}
	recipient, period := notify.GetOpsEnvVars()
	err = notifier.Init(notify.WithSecrets(secrets), notify.WithRecipient(recipient), notify.WithStore(notify.NewTimeStore(settingsStore, period)))
	if err != nil {
		log.Fatalf("could not set up email notifier: %v", err)
	}
}

// setupCronScheduler starts a cron scheduler and loads all stored jobs.
func setupCronScheduler(ctx context.Context) error {
	var err error
	cronScheduler, err = newScheduler()
	if err != nil {
		return fmt.Errorf("could not create new scheduler: %w", err)
	}

	sites, err := model.GetAllSites(ctx, settingsStore)
	if err != nil {
		if sites == nil {
			return fmt.Errorf("could not get sites for cron initialization: %v", err)
		}
		log.Printf("got sites for cron initialization but encountered error: %v", err)
	}
	for _, site := range sites {
		crons, err := model.GetCronsBySite(ctx, settingsStore, site.Skey)
		if err != nil {
			log.Printf("failed to get crons from site=%d: %v", site.Skey, err)
			continue
		}
		for j := range crons {
			err = cronScheduler.Set(&crons[j])
			if err != nil {
				log.Printf("failed to set job %v from site=%d: %v", crons[j], site.Skey, err)
			}
		}
		log.Printf("set %d crons for site=%d", len(crons), site.Skey)
	}

	return nil
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
