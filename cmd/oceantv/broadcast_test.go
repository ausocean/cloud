/*
DESCRIPTION
  broadcast_test.go houses testing for functionality found in broadcast.go.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2022 the Australian Ocean Lab (AusOcean)

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

package main

import (
	"io"
	"log"
	"testing"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/openfish/datastore"
)

// TestRemoveDate tests the removeDate helper function.
func TestRemoveDate(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "A Broadcast 23/04/15", want: "A Broadcast "},
		{in: "A Broadcast 04/23/15", want: "A Broadcast "},
		{in: "ABroadcast04/23/15", want: "ABroadcast"},
		{in: "ABroadcast04/23/15AStream", want: "ABroadcastAStream"},
	}

	for i, test := range tests {
		got := removeDate(test.in)
		if got != test.want {
			t.Errorf("did not get expected result for test no. %d \ngot: %s \nwant: %s", i, got, test.want)
		}
	}
}

// dummyManager is a dummy implementation of the broadcastManager interface.
type dummyManager struct {
	startDone                                                          chan struct{}
	saved, started, stopped, healthHandled, statusHandled, chatHandled bool
	savedCfgs                                                          map[string]*Cfg
	t                                                                  *testing.T
}

func NewDummyManager(t *testing.T) *dummyManager {
	log.Println("creating dummy manager")
	return &dummyManager{
		t:         t,
		startDone: make(chan struct{}),
	}
}

func (d *dummyManager) CreateBroadcast(
	cfg *Cfg,
	store Store,
	svc BroadcastService,
) error {
	return nil
}

func (d *dummyManager) StartBroadcast(
	ctx Ctx,
	cfg *Cfg,
	store Store,
	svc Svc,
	extStart func() error,
	onSuccess func(),
	onFailure func(error),
) {
	onSuccess()
	// This will only close the channel if it's not closed yet.
	defer func() {
		ok := true
		select {
		case _, ok = <-d.startDone:
		default:
		}
		if ok {
			close(d.startDone)
		}
	}()
	d.started = true
}
func (d *dummyManager) StopBroadcast(ctx Ctx, cfg *Cfg, store Store, svc Svc) error {
	d.stopped = true
	return nil
}
func (d *dummyManager) SaveBroadcast(ctx Ctx, cfg *Cfg, store Store) error {
	d.saved = true
	d.logf("saving broadcast: %s", cfg.Name)
	if d.savedCfgs == nil {
		d.savedCfgs = make(map[string]*Cfg)
	}
	d.savedCfgs[cfg.Name] = cfg
	return nil
}
func (d *dummyManager) HandleStatus(ctx Ctx, cfg *Cfg, store Store, svc Svc, call BroadcastCallback) error {
	d.statusHandled = true
	return nil
}
func (d *dummyManager) HandleChatMessage(ctx Ctx, cfg *Cfg) error {
	d.chatHandled = true
	return nil
}
func (d *dummyManager) HandleHealth(ctx Ctx, cfg *Cfg, goodHealthCallback, badHealthCallback func()) error {
	d.healthHandled = true
	return nil
}
func (d *dummyManager) SetupSecondary(ctx Ctx, cfg *Cfg, store Store) error { return nil }

func (d *dummyManager) logf(format string, args ...interface{}) {
	if d.t == nil {
		return
	}
	d.t.Logf(format, args...)
}

// dummyStore is a dummy implementation of the datastore.Store interface.
// It basically does nothing and is used to test the broadcast functions.
type dummyStore struct{}

func (d *dummyStore) IDKey(kind string, id int64) *Key       { return nil }
func (d *dummyStore) NameKey(kind, name string) *Key         { return nil }
func (d *dummyStore) IncompleteKey(kind string) *Key         { return nil }
func (d *dummyStore) Get(ctx Ctx, key *Key, dst Ety) error   { return nil }
func (d *dummyStore) DeleteMulti(ctx Ctx, keys []*Key) error { return nil }
func (d *dummyStore) NewQuery(kind string, keysOnly bool, keyParts ...string) datastore.Query {
	return nil
}
func (d *dummyStore) GetAll(ctx Ctx, q datastore.Query, dst interface{}) ([]*Key, error) {
	return nil, nil
}
func (d *dummyStore) Put(ctx Ctx, key *Key, src Ety) (*Key, error)          { return nil, nil }
func (d *dummyStore) Create(ctx Ctx, key *Key, src Ety) error               { return nil }
func (d *dummyStore) Update(ctx Ctx, key *Key, fn func(Ety), dst Ety) error { return nil }
func (d *dummyStore) Delete(ctx Ctx, key *Key) error                        { return nil }

// dummyService is a dummy implementation of the BroadcastService interface.
// It does nothing and is used to test the broadcast functions.
type dummyService struct{}

func (d *dummyService) CreateBroadcast(
	ctx Ctx,
	broadcastName, description, streamName, privacy, resolution string,
	start, end time.Time,
	opts ...BroadcastOption,
) (ServerResponse, broadcast.IDs, string, error) {
	return nil, broadcast.IDs{}, "", nil
}

func (d *dummyService) StartBroadcast(
	name, bID, sID string,
	saveLink func(key, link string) error,
	extStart, extStop func() error,
	notify func(msg string) error,
	onLiveActions func() error,
) error {
	return nil
}
func (d *dummyService) BroadcastStatus(ctx Ctx, id string) (string, error) { return "", nil }
func (d *dummyService) RTMPKey(ctx Ctx, streamName string) (string, error) { return "", nil }
func (d *dummyService) CompleteBroadcast(ctx Ctx, id string) error         { return nil }

type dummyForwardingService struct{}

func newDummyForwardingService() *dummyForwardingService                                  { return &dummyForwardingService{} }
func (v *dummyForwardingService) Stream(cfg *Cfg) error                                   { return nil }
func (v *dummyForwardingService) Slate(cfg *Cfg) error                                    { return nil }
func (v *dummyForwardingService) UploadSlate(cfg *Cfg, name string, file io.Reader) error { return nil }

type dummyHardwareManager struct {
	hardwareHealthy bool
	startCalled     bool
	stopCalled      bool
}

func newDummyHardwareManager(healthy bool) *dummyHardwareManager {
	return &dummyHardwareManager{hardwareHealthy: healthy}
}
func (h *dummyHardwareManager) start(ctx *broadcastContext) {
	h.startCalled = true
}
func (h *dummyHardwareManager) stop(ctx *broadcastContext) {
	h.stopCalled = true
}
func (h *dummyHardwareManager) publishEventIfStatus(event event, status bool, mac int64, store Store, log func(format string, args ...interface{}), publish func(event event)) {
	log("status is %v, hardware is healthy %v", status, h.hardwareHealthy)
	if status == true && h.hardwareHealthy {
		publish(event)
	} else if status == false {
		publish(event)
	}
}
