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
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/paymentintent"
)

// Project constants.
const (
	projectID = "ausoceantv"
	version   = "v0.0.1"
)

// service defines the properties of our web service.
type service struct {
	setupMutex    sync.Mutex
	settingsStore datastore.Store
	debug         bool
	standalone    bool
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

	// Get stripe secret key.
	v = os.Getenv("AUSOCEAN_STRIPE_SECRET_KEY")
	if v == "" {
		log.Println("AUSOCEAN_STRIPE_SECRET_KEY not found, cannot take payments")
	} else {
		stripe.Key = v
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

	// Serve static files when running locally.
	http.Handle("/s/", http.StripPrefix("/s", http.FileServer(http.Dir("s"))))

	http.HandleFunc("/", app.indexHandler)

	// Stripe integration endpoints.
	http.HandleFunc("/stripe/create-payment-intent", handleCreatePaymentIntent)

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

// handleCreatePaymentIntent handles requests to /stripe/create-payment-intent.
func handleCreatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	app.logRequest(r)

	// Enable Cross-Origin Scripting from Vite.
	enableCors(&w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(200)
		return
	} else if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// TODO: Get product details.
	//	description := product.description
	//	price := calculatePrice(product)

	// Enable auto payment method for better conversions.
	autoPaymentMethodEnabled := true

	// Create a PaymentIntent with amount and currency.
	params := &stripe.PaymentIntentParams{
		Amount:                  stripe.Int64(1099),
		Currency:                stripe.String(string(stripe.CurrencyAUD)),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{Enabled: &autoPaymentMethodEnabled},
	}

	// NOTE: DO NOT LOG PAYMENT INTENT.
	pi, err := paymentintent.New(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error creating new Stripe payment intent: %v", err)
		return
	}

	writeJSON(w, struct {
		ClientSecret string `json:"clientSecret"`
		// DpmCheckerLink string `json:"dpmCheckerLink"` <-- Can be used for debugging of the integration.
	}{
		ClientSecret: pi.ClientSecret,
		// [DEV]: For demo purposes only, you should avoid exposing the PaymentIntent ID in the client-side code.
		// DpmCheckerLink: fmt.Sprintf("https://dashboard.stripe.com/settings/payment_methods/review?transaction_id=%s", pi.ID),
	})
}

// enableCors allows the vite server to read responses from this webserver.
func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	(*w).Header().Set("Access-Control-Content-Type", "application/json")

	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("json.NewEncoder.Encode: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.Copy(w, &buf); err != nil {
		log.Printf("io.Copy: %v", err)
		return
	}
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
