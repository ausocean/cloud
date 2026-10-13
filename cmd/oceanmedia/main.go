/*
DESCRIPTION
  OceanMedia generates HLS playlists for OceanMedia streams.

AUTHORS
  Elliot Shine <elliot@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean).

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
  or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
  License for more details.

  You should have received a copy of the GNU General Public License in
  gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

// OceanMedia serves HLS playlists for OceanMedia broadcasts. A
// broadcast is made up of a BroadcastEvent and a series of MtsMediaV2
// segments whose media is stored in an object store bucket. Playlists
// are generated on demand with pre-signed segment URLs so that players
// fetch media directly from the bucket.
//
// Playlists support full refreshes, LL-HLS blocking reloads
// (_HLS_msn), delta updates (_HLS_skip) and DVR of the whole stream.
package main

import (
	_ "github.com/joho/godotenv/autoload"

	"context"
	"flag"
	"fmt"
	"log"
	"os"
	rtdebug "runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/storage"
	"github.com/gofiber/fiber/v2"
)

const (
	projectID = "oceanmedia"
	version   = "v0.1.0"
)

// Secret names within OCEANMEDIA_SECRETS.
const (
	secretJWT               = "jwtSecret"           // JWT signing secret shared with referring services.
	secretCloudflareAccount = "cloudflareAccountID" // Cloudflare R2 account ID.
	secretCloudflareAccess  = "cloudflareAccessKey" // Cloudflare R2 access key ID.
	secretCloudflareSecret  = "cloudflareSecretKey" // Cloudflare R2 secret access key.
)

var (
	setupMutex sync.Mutex
	mediaStore datastore.Store
	srv        *server
	debug      bool
	standalone bool
	storePath  string
	commitHash string
)

func init() {
	if info, ok := rtdebug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				commitHash = setting.Value
				if len(commitHash) > 7 {
					commitHash = commitHash[:7]
				}
				break
			}
		}
	}
	if commitHash == "" {
		commitHash = os.Getenv("COMMIT_HASH")
	}
}

func main() {
	defaultPort := 8085
	v := os.Getenv("PORT")
	if v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			defaultPort = i
		}
	}

	var (
		host            string
		port            int
		signedURLExpiry time.Duration
	)
	flag.BoolVar(&debug, "debug", false, "Run in debug mode.")
	flag.BoolVar(&standalone, "standalone", false, "Run in standalone mode.")
	flag.StringVar(&host, "host", "localhost", "Host we run on in standalone mode")
	flag.IntVar(&port, "port", defaultPort, "Port we listen on in standalone mode")
	flag.StringVar(&storePath, "filestore", "store", "File store path")
	flag.DurationVar(&signedURLExpiry, "signedurlttl", 12*time.Hour, "Pre-signed segment URL time to live")
	flag.Parse()

	// Perform one-time setup or bail.
	setup(context.Background(), signedURLExpiry)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Get("/", indexHandler)
	app.Get("/health", indexHandler)
	app.Get("/_ah/warmup", indexHandler)

	// Playlists are requested as /<broadcastID>.m3u8.
	app.Get("/:file", srv.handlePlaylist)

	log.Printf("Listening on %s:%d", host, port)
	log.Fatal(app.Listen(fmt.Sprintf("%s:%d", host, port)))
}

// indexHandler handles health and liveness requests.
func indexHandler(c *fiber.Ctx) error {
	if commitHash != "" {
		return c.SendString(projectID + " " + version + " (" + commitHash + ")")
	}
	return c.SendString(projectID + " " + version)
}

// setup executes per-instance one-time warmup and is used to
// initialize the datastores, secrets and signer.
func setup(ctx context.Context, signedURLExpiry time.Duration) {
	setupMutex.Lock()
	defer setupMutex.Unlock()

	if srv != nil {
		return
	}

	var err error
	_, mediaStore, err = model.SetupDatastore(standalone, false, storePath, ctx)
	if err != nil {
		log.Fatalf("could not setup datastores: %v", err)
	}

	secrets, err := gauth.GetSecrets(ctx, projectID, nil)
	if err != nil {
		log.Fatalf("could not get secrets: %v", err)
	}

	jwtSecret := []byte(secrets[secretJWT])
	if len(jwtSecret) == 0 {
		log.Fatalf("could not get %s, cannot validate tokens", secretJWT)
	}
	if secrets[secretCloudflareAccount] == "" {
		log.Fatalf("could not get %s, cannot sign segment URLs", secretCloudflareAccount)
	}
	if secrets[secretCloudflareAccess] == "" || secrets[secretCloudflareSecret] == "" {
		log.Fatalf("could not get %s/%s, cannot sign segment URLs", secretCloudflareAccess, secretCloudflareSecret)
	}

	// The provider's account ID is used to reject segment URIs belonging to
	// another account. The bucket is taken from each segment's StorageURI.
	provider := storage.NewCloudflare(secrets[secretCloudflareAccount], secrets[secretCloudflareAccess], secrets[secretCloudflareSecret], "")

	// Both BroadcastEvent and MtsMediaV2 live in the media (vidgrind) datastore.
	srv = newServer(mediaStore, provider, jwtSecret, signedURLExpiry)
}
