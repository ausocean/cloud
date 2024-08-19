/*
DESCRIPTION
  broadcast_manager.go provides the BroadcastManager interface and
  implementations i.e. OceanBroadcastManager.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2023 the Australian Ocean Lab (AusOcean)

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
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
	"github.com/ausocean/utils/nmea"
)

// BroadcastManager is an interface for managing broadcasts.
type BroadcastManager interface {
	CreateBroadcast(cfg *Cfg, store Store, svc BroadcastService) error

	StartBroadcast(ctx Ctx, cfg *Cfg, store Store, svc BroadcastService, extStart func() error,
		onSuccess func(),
		onFailure func(error))
	StopBroadcast(ctx Ctx, cfg *Cfg, store Store, svc BroadcastService) error
	Save(ctx Ctx, update func(*BroadcastConfig)) error

	// HandleStatus checks the status of a broadcast and would perform any
	// necessary actions based on this status. For example, if the broadcast
	// status is complete or revoked, it might stop the broadcast.
	HandleStatus(ctx Ctx, cfg *Cfg, store Store, svc BroadcastService, noBroadcastCallBack BroadcastCallback) error

	// HandleChatMessage prepares and sends chat messages to the broadcast
	// service's chat session. This might contain information such as
	// auxillary sensor data.
	HandleChatMessage(ctx Ctx, cfg *Cfg) error

	// HandleHealth interprets the health of a broadcast and would perform any
	// necessary actions based on this health. For example, if the health is
	// bad, it might restart the broadcast.
	HandleHealth(ctx Ctx, cfg *Cfg, store Store, goodHealthCallback func(), badHealthCallback func(string)) error

	SetupSecondary(ctx Ctx, cfg *Cfg, store Store) error
}

// OceanBroadcastManager is an implementation of BroadcastManager with
// a particular focus around ocean broadcasts and AusOcean's infrastructure.
type OceanBroadcastManager struct {
	svc   BroadcastService
	log   func(string, ...interface{})
	cfg   *Cfg
	store Store
}

// newOceanBroadcastManager creates a new OceanBroadcastManager.
// svc may be nil, but any methods that require it will panic.
func newOceanBroadcastManager(svc BroadcastService, cfg *Cfg, store Store, log func(string, ...interface{})) *OceanBroadcastManager {
	return &OceanBroadcastManager{svc: svc, cfg: cfg, store: store, log: log}
}

func (m *OceanBroadcastManager) CreateBroadcast(
	cfg *Cfg,
	store Store,
	svc BroadcastService,
) error {
	// Only create a new broadcast if a valid one doesn't already exist.
	if m.broadcastCanBeReused(cfg, svc) {
		m.log("broadcast already exists with broadcastID: %s, streamID: %s", cfg.ID, cfg.SID)
		err := m.Save(nil, func(_cfg *Cfg) { _cfg.ID = cfg.ID; _cfg.SID = cfg.SID; _cfg.CID = cfg.CID; _cfg.RTMPKey = cfg.RTMPKey })
		if err != nil {
			return fmt.Errorf("could not save broadcast config: %w", err)
		}
		return nil
	}

	// We're going to add the date to the broadcast's name, so get this and format.
	loc, err := time.LoadLocation(locationID)
	if err != nil {
		return fmt.Errorf("could not load location: %w", err)
	}

	const layout = "02/01/2006"
	dateStr := time.Now().In(loc).Format(layout)

	const (
		// This allows for 10 broadcasts to be created with 3 retries each
		// all being started within the same hour.
		limiterMaxTokens  = 30.0
		limiterRefillRate = 2.0 // per hour
		limiterID         = "ocean_token_bucket"
	)
	limiter, err := GetOceanTokenBucketLimiter(limiterMaxTokens, limiterRefillRate, limiterID, store)
	if err != nil {
		return fmt.Errorf("could not get token bucket limiter: %w", err)
	}

	timeCreated := time.Now().Add(1 * time.Minute)
	resp, ids, rtmpKey, err := svc.CreateBroadcast(
		context.Background(),
		cfg.Name+" "+dateStr,
		cfg.Description,
		cfg.StreamName,
		cfg.Privacy,
		cfg.Resolution,
		timeCreated,
		cfg.End,
		WithRateLimiter(limiter),
	)
	if err != nil {
		return fmt.Errorf("could not create broadcast: %w, resp: %v", err, resp)
	}
	err = m.Save(nil, func(_cfg *Cfg) {
		_cfg.ID = ids.BID
		_cfg.SID = ids.SID
		_cfg.CID = ids.CID
		_cfg.RTMPKey = rtmpKey
	})
	if err != nil {
		return fmt.Errorf("could not update config with transaction: %w", err)
	}
	return nil
}

// StartBroadcast starts a broadcast using the youtube live streaming API.
// It provides AusOcean specific callbacks for starting and stopping
// external hardware (cameras) and services.
func (m *OceanBroadcastManager) StartBroadcast(
	ctx Ctx,
	cfg *Cfg,
	store Store,
	svc BroadcastService,
	extStart func() error,
	onSuccess func(),
	onFailure func(error),
) {
	if extStart != nil {
		err := extStart()
		if err != nil {
			onFailure(fmt.Errorf("could not start external hardware: %w", err))
			return
		}
	}

	go func() {
		err := svc.StartBroadcast(
			cfg.Name,
			cfg.ID,
			cfg.SID,
			saveLinkFunc(),
			func() error { return nil }, // This is now handled by the hardware state machine.
			func() error { return nil }, // This is now handled by the hardware state machine.
			opsHealthNotifyFunc(ctx, cfg),
			func() error { return nil }) // This is now handled by the hardware state machine.
		if err != nil {
			onFailure(fmt.Errorf("could not start broadcast: %w", err))
			return
		}
		onSuccess()
	}()
}

// StopBroadcast stops a broadcast using the youtube live streaming API.
// It uses AusOcean methods for saving and stopping external hardware.
func (m *OceanBroadcastManager) StopBroadcast(ctx Ctx, cfg *Cfg, store Store, svc BroadcastService) error {
	return stopBroadcast(ctx, cfg, store, svc, m.log)
}

// Save performs broadcast configuration update operations.
// If ctx is nil, the background context will be used.
//
// update allows for the update of specific fields. After this update takes
// place, the config we currently point at will be updated with any changes
// that we have applied, and with anything from the store before the update.
// If this is nil, the config currently in store will be replaced.
func (m *OceanBroadcastManager) Save(ctx Ctx, update func(_cfg *Cfg)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	_update := func(_cfg *BroadcastConfig) { *_cfg = *m.cfg }
	if update != nil {
		_update = func(_cfg *Cfg) {
			update(_cfg)
			*m.cfg = *_cfg
		}
	}
	return updateConfigWithTransaction(ctx, m.store, m.cfg.SKey, m.cfg.Name, _update)
}

// HandleStatus checks the status of a broadcast and stops it if it has
// complete or revoked status.
func (m *OceanBroadcastManager) HandleStatus(ctx Ctx, cfg *Cfg, store Store, svc BroadcastService, noBroadcastCallBack BroadcastCallback) error {
	m.log("handling status check")
	status, err := svc.BroadcastStatus(ctx, cfg.ID)
	if err != nil {
		if !errors.Is(err, broadcast.ErrNoBroadcastItems) {
			return fmt.Errorf("could not get broadcast status: %w", err)
		}

		m.log("no broadcast items with this configuration listed")
		err := noBroadcastCallBack(ctx, cfg, store, svc)
		if err != nil {
			return fmt.Errorf("could not call no broadcast callback: %w", err)
		}
	}

	if status != broadcast.StatusComplete && status != broadcast.StatusRevoked {
		return nil
	}

	m.log("status is complete or revoked")
	err = noBroadcastCallBack(ctx, cfg, store, svc)
	if err != nil {
		return fmt.Errorf("could not call no broadcast callback: %w", err)
	}

	return nil
}

// HandleChatMessage generates a message with sensor readings for the
// relevant site and posts the message to the broadcast chat. This works by
// searching the site for any registered ESP devices and looking at the latest
// signal values on sensors which have been marked true to send a message.
func (m *OceanBroadcastManager) HandleChatMessage(ctx Ctx, cfg *Cfg) error {
	if !cfg.SendMsg {
		m.log("ignoring sensors")
		return nil
	}

	m.log("building message")
	var msg string

	for _, sensor := range cfg.SensorList {
		if !sensor.SendMsg {
			continue
		}
		// Get the latest signal for the sensor.
		var qty string

		scalar, err := getLatestScalar(ctx, mediaStore, model.ToSID(model.MacDecode(sensor.DeviceMac), sensor.Sensor.Pin))
		if err == datastore.ErrNoSuchEntity {
			continue
		} else if err != nil {
			return fmt.Errorf("could not get scalar for chat message: %v", err)
		}

		value, err := sensor.Sensor.Transform(scalar.Value)
		if err != nil {
			return fmt.Errorf("could not transform scalar: %v", err)
		}

		for _, q := range nmea.DefaultQuantities() {
			if q.Code == nmea.Code(sensor.Sensor.Quantity) {
				qty = q.Name
			}
		}

		// Add the latest sensor value to the message.
		var line string
		if msg == "" {
			line = fmt.Sprintf("%s: %3.1f %s ", qty, value, sensor.Sensor.Units)
		} else {
			line = fmt.Sprintf("| %s: %3.1f %s ", qty, value, sensor.Sensor.Units)
		}
		msg += line
	}

	if msg == "" {
		m.log("chat message empty")
		return nil
	}

	err := m.svc.PostChatMessage(cfg.CID, msg)
	if err != nil {
		return fmt.Errorf("broadcast chat message post error: %w", err)
	}
	return nil
}

// HandleHealth interprets the health of a broadcast and calls the provided callbacks in response to the health.
// For tolerance to temporary issues, we only call the badHealthCallback if the health is bad for more than 4 checks.
func (m *OceanBroadcastManager) HandleHealth(ctx Ctx, cfg *Cfg, store Store, goodHealthCallback func(), badHealthCallback func(string)) error {
	m.log("handling health check")
	issue, err := m.svc.BroadcastHealth(ctx, cfg.SID)
	if err != nil {
		return fmt.Errorf("could not check for stream issues: %w", err)
	}

	if issue == "" {
		cfg.Issues = 0
		goodHealthCallback()
		return nil
	}
	m.log("issue found: %s", issue)

	cfg.Issues++
	const maxHealthIssues = 4
	if cfg.Issues > maxHealthIssues {
		badHealthCallback(issue)
		cfg.Issues = 0
	}

	return m.Save(nil, func(_cfg *Cfg) { _cfg.Issues = cfg.Issues; *cfg = *_cfg })
}

func (m *OceanBroadcastManager) SetupSecondary(ctx Ctx, cfg *Cfg, store Store) error {
	m.log("setting up vidforward broadcasting for %v", cfg.Name)

	// Sanity check. This should only be invoked for the primary broadcast only so make sure
	// the name does not contain the secondary broadcast postfix.
	if strings.Contains(cfg.Name, secondaryBroadcastPostfix) {
		panic("setupVidforwardBroadcasting should only be invoked for the primary broadcast")
	}

	// Let's first set up the device.
	// Set the HTTPAddress variable to send to the vidforward service.
	// Set the Outputs variable to HTTP so that we're using MPEG-TS over HTTP.
	mac := fmt.Sprintf("%012x", cfg.CameraMac)
	err := setVar(ctx, store, mac+"."+config.KeyHTTPAddress, cfg.VidforwardHost, cfg.SKey, m.log)
	if err != nil {
		return fmt.Errorf("could not set the HTTPAddress variable for the camera: %w", err)
	}
	err = setVar(ctx, store, mac+"."+config.KeyOutputs, "HTTP", cfg.SKey, m.log)
	if err != nil {
		return fmt.Errorf("could not set the camera output to http: %w", err)
	}
	// Check if secondary broadcast already exists.
	secondaryName := cfg.Name + secondaryBroadcastPostfix

	populateFields := func(_cfg *BroadcastConfig) {
		// The secondary broadcast will for the most part copy the long term broadcast
		// configuration, except for a few of the fields.
		_cfg.Name = secondaryName
		_cfg.StreamName = secondaryName
		_cfg.Privacy = "unlisted" // We don't want the secondary broadcast to be easily discovered by youtube watchers.
		_cfg.OnActions = ""       // We don't need it to have any control of the camera hardware.
		_cfg.OffActions = ""      // Ditto.
		_cfg.SendMsg = true       // It would be handy to have sensors stored in the store broadcasts too.
		_cfg.Start = cfg.Start
		_cfg.End = cfg.End
		_cfg.Resolution = cfg.Resolution
		_cfg.Enabled = true
	}

	_, err = broadcastByName(cfg.SKey, secondaryName)
	switch {
	// Broadcast not found, so we need to create it.
	case errors.Is(err, ErrBroadcastNotFound{}):
		secondaryCfg := *cfg
		populateFields(&secondaryCfg)
		err = saveBroadcast(ctx, &secondaryCfg, store, m.log)
		if err != nil {
			return fmt.Errorf("could not save secondary broadcast: %w", err)
		}
	case err != nil:
		return fmt.Errorf("could not check if secondary broadcast exists: %w", err)

	// Broadcast found so we need to update it with a transaction.
	default:
		err = m.Save(nil, populateFields)
		if err != nil {
			return fmt.Errorf("could not update secondary broadcast: %w", err)
		}
	}

	return nil
}

// opsHealthNotifyFunc returns a closure of notifier.Send given to the
// broadcast.BroadcastStream function for notifications.
func opsHealthNotifyFunc(ctx context.Context, cfg *BroadcastConfig) func(string) error {
	return func(msg string) error {
		return notifier.Send(ctx, cfg.SKey, broadcastGeneric, msg)
	}
}

// broadcastCanBeReused checks if a broadcast can be reused based on how old it
// is, if it has been revoked or completed, and if its IDs have been set.
func (m *OceanBroadcastManager) broadcastCanBeReused(cfg *BroadcastConfig, svc BroadcastService) bool {
	startTime, err := svc.BroadcastScheduledStartTime(context.Background(), cfg.ID)

	// Check if the broadcast was created today. Don't reuse an old broadcast.
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if startTime.Before(startOfToday) || startTime.IsZero() {
		m.log("broadcast does not exist for today, last start time: %v", startTime)
		return false
	}
	status, err := svc.BroadcastStatus(context.Background(), cfg.ID)
	if err != nil {
		m.log("could not get today's broadcast status: %v", err)
		return false
	}
	m.log("today's broadcast has status: %s", status)
	return cfg.ID != "" && cfg.SID != "" && status != "" && status != broadcast.StatusRevoked && status != broadcast.StatusComplete
}
