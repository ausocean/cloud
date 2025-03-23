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
	"encoding/base64"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"

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
	projectID     = "oceanbench"
	oauthClientID = "802166617157-v67emnahdpvfuc13ijiqb7qm3a7sf45b.apps.googleusercontent.com"
	oauthMaxAge   = 60 * 60 * 24 * 7 // 7 days.
	version       = "v0.5.7"
)

// service defines the properties of our web service.
type service struct {
	setupMutex    sync.Mutex
	mediaStore    datastore.Store
	settingsStore datastore.Store
	debug         bool
	standalone    bool
	development   bool
	lite          bool
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

	v = os.Getenv("LITE")
	if v != "" {
		svc.lite = true
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
	app := fiber.New(fiber.Config{ErrorHandler: api.ErrorHandler, ReadBufferSize: 8192})

	// Encrypt cookies.
	// NOTE: This must be done before any middleware which uses cookies.
	ctx := context.Background()
	keyBytes, err := gauth.GetHexSecret(ctx, projectID, "sessionKey")
	if err != nil {
		log.Panicf("unable to get sessionKey secret: %v", err)
	}
	if len(keyBytes) != 16 && len(keyBytes) != 24 && len(keyBytes) != 32 {
		log.Panicf("sessionKey has invalid length %d", len(keyBytes))
	}
	app.Use(encryptcookie.New(encryptcookie.Config{
		Key: base64.StdEncoding.EncodeToString(keyBytes),
	}))

	// Perform one-time setup or bail.
	svc.setup(ctx)

	auth := &gauth.UserAuth{ProjectID: projectID, ClientID: oauthClientID, MaxAge: oauthMaxAge}
	auth.Init(backend.NewNetHandler(nil, nil, nil))

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

	if svc.mediaStore != nil {
		return
	}

	err := dsclient.Init(svc.standalone, svc.storePath)
	if err != nil {
		log.Fatalf("could not set up datastore: %v", err)
	}
	svc.mediaStore = dsclient.Get()
	model.RegisterEntities()
	log.Info("set up datastore")

	// Initialise OAuth2.
	log.Info("Initializing OAuth2")
	svc.auth = &gauth.UserAuth{ProjectID: projectID, ClientID: oauthClientID, MaxAge: oauthMaxAge}
	svc.auth.Init(backend.NewFiberHandler(nil))
}

type loggingErrorOption func(c *fiber.Ctx, m map[string]string) error

// withStatus sets the status of the response.
func withStatus(status int) loggingErrorOption {
	return func(c *fiber.Ctx, m map[string]string) error {
		c.Status(status)
		return nil
	}
}

// withUserMessage updates the message that will be sent to the frontend,
// this is intended for user readable messages.
func withUserMessage(userMsg string) loggingErrorOption {
	return func(c *fiber.Ctx, m map[string]string) error {
		m["user-message"] = userMsg
		return nil
	}
}

// logAndReturnError logs the passed message as an error and returns an response to the client.
// The response code defaults to internal server error (500) and the message defaults to the status text.
func logAndReturnError(c *fiber.Ctx, message string, opts ...loggingErrorOption) error {
	fmt.Println(message)
	log.Error(message)
	c.Status(fiber.StatusInternalServerError)
	kv := make(map[string]string)
	kv["message"] = http.StatusText(c.Response().StatusCode())
	kv["error"] = message
	for i, opt := range opts {
		err := opt(c, kv)
		if err != nil {
			log.Errorf("error applying opt[%d]: %v", i, err)
		}
	}
	return c.JSON(kv)
}
