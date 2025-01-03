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
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/encryptcookie"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/ausocean/cloud/backend"
	"github.com/ausocean/cloud/cmd/ausoceantv/dsclient"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/cmd/openfish/api"
	"github.com/ausocean/openfish/datastore"
)

// Project constants.
const (
	projectID     = "ausoceantv"
	oauthClientID = "1005382600755-7st09cc91eqcqveviinitqo091dtcmf0.apps.googleusercontent.com"
	oauthMaxAge   = 60 * 60 * 24 * 7 // 7 days.
	version       = "v0.2.2"
)

// service defines the properties of our web service.
type service struct {
	setupMutex    sync.Mutex
	settingsStore datastore.Store
	debug         bool
	standalone    bool
	development   bool
	storePath     string
	auth          *gauth.UserAuth
}

// svc is an instance of our service.
var svc *service = &service{}

func registerAPIRoutes(app *fiber.App) {
	v1 := app.Group("/api/v1")

	// Authentication Routes.
	v1.Group("/auth").
		Get("/login", svc.loginHandler).
		Get("/logout", svc.logoutHandler).
		Get("oauth2callback", svc.callbackHandler).
		Get("profile", svc.profileHandler)

	v1.Get("version", svc.versionHandler)

	v1.Group("/stripe").
		Options("/create-payment-intent", svc.preFlightOK).
		Post("/create-payment-intent", svc.handleCreatePaymentIntent).
		Post("/cancel", svc.cancelSubscription)

	v1.Group("/get").
		Get("/subscription", svc.getSubscriptionHandler)
}

func main() {
	defaultPort := 8084
	v := os.Getenv("PORT")
	if v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			defaultPort = i
		}
	}

	v = os.Getenv("DEVELOPMENT")
	if v != "" {
		svc.development = true
	}

	var host string
	var port int
	flag.BoolVar(&svc.debug, "debug", false, "Run in debug mode.")
	flag.BoolVar(&svc.standalone, "standalone", false, "Run in standalone mode.")
	flag.StringVar(&host, "host", "localhost", "Host we run on in standalone mode")
	flag.IntVar(&port, "port", defaultPort, "Port we listen on in standalone mode")
	flag.StringVar(&svc.storePath, "filestore", "store", "File store path")
	flag.Parse()

	// Create app.
	app := fiber.New(fiber.Config{ErrorHandler: api.ErrorHandler})

	// Encrypt cookies.
	// NOTE: This must be done before any middleware which uses cookies.
	ctx := context.Background()
	key, err := gauth.GetSecret(ctx, projectID, "sessionKey")
	if err != nil {
		log.Panicf("unable to get sessionKey secret: %v", err)
	}
	app.Use(encryptcookie.New(encryptcookie.Config{
		Key: key,
	}))

	// Perform one-time setup or bail.
	svc.setup(ctx)

	// Recover from panics.
	app.Use(recover.New())

	// CORS middleware.
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
	}))

	// Set the logging level.
	if svc.debug {
		log.SetLevel(log.LevelDebug)
	} else if svc.standalone {
		log.SetLevel(log.LevelInfo)
	} else {
		// Appengine logs requests for us.
		log.SetLevel(log.LevelError)
	}

	// Add logging middleware to log requests if applicable.
	app.Use(func(ctx *fiber.Ctx) error {
		log.Info(ctx.Path())
		return ctx.Next()
	})

	// Register routes.
	registerAPIRoutes(app)

	// Start web server.
	listenOn := fmt.Sprintf(":%d", port)
	fmt.Printf("starting web server on %s\n", listenOn)
	log.Fatal(app.Listen(listenOn))
}

// preFlightOK returns a statusOK message to preflight messages.
func (svc *service) preFlightOK(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

// versionHandler handles requests for the ausoceantv API.
func (svc *service) versionHandler(ctx *fiber.Ctx) error {
	ctx.WriteString(projectID + " " + version)
	return nil
}

// setup executes per-instance one-time warmup and is used to
// initialize the service. Any errors are considered fatal.
//
// NOTE: This function must be called before any middleware which uses
// cookies is attached to the app.
func (svc *service) setup(ctx context.Context) {
	svc.setupMutex.Lock()
	defer svc.setupMutex.Unlock()

	if svc.settingsStore != nil {
		return
	}

	err := dsclient.Init(svc.standalone, svc.storePath)
	if err != nil {
		log.Fatalf("could not set up datastore: %v", err)
	}
	svc.settingsStore = dsclient.Get()
	model.RegisterEntities()
	log.Info("set up datastore")

	svc.setupStripe(ctx)

	// Initialise OAuth2.
	log.Info("Initializing OAuth2")
	svc.auth = &gauth.UserAuth{ProjectID: projectID, ClientID: oauthClientID, MaxAge: oauthMaxAge}
	svc.auth.Init(backend.NewFiberHandler(nil))
}

func (svc *service) getSubscriptionHandler(c *fiber.Ctx) error {
	ctx := context.Background()
	p, err := svc.auth.GetProfile(backend.NewFiberHandler(c))
	if errors.Is(err, gauth.SessionNotFound) || errors.Is(err, gauth.TokenNotFound) {
		return fiber.NewError(fiber.StatusUnauthorized, fmt.Sprintf("error getting profile: %v", err))
	} else if err != nil {
		return fmt.Errorf("unable to get profile: %w", err)
	}

	subscriber, err := model.GetSubscriberByEmail(ctx, svc.settingsStore, p.Email)
	if err != nil {
		return fmt.Errorf("error getting subscriber by email for: %s: %w", p.Email, err)
	}

	subscription, err := model.GetSubscription(ctx, svc.settingsStore, subscriber.ID, model.NoFeedID)
	if err != nil {
		return fmt.Errorf("error getting subscription for id: %d: %w", subscriber.ID, err)
	}

	// Check that the current time is prior to the end date of the subscription.
	// ie. the subscription hasn't expired and is still valid.
	if subscription.Finish.Before(time.Now()) {
		return c.JSON(nil)
	}

	return c.JSON(subscription)
}
