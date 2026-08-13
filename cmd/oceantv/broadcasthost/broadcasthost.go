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
	"fmt"
	"net/http"
	"time"

	"github.com/ausocean/cloud/ytclient"
)

// Response is an interface for a server response.
type Response interface {
	fmt.Stringer
	StatusCode() int
	HTTPHeader() http.Header
}

// Option is an option to pass when creating a new broadcast host.
type Option func(BroadcastHost) error

// BroadcastHost is an interface for a broadcast host that the camera will stream to.
// For example, YouTube and OceanMedia.
type BroadcastHost interface {
	CreateBroadcast(
		ctx context.Context,
		broadcastName, description, streamName, privacy, resolution string,
		start, end time.Time,
		opts ...Option,
	) (Response, ytclient.IDs, string, error)

	StartBroadcast(
		name, bID, sID string,
		saveLink func(key, link string) error,
		extStart, extStop func() error,
		notify func(msg string) error,
		onLiveActions func() error,
	) error

	BroadcastStatus(ctx context.Context, id string) (string, error)
	BroadcastScheduledStartTime(ctx context.Context, id string) (time.Time, error)
	BroadcastHealth(ctx context.Context, sid string) (string, error)
	RTMPKey(ctx context.Context, streamName string) (string, error)
	CompleteBroadcast(ctx context.Context, id string) error
	PostChatMessage(cID, msg string) error
	SetBroadcastPrivacy(ctx context.Context, id, privacy string) error
}
