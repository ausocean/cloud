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

package manager

import (
	"context"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/yt"
	"github.com/ausocean/cloud/datastore"
)

type BroadcastCallback func(context.Context, *broadcast.Config, datastore.Store, yt.BroadcastService) error

// Broadcast is an interface for managing broadcasts.
type Broadcast interface {
	CreateBroadcast(cfg *broadcast.Config, store datastore.Store, svc yt.BroadcastService) error

	StartBroadcast(ctx context.Context, cfg *broadcast.Config, store datastore.Store, svc yt.BroadcastService, extStart func() error,
		onSuccess func(),
		onFailure func(error))
	StopBroadcast(ctx context.Context, cfg *broadcast.Config, store datastore.Store, svc yt.BroadcastService) error
	Save(ctx context.Context, update func(*broadcast.Config)) error

	// HandleStatus checks the status of a broadcast and would perform any
	// necessary actions based on this status. For example, if the broadcast
	// status is complete or revoked, it might stop the broadcast.
	HandleStatus(ctx context.Context, cfg *broadcast.Config, store datastore.Store, svc yt.BroadcastService, noBroadcastCallBack BroadcastCallback) error

	// HandleChatMessage prepares and sends chat messages to the broadcast
	// service's chat session. This might contain information such as
	// auxillary sensor data.
	HandleChatMessage(ctx context.Context, cfg *broadcast.Config) error

	// HandleHealth interprets the health of a broadcast and would perform any
	// necessary actions based on this health. For example, if the health is
	// bad, it might restart the broadcast.
	HandleHealth(ctx context.Context, cfg *broadcast.Config, store datastore.Store, goodHealthCallback func(), badHealthCallback func(string)) error

	SetupSecondary(ctx context.Context, cfg *broadcast.Config, store datastore.Store) error
}
