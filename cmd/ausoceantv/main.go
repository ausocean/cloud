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
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/ausocean/cloud/cmd/ausoceantv/dsclient"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/cmd/openfish/api"
	"github.com/ausocean/openfish/datastore"
)

// Project constants.
const (
	projectID = "ausoceantv"
	version   = "v0.1.2"
)

// service defines the properties of our web service.
type service struct {
	setupMutex    sync.Mutex
	settingsStore datastore.Store
	debug         bool
	standalone    bool
	storePath     string
}

// svc is an instance of our service.
var svc *service = &service{}

func registerAPIRoutes(app *fiber.App) {
	v1 := app.Group("/api/v1")
	v1.Get("version", svc.versionHandler)

	v1.Group("/stripe").
		Options("/create-payment-intent", svc.preFlightOK).
		Post("/create-payment-intent", svc.handleCreatePaymentIntent)
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

	var host string
	var port int
	flag.BoolVar(&svc.debug, "debug", false, "Run in debug mode.")
	flag.BoolVar(&svc.standalone, "standalone", false, "Run in standalone mode.")
	flag.StringVar(&host, "host", "localhost", "Host we run on in standalone mode")
	flag.IntVar(&port, "port", defaultPort, "Port we listen on in standalone mode")
	flag.StringVar(&svc.storePath, "filestore", "store", "File store path")
	flag.Parse()

	// Perform one-time setup or bail.
	ctx := context.Background()
	svc.setup(ctx)

	// Create app.
	app := fiber.New(fiber.Config{ErrorHandler: api.ErrorHandler})

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
	model.RegisterEntities()
	log.Info("set up datastore")

	svc.setupStripe(ctx)
}
