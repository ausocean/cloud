/*
AUTHORS
  Elliot Shine <elliot@ausocean.org>

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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/notifier"
	"github.com/ausocean/cloud/cmd/oceantv/registry"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/storage"
	"github.com/ausocean/cloud/ytclient"
)

type OceanMedia struct {
	log             func(string, ...any)
	store           datastore.Store
	broadcastCfgID  string
	storageProvider storage.Provider
}

func NewOceanMedia(
	log func(string, ...any),
	store datastore.Store,
	broadcastCfgID string,
	storageProvider storage.Provider,
) *OceanMedia {
	return &OceanMedia{log: log, store: store, broadcastCfgID: broadcastCfgID, storageProvider: storageProvider}
}

func (o OceanMedia) Name() string {
	return "oceanmedia"
}

func (o OceanMedia) New(args ...any) (any, error) {
	// If no arguments are provided, return an empty OceanMedia
	// so that we can still get the protocol.
	if len(args) == 0 {
		return OceanMedia{}, nil
	}
	p, ok := args[0].(Params)
	if !ok {
		return nil, fmt.Errorf("expected broadcasthost.Params")
	}
	if p.StorageProvider == nil {
		return nil, fmt.Errorf("no storage.Provider provided")
	}
	return &OceanMedia{log: p.Log, store: p.Store, broadcastCfgID: p.BroadcastCfgID, storageProvider: *p.StorageProvider}, nil
}

// CreateBroadcast creates an OceanMedia broadcast with the given parameters.
func (o *OceanMedia) CreateBroadcast(
	ctx context.Context,
	broadcastName, description, streamName, privacy, resolution string,
	start, end time.Time,
	opts ...Option,
) (Response, ytclient.IDs, string, error) {
	b := model.BroadcastEvent{
		ScheduledStartTime: start,
		ScheduledEndTime:   end,
		ConfigID:           o.broadcastCfgID,
	}
	bc, err := model.CreateBroadcastEvent(ctx, o.store, b)
	if err != nil {
		return nil, ytclient.IDs{}, "", err
	}
	authKey, err := o.AuthKey(ctx, bc.ID)
	if err != nil {
		return nil, ytclient.IDs{}, "", err
	}
	return nil, ytclient.IDs{BID: bc.ID, SID: "", CID: ""}, authKey, nil
}

// StartBroadcast starts a broadcast with the given parameters.
func (o *OceanMedia) StartBroadcast(
	name, bID, sID string,
	saveLink func(key, link string) error,
	notify func(msg string) error,
) error {
	ctx := context.Background()
	if saveLink != nil {
		if err := saveLink(strings.ReplaceAll(name, " ", ""), "oceanmedia:"+bID); err != nil {
			notifier.LogAndNotify(notify, "broadcast: %s, ID: %s, could not save livestream link: %v", name, bID, err)
		}
	}
	if err := model.SetBroadcastEventStart(ctx, o.store, bID); err != nil {
		return err
	}
	return nil
}

// BroadcastStatus gets the status of the broadcast with the given ID.
func (o *OceanMedia) BroadcastStatus(ctx context.Context, id string) (string, error) {
	b, err := model.GetBroadcastEvent(ctx, o.store, id)
	if err != nil {
		return "", err
	}
	if !b.EndTime.IsZero() {
		return ytclient.StatusComplete, nil
	}
	if !b.StartTime.IsZero() {
		return ytclient.StatusLive, nil
	}
	return ytclient.StatusReady, nil
}

// BroadcastScheduledStartTime gets the scheduled start time of the broadcast
// with the given ID.
func (o *OceanMedia) BroadcastScheduledStartTime(ctx context.Context, id string) (time.Time, error) {
	b, err := model.GetBroadcastEvent(ctx, o.store, id)
	if err != nil {
		return time.Time{}, err
	}
	return b.ScheduledStartTime, nil
}

// BroadcastHealth gets the health of the broadcast with the given ID.
// TODO: implement this.
func (o *OceanMedia) BroadcastHealth(ctx context.Context, sid string) (string, error) {
	return "", nil
}

// AuthKey returns the authentication key for the storage bucket.
// This consists of a JSON encoded TempCredentials object.
// streamName should be the broadcast event ID.
func (o *OceanMedia) AuthKey(ctx context.Context, streamName string) (string, error) {
	tempCreds, err := o.storageProvider.GenerateTempCredentials(ctx, 12*time.Hour, streamName)
	if err != nil {
		return "", err
	}

	jsonBytes, err := json.Marshal(tempCreds)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

func (o *OceanMedia) DestinationURL() string {
	return o.storageProvider.GetBaseURL()
}

func (o OceanMedia) Protocol() string {
	return "oceanmedia"
}

// CompleteBroadcast completes the broadcast with the given ID.
func (o *OceanMedia) CompleteBroadcast(ctx context.Context, id string) error {
	return model.SetBroadcastEventEnd(ctx, o.store, id)
}

// PostChatMessage posts a chat message to the given chat ID.
// This is a no-op as OceanMedia does not have chat functionality.
func (o *OceanMedia) PostChatMessage(cID, msg string) error {
	return nil
}

// SetBroadcastPrivacy sets the privacy of the broadcast with the given ID.
// This is a no-op as OceanMedia streams are always private.
func (o *OceanMedia) SetBroadcastPrivacy(ctx context.Context, id, privacy string) error {
	return nil
}

func init() {
	registry.Register(&OceanMedia{})
}
