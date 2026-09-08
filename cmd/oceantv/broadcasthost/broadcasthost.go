/*
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package broadcasthost

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/registry"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/storage"
)

// Broadcast statuses.
const (
	StatusComplete = "complete"
	StatusRevoked  = "revoked"
	StatusTesting  = "testing"
	StatusLive     = "live"
	StatusReady    = "ready"
)

// ErrNoBroadcastItems is returned when a broadcast host has no broadcast
// matching the provided ID.
var ErrNoBroadcastItems = errors.New("no broadcast items")

// IDs contains Broadcast ID, Stream ID and Chat ID.
type IDs struct {
	BID, SID, CID string
}

// Response is an interface for a server response.
type Response interface {
	fmt.Stringer
	StatusCode() int
	HTTPHeader() http.Header
}

// Option is an option to pass when creating a new broadcast host.
type Option func(Host) error

// Host is an interface for a broadcast host that the camera will stream to.
// For example, YouTube and OceanMedia.
type Host interface {
	registry.Named
	registry.Newable
	CreateBroadcast(
		ctx context.Context,
		broadcastName, description, streamName, privacy, resolution string,
		start, end time.Time,
		opts ...Option,
	) (Response, IDs, string, error)

	StartBroadcast(
		name, bID, sID string,
		saveLink func(key, link string) error,
		notify func(msg string) error,
	) error

	BroadcastStatus(ctx context.Context, id string) (string, error)
	BroadcastScheduledStartTime(ctx context.Context, id string) (time.Time, error)
	BroadcastHealth(ctx context.Context, sid string) (string, error)
	AuthKey(ctx context.Context, streamName string) (string, error)
	DestinationURL() string
	Protocol() string
	CompleteBroadcast(ctx context.Context, id string) error
	PostChatMessage(cID, msg string) error
	SetBroadcastPrivacy(ctx context.Context, id, privacy string) error
}

// Params are the parameters that are passed to the broadcast host when initializing it via the registry.
// This is needed so that we can use the registry to initialize the broadcast host without
// using specific parameters for each host.
type Params struct {
	Log             func(string, ...interface{})
	Store           datastore.Store
	BroadcastCfgID  string
	StorageProvider *storage.Provider
	TokenURI        string
}
