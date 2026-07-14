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
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	rv_config "github.com/ausocean/av/revid/config"
	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/notifier"
	"github.com/ausocean/cloud/cmd/oceantv/ratelimit"
	"github.com/ausocean/cloud/cmd/oceantv/yt"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/utils/nmea"
	"github.com/google/uuid"
)

const locationID = "Australia/Adelaide" // TODO: Use site location (remove duplicate).

// OceanBroadcast is an implementation of BroadcastManager with
// a particular focus around ocean broadcasts and AusOcean's infrastructure.
type OceanBroadcast struct {
	svc   yt.BroadcastService
	log   func(string, ...interface{})
	cfg   *broadcast.Config
	store datastore.Store

	// TODO: remove these once setVar and broadcastByName can be imported.
	setVar          func(ctx context.Context, store datastore.Store, name, value string, sKey int64, log func(string, ...interface{})) error
	broadcastByName func(sKey int64, name string) (*broadcast.Config, error)
}

// NewOceanBroadcast creates a new OceanBroadcastManager.
// svc may be nil, but any methods that require it will panic.
func NewOceanBroadcast(
	svc yt.BroadcastService,
	cfg *broadcast.Config,
	store datastore.Store,
	log func(string, ...interface{}),
	setVar func(
		ctx context.Context,
		store datastore.Store,
		name, value string,
		sKey int64,
		log func(string, ...interface{}),
	) error,
	broadcastByName func(sKey int64, name string) (*broadcast.Config, error),
) *OceanBroadcast {
	return &OceanBroadcast{svc: svc, cfg: cfg, store: store, log: log}
}

func (m *OceanBroadcast) CreateBroadcast(
	cfg *broadcast.Config,
	store datastore.Store,
	svc yt.BroadcastService,
) error {
	// Only create a new broadcast if a valid one doesn't already exist.
	if m.BroadcastCanBeReused() {
		m.log("broadcast already exists with broadcastID: %s, streamID: %s", cfg.BID, cfg.SID)
		err := m.Save(nil, func(_cfg *broadcast.Config) {
			_cfg.BID = cfg.BID
			_cfg.SID = cfg.SID
			_cfg.CID = cfg.CID
			_cfg.RTMPKey = cfg.RTMPKey
		})
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
	limiter, err := ratelimit.GetOceanTokenBucketLimiter(limiterMaxTokens, limiterRefillRate, limiterID, store)
	if err != nil {
		return fmt.Errorf("could not get token bucket limiter: %w", err)
	}

	timeCreated := time.Now().Add(1 * time.Minute)
	resp, ids, rtmpKey, err := svc.CreateBroadcast(
		context.Background(),
		cfg.Name+" "+dateStr,
		cfg.Description,
		cfg.StreamName,
		cfg.LivePrivacy,
		cfg.Resolution,
		timeCreated,
		cfg.End,
		yt.WithRateLimiter(limiter),
	)
	if err != nil {
		return fmt.Errorf("could not create broadcast: %w, resp: %v", err, resp)
	}
	err = m.Save(nil, func(_cfg *broadcast.Config) {
		_cfg.BID = ids.BID
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
func (m *OceanBroadcast) StartBroadcast(
	ctx context.Context,
	cfg *broadcast.Config,
	store datastore.Store,
	svc yt.BroadcastService,
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
			cfg.BID,
			cfg.SID,
			saveLinkFunc(store),
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
func (m *OceanBroadcast) StopBroadcast(ctx context.Context, cfg *broadcast.Config, store datastore.Store, svc yt.BroadcastService) error {
	m.log("stopping broadcast")

	status, err := svc.BroadcastStatus(ctx, cfg.BID)
	if err != nil {
		return fmt.Errorf("could not get broadcast status: %w", err)
	}

	if status != yt.StatusComplete && status != "" {
		err := svc.CompleteBroadcast(ctx, cfg.BID)
		if err != nil {
			return fmt.Errorf("could not complete broadcast: %w", err)
		}
	}

	err = m.Save(ctx, func(_cfg *broadcast.Config) { _cfg.Active = false })
	if err != nil {
		return fmt.Errorf("could not save broadcast config, to update Active state: %w", err)
	}

	// Change privacy to post live privacy.
	// This will also set the privacy of the video after the broadcast has ended.
	err = svc.SetBroadcastPrivacy(ctx, cfg.BID, cfg.PostLivePrivacy)
	if err != nil {
		return fmt.Errorf("could not update broadcast privacy: %w", err)
	}

	return nil
}

// Save performs broadcast configuration update operations.
// If ctx is nil, the background context will be used.
//
// update allows for the update of specific fields. After this update takes
// place, the config we currently point at will be updated with any changes
// that we have applied, and with anything from the store before the update.
// If this is nil, the config currently in store will be replaced.
func (m *OceanBroadcast) Save(ctx context.Context, update func(_cfg *broadcast.Config)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	_update := func(_cfg *broadcast.Config) { *_cfg = *m.cfg }
	if update != nil {
		_update = func(_cfg *broadcast.Config) {
			update(_cfg)
			*m.cfg = *_cfg
		}
	}

	// Reference by UUID if we have one.
	if uuid.Validate(m.cfg.UUID) == nil {
		return broadcast.UpdateConfigWithTransaction(ctx, m.store, m.cfg.SKey, m.cfg.UUID, _update)
	}

	// If we don't have a UUID, we will follow the following steps:
	//   1. Add a UUID to the config
	//   2. Run the transaction to update the config
	//   3. Copy the config, and index it by the UUID
	//   4. Delete the old config
	origUpdate := _update

	_update = func(_cfg *broadcast.Config) {
		_cfg.UUID = uuid.NewString()
		origUpdate(_cfg)
	}

	err := broadcast.UpdateConfigWithTransaction(ctx, m.store, m.cfg.SKey, m.cfg.Name, _update)
	if err != nil {
		return fmt.Errorf("unable to update config: %w", err)
	}
	oldName := broadcast.Scope + "." + m.cfg.Name
	v, err := model.GetVariable(ctx, m.store, m.cfg.SKey, oldName)
	if err != nil {
		return fmt.Errorf("unable to get variable for updated config: %w", err)
	}

	newName := broadcast.Scope + "." + m.cfg.UUID
	err = model.PutVariable(ctx, m.store, m.cfg.SKey, newName, v.Value)
	if err != nil {
		return fmt.Errorf("unable to put config indexed with UUID: %w", err)
	}

	err = model.DeleteVariable(ctx, m.store, m.cfg.SKey, oldName)
	if err != nil {
		return fmt.Errorf("unable to delete config indexed by name: %w", err)
	}

	return nil
}

// HandleStatus checks the status of a broadcast and stops it if it has
// complete or revoked status.
func (m *OceanBroadcast) HandleStatus(ctx context.Context, cfg *broadcast.Config, store datastore.Store, svc yt.BroadcastService, noBroadcastCallBack BroadcastCallback) error {
	m.log("handling status check")
	status, err := svc.BroadcastStatus(ctx, cfg.BID)
	if err != nil {
		if !errors.Is(err, yt.ErrNoBroadcastItems) {
			return fmt.Errorf("could not get broadcast status: %w", err)
		}

		m.log("no broadcast items with this configuration listed")
		err := noBroadcastCallBack(ctx, cfg, store, svc)
		if err != nil {
			return fmt.Errorf("could not call no broadcast callback: %w", err)
		}
	}

	if status != yt.StatusComplete && status != yt.StatusRevoked {
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
func (m *OceanBroadcast) HandleChatMessage(ctx context.Context, cfg *broadcast.Config) error {
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

		scalar, err := model.GetLatestScalar(ctx, m.store, model.ToSID(model.MacDecode(sensor.DeviceMac), sensor.Sensor.Pin))
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
func (m *OceanBroadcast) HandleHealth(ctx context.Context, cfg *broadcast.Config, store datastore.Store, goodHealthCallback func(), badHealthCallback func(string)) error {
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

	return m.Save(nil, func(_cfg *broadcast.Config) { _cfg.Issues = cfg.Issues; *cfg = *_cfg })
}

func (m *OceanBroadcast) SetupSecondary(ctx context.Context, cfg *broadcast.Config, store datastore.Store) error {
	m.log("setting up vidforward broadcasting for %v", cfg.Name)

	// Sanity check. This should only be invoked for the primary broadcast only so make sure
	// the name does not contain the secondary broadcast postfix.
	if strings.Contains(cfg.Name, broadcast.SecondaryPostfix) {
		panic("setupVidforwardBroadcasting should only be invoked for the primary broadcast")
	}

	// Let's first set up the device.
	// Set the HTTPAddress variable to send to the vidforward service.
	// Set the Outputs variable to HTTP so that we're using MPEG-TS over HTTP.
	mac := fmt.Sprintf("%012x", cfg.CameraMac)
	err := m.setVar(ctx, store, mac+"."+rv_config.KeyHTTPAddress, cfg.VidforwardHost, cfg.SKey, m.log)
	if err != nil {
		return fmt.Errorf("could not set the HTTPAddress variable for the camera: %w", err)
	}
	err = m.setVar(ctx, store, mac+"."+rv_config.KeyOutputs, "HTTP", cfg.SKey, m.log)
	if err != nil {
		return fmt.Errorf("could not set the camera output to http: %w", err)
	}
	// Check if secondary broadcast already exists.
	secondaryName := cfg.Name + broadcast.SecondaryPostfix

	populateFields := func(_cfg *broadcast.Config) {
		// The secondary broadcast will for the most part copy the long term broadcast
		// configuration, except for a few of the fields.
		_cfg.Name = secondaryName
		_cfg.StreamName = secondaryName
		_cfg.LivePrivacy = "unlisted"     // We don't want the secondary broadcast to be easily discovered by youtube watchers.
		_cfg.PostLivePrivacy = "unlisted" // This will be public eventually, but not while the software is young.
		_cfg.OnActions = ""               // We don't need it to have any control of the camera hardware.
		_cfg.OffActions = ""              // Ditto.
		_cfg.SendMsg = true               // It would be handy to have sensors stored in the store broadcasts too.
		_cfg.Start = cfg.Start
		_cfg.End = cfg.End
		_cfg.Resolution = cfg.Resolution
		_cfg.Enabled = true
	}

	secondary, err := m.broadcastByName(cfg.SKey, secondaryName)
	switch {
	// Broadcast not found, so we need to create it.
	case errors.Is(err, broadcast.ErrBroadcastNotFound{}):
		secondaryCfg := *cfg
		populateFields(&secondaryCfg)

		// Create a temporary OceanBroadcastManager for the secondary broadcast and create it (no update func required).
		err = NewOceanBroadcast(nil, &secondaryCfg, store, m.log, m.setVar, m.broadcastByName).Save(ctx, nil)
		if err != nil {
			return fmt.Errorf("could not save secondary broadcast: %w", err)
		}
	case err != nil:
		return fmt.Errorf("could not check if secondary broadcast exists: %w", err)

	// Broadcast found so we need to update it with a transaction.
	default:
		// Create a temporary OceanBroadcastManager for the secondary broadcast and update it.
		err = NewOceanBroadcast(nil, secondary, store, m.log, m.setVar, m.broadcastByName).Save(ctx, populateFields)
		if err != nil {
			return fmt.Errorf("could not update secondary broadcast: %w", err)
		}
	}

	return nil
}

// opsHealthNotifyFunc returns a closure of notifier.Send given to the
// broadcast.BroadcastStream function for notifiers.
func opsHealthNotifyFunc(ctx context.Context, cfg *broadcast.Config) func(string) error {
	return func(msg string) error {
		return notifier.N.Send(ctx, cfg.SKey, notifier.KindGeneric, msg)
	}
}

// BroadcastCanBeReused checks if a broadcast can be reused based on how old it
// is, if it has been revoked or completed, and if its IDs have been set.
func (m *OceanBroadcast) BroadcastCanBeReused() bool {
	// Check if the broadcast was created today. Don't reuse an old broadcast.
	startTime, err := m.svc.BroadcastScheduledStartTime(context.Background(), m.cfg.BID)
	if err != nil {
		m.log("could not get today's broadcast start time: %v", err)
		return false
	}
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if startTime.Before(startOfToday) || startTime.IsZero() {
		m.log("broadcast does not exist for today, last start time: %v", startTime)
		return false
	}

	status, err := m.svc.BroadcastStatus(context.Background(), m.cfg.BID)
	if err != nil {
		m.log("could not get today's broadcast status: %v", err)
		return false
	}
	m.log("today's broadcast has status: %s", status)
	return m.cfg.BID != "" && m.cfg.SID != "" && status != "" && status != yt.StatusRevoked && status != yt.StatusComplete
}

// saveLinkFunc provides a closure for saving a broadcast link with a given key.
func saveLinkFunc(store datastore.Store) func(string, string) error {
	return func(key, link string) error {
		key = removeDate(key)
		return model.PutVariable(context.Background(), store, -1, broadcast.LiveScope+"."+key, link)
	}
}

// removeDate removes a date from within a string that matches dd/mm/yyyy or mm/dd/yyyy.
func removeDate(s string) string {
	const dateRegex = "[0-3][0-9]/[0-3][0-9]/(?:[0-9][0-9])?[0-9][0-9]"
	r := regexp.MustCompile(dateRegex)
	return r.ReplaceAllString(s, "")
}
