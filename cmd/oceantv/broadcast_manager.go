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
)

// BroadcastManager is an interface for managing broadcasts.
type BroadcastManager interface {
	CreateBroadcast(cfg *Cfg, store Store, svc BroadcastService) error

	StartBroadcast(ctx Ctx, cfg *Cfg, store Store, svc BroadcastService, extStart func() error,
		onSuccess func(),
		onFailure func(error))
	StopBroadcast(ctx Ctx, cfg *Cfg, store Store, svc BroadcastService) error
	SaveBroadcast(ctx Ctx, cfg *Cfg, store Store) error

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
	HandleHealth(ctx Ctx, cfg *Cfg, goodHealthCallback, badHealthCallback func()) error

	SetupSecondary(ctx Ctx, cfg *Cfg, store Store) error
}

// OceanBroadcastManager is an implementation of BroadcastManager with
// a particular focus around ocean broadcasts and AusOcean's infrastructure.
type OceanBroadcastManager struct {
	log func(string, ...interface{})
}

func newOceanBroadcastManager(log func(string, ...interface{})) *OceanBroadcastManager {
	return &OceanBroadcastManager{log}
}

func (m *OceanBroadcastManager) CreateBroadcast(
	cfg *Cfg,
	store Store,
	svc BroadcastService,
) error {
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

	resp, ids, rtmpKey, err := svc.CreateBroadcast(
		context.Background(),
		cfg.Name+" "+dateStr,
		cfg.Description,
		cfg.StreamName,
		cfg.Privacy,
		cfg.Resolution,
		time.Now().Add(1*time.Minute),
		cfg.End,
		WithRateLimiter(limiter),
	)
	if err != nil {
		return fmt.Errorf("could not create broadcast: %v, resp: %v", err, resp)
	}
	err = updateConfigWithTransaction(context.Background(), store, cfg.SKey, cfg.Name, func(_cfg *Cfg) error {
		_cfg.ID = ids.BID
		_cfg.SID = ids.SID
		_cfg.CID = ids.CID
		_cfg.RTMPKey = rtmpKey
		*cfg = *_cfg
		return nil
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
}

// StopBroadcast stops a broadcast using the youtube live streaming API.
// It uses AusOcean methods for saving and stopping external hardware.
func (m *OceanBroadcastManager) StopBroadcast(ctx Ctx, cfg *Cfg, store Store, svc BroadcastService) error {
	return stopBroadcast(ctx, cfg, store, svc, m.log)
}

// SaveBroadcast saves a broadcast to the datastore.
// It uses AusOcean methods for saving, and updating the vidforward service if
// configuration if in use.
func (m *OceanBroadcastManager) SaveBroadcast(ctx Ctx, cfg *Cfg, store Store) error {
	return saveBroadcast(ctx, cfg, store, m.log)
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

// HandleChatMessage generates chat messages containing sensor info such as
// water temperature and uses the youtube API to post the message. The
// sensors used are those specified in the configuration.
func (m *OceanBroadcastManager) HandleChatMessage(ctx Ctx, cfg *Cfg) error {
	return handleChatMessage(ctx, cfg, m.log)
}

// HandleHealth interprets the health of a broadcast and calls the provided callbacks in response to the health.
// For tolerance to temporary issues, we only call the badHealthCallback if the health is bad for more than 4 checks.
func (m *OceanBroadcastManager) HandleHealth(ctx Ctx, cfg *Cfg, goodHealthCallback, badHealthCallback func()) error {
	m.log("handling health check")
	hasIssue, err := checkIssues(ctx, cfg, m.log)
	if err != nil {
		return fmt.Errorf("could not check for stream issues: %w", err)
	}

	if !hasIssue {
		cfg.Issues = 0
		goodHealthCallback()
		return nil
	}
	cfg.Issues++

	const maxHealthIssues = 4
	if cfg.Issues > maxHealthIssues {
		badHealthCallback()
		cfg.Issues = 0
	}

	return nil
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

	populateFields := func(_cfg *BroadcastConfig) error {
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
		return nil
	}

	_, err = broadcastByName(cfg.SKey, secondaryName)
	switch {
	// Broadcast not found, so we need to create it.
	case errors.Is(err, ErrBroadcastNotFound{}):
		secondaryCfg := *cfg
		err = populateFields(&secondaryCfg)
		if err != nil {
			return fmt.Errorf("could not populate secondary broadcast fields: %w", err)
		}
		err = saveBroadcast(ctx, &secondaryCfg, store, m.log)
		if err != nil {
			return fmt.Errorf("could not save secondary broadcast: %w", err)
		}
	case err != nil:
		return fmt.Errorf("could not check if secondary broadcast exists: %w", err)

	// Broadcast found so we need to update it with a transaction.
	default:
		err = updateConfigWithTransaction(
			context.Background(),
			store,
			cfg.SKey,
			secondaryName,
			populateFields,
		)
		if err != nil {
			return fmt.Errorf("could not update secondary broadcast: %w", err)
		}
	}

	return nil
}
