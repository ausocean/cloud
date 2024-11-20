/*
AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This file is part of AusOcean TV. AusOcean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  AusOcean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with AusOcean TV in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

// AusOcean TV is a cloud service serving AusOcean live streaming content and more.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

// Project constants.
const (
	projectID     = "ausoceantv"
	version       = "v0.1.1"
	oauthClientID = "1005382600755-7st09cc91eqcqveviinitqo091dtcmf0.apps.googleusercontent.com"
	oauthMaxAge   = 60 * 60 * 24 * 7 // 7 days
)

const (
	localEmail = "localuser@localhost"
)

// service defines the properties of our web service.
type service struct {
	setupMutex    sync.Mutex
	settingsStore datastore.Store
	debug         bool
	standalone    bool
	storePath     string
	auth          *gauth.UserAuth
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

	http.HandleFunc("/api/", app.apiHandler)
	http.HandleFunc("/auth/login", loginHandler)
	http.HandleFunc("/auth/logout", logoutHandler)
	http.HandleFunc("/auth/getprofile", profileHandler)
	http.HandleFunc("/auth/oauth2callback", oauthCallbackHandler)

	if !app.standalone {
		log.Printf("Initializing OAuth2")
		app.auth = &gauth.UserAuth{ProjectID: projectID, ClientID: oauthClientID, MaxAge: oauthMaxAge}
		app.auth.Init()
	}

	log.Printf("Listening on %s:%d", host, port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil))
}

// apiHandler handles requests for the ausoceantv API.
func (svc *service) apiHandler(w http.ResponseWriter, r *http.Request) {
	svc.logRequest(r)
	w.Write([]byte(projectID + " " + version))
}

// loginHandler handles user login requests.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	app.logRequest(r)
	if app.standalone {
		return
	}
	err := app.auth.LoginHandler(w, r)
	if err != nil {
		writeError(w, err)
	}
}

// logoutHandler handles user logout requests.
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	app.logRequest(r)
	if app.standalone {
		return
	}
	err := app.auth.LogoutHandler(w, r)
	if err != nil {
		writeError(w, err)
	}
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	p, err := app.auth.GetProfile(w, r)
	if err != nil {
		writeError(w, err)
	}

	// Write JSON profile data.
	b, err := json.Marshal(p)
	if err != nil {
		writeError(w, err)
	}
	w.Write(b)
}

// oauthCallbackHandler implements the OAuth2 callback that completes the authentication process.
func oauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	app.logRequest(r)
	if app.standalone {
		return
	}
	err := app.auth.CallbackHandler(w, r)
	if err != nil {
		writeError(w, err)
	}
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
		svc.settingsStore, err = datastore.NewStore(ctx, "file", "vidgrind", svc.storePath)
	} else {
		log.Printf("Running in App Engine mode")
		svc.settingsStore, err = datastore.NewStore(ctx, "cloud", "netreceiver", "")
	}
	if err != nil {
		log.Fatalf("could not set up datastore: %v", err)
	}
	model.RegisterEntities()
	log.Printf("set up datastore")
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

// writeError writes an error in JSON format.
func writeError(w http.ResponseWriter, err error) {
	w.Header().Add("Content-Type", "application/json")
	err2 := json.NewEncoder(w).Encode(map[string]string{"er": err.Error()})
	if err2 != nil {
		log.Printf("failed to write error (%v): %v", err, err2)
		return
	}
	if app.debug {
		log.Println("Wrote error: " + err.Error())
	}
}
