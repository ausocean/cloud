/*
DESCRIPTION
  broadcast.go provides youtube broadcast scheduling request handling.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Russell Stanley <russell@ausocean.org>

LICENSE
  Copyright (C) 2021 the Australian Ocean Lab (AusOcean)

  This file is part of VidGrind. VidGrind is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  VidGrind is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with NetReceiver in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/ausocean/iotsvc/gauth"
	"bitbucket.org/ausocean/iotsvc/iotds"
	"bitbucket.org/ausocean/vidgrind/broadcast"
)

type Action int

type (
	Cfg   = BroadcastConfig
	Ctx   = context.Context
	Store = iotds.Store
	Key   = iotds.Key
	Ety   = iotds.Entity
	Svc   = BroadcastService
)

const (
	none Action = iota

	// Actions related to vidgrind broadcast control.
	broadcastStart
	broadcastStop
	broadcastSave
	broadcastToken
	broadcastDelete
	broadcastSelect

	// Vidforward control API request actions.
	vidforwardCreate
	vidforwardPlay
	vidforwardSlate
	vidforwardDelete
	vidforwardSlateUpdate
)

// Datastore broadcast and live scopes.
const (
	broadcastScope            = "Broadcast"                           // Scope under which broadcast configs are stored.
	liveScope                 = "Live"                                // Scope under which live stream URLs are stored.
	defaultMessage            = "Welcome to the AusOcean livestream!" // Default message to be sent to the YouTube live chat.
	tempPin                   = "X60"                                 // Standard temperature pin value.
	scalar                    = 0.1                                   // Scalar for temperature conversions from int to float.
	absZero                   = -273.15                               // Offset for temperature conversions from int to float.
	rtmpDestinationAddress    = "rtmp://a.rtmp.youtube.com/live2/"    // Base address for RTMP destination (RTMP key is appended).
	secondaryBroadcastPostfix = "(Secondary)"                         // Post fix used on end of secondary broadcast names.
	longTermBroadcastDuration = 1                                     // The duration of the long term broadcast in years.
)

// broadcastRequest is used by the broadcastHandler to hold broadcast information.
type broadcastRequest struct {
	BroadcastVars      []iotds.Variable // Holds prior saved broadcast configs.
	CurrentBroadcast   BroadcastConfig  // Holds configuration data for broadcast config in form.
	Cameras            []Camera         // Slice of all the cameras on the site.
	Action             string           // Holds value of any button pressed.
	ListingSecondaries bool             // Are we listing secondary broadcasts?
	commonData
}

// BroadcastConfig holds configuration data for a YouTube broadcast.
type BroadcastConfig struct {
	SKey              int64         // The key of the site this broadcast belongs to.
	Name              string        // The name of the broadcast.
	ID                string        // Broadcast identification.
	SID               string        // Stream ID for any currently associated stream.
	CID               string        // ID of associated chat.
	StreamName        string        // The name of the stream we'll bind to the broadcast.
	Description       string        // The broadcast description shown below viewing window.
	Privacy           string        // Privacy of the broadcast i.e. public, private or unlisted.
	Resolution        string        // Resolution of the stream e.g. 1080p.
	StartTime         string        // Start time of the broadcast in yy/mm/dd, hh:mm format.
	StartTimeUnix     string        // Start time of the broadcast in unix format.
	Start             time.Time     // Start time in native go format for easy operations.
	EndTime           string        // End time of the broadcast in yy/mm/dd, hh:mm format.
	EndTimeUnix       string        // End time of the broadcast in unix format.
	End               time.Time     // End time in native go format for easy operations.
	VidforwardHost    string        // Host address of vidforward service.
	CameraMac         int64         // Camera hardware's MAC address.
	OnActions         string        // A series of actions to be used for power up of camera hardware.
	OffActions        string        // A series of actions to be used for power down of camera hardware.
	RTMPVar           string        // The variable name that holds the RTMP URL and key.
	Active            bool          // This is true if the broadcast is currently active i.e. waiting for data or currently streaming.
	Slate             bool          // This is true if the broadcast is currently in slate mode i.e. no camera.
	LastStatusCheck   time.Time     // Time of last status check i.e. if complete or not.
	LastChatMsg       time.Time     // Time of last chat message posted.
	LastHealthCheck   time.Time     // Time of last stream health check.
	Issues            int           // The number of successive stream issues currently experienced. Reset when good health seen.
	SendMsg           bool          // True if sensor data will be sent to the YouTube live chat.
	SensorList        []SensorEntry // List of sensors which can be reported to the YouTube live chat.
	RTMPKey           string        // The RTMP key corresponding to the newly created broadcast.
	UsingVidforward   bool          // Indicates if we're using vidforward i.e. doing long term broadcast.
	CamOn             string        // The time that the slate will be removed and the camera will turn on.
	CamOff            string        // The time that the camera will be turned off and the slate will be encoded.
	CheckingHealth    bool          // Are we performing health checks for the broadcast? Having this false is useful for dodgy testing streams.
	AttemptingToStart bool          // Indicates if we're currently attempting to start the broadcast.
	Enabled           bool          // Is the broadcast enabled? If not, it will not be started.
	Events            []string      // Holds names of events that are yet to be handled.
	Unhealthy         bool          // True if the broadcast is unhealthy.
	HardwareState     string        // Holds the current state of the hardware.
	StartFailures     int           // The number of times the broadcast has failed to start.
	Transitioning     bool          // If the broadcast is transition from live to slate, or vice versa.
	StateData         []byte        // States will be marshalled and their data stored here.
}

// SensorEntry contains the information for each sensor.
type SensorEntry struct {
	SendMsg   bool
	Sensor    iotds.SensorV2
	Name      string
	DeviceMac int64
}

type Camera struct {
	Name string // Name of camera device.
	MAC  string // Encoded MAC address of associated camera device.
}

// parseStartEnd takes the start and end time unix strings from the broadcast
// and provides these as time.Time.
func (c *BroadcastConfig) parseStartEnd() error {
	sInt, err := strconv.ParseInt(c.StartTimeUnix, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse unix start time: %w", err)
	}
	eInt, err := strconv.ParseInt(c.EndTimeUnix, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse unix end time: %w", err)
	}
	c.Start, c.End = time.Unix(sInt, 0), time.Unix(eInt, 0)
	return nil
}

// broadcastHandler handles modification to broadcast configurations.
func broadcastHandler(w http.ResponseWriter, r *http.Request) {
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	sKey, _ := profileData(profile)

	req := broadcastRequest{
		commonData: commonData{
			Pages: pages("broadcast"),
		},
		CurrentBroadcast: BroadcastConfig{
			SKey:            sKey,
			Name:            r.FormValue("broadcast-name"),
			ID:              r.FormValue("broadcast-id"),
			StreamName:      r.FormValue("stream-name"),
			Description:     r.FormValue("description"),
			Privacy:         r.FormValue("privacy"),
			Resolution:      r.FormValue("resolution"),
			StartTime:       r.FormValue("start-time"),
			StartTimeUnix:   r.FormValue("start-time-unix"),
			EndTime:         r.FormValue("end-time"),
			EndTimeUnix:     r.FormValue("end-time-unix"),
			RTMPVar:         r.FormValue("rtmp-key-var"),
			RTMPKey:         r.FormValue("rtmp-key"),
			VidforwardHost:  r.FormValue("vidforward-host"),
			CameraMac:       iotds.MacEncode(r.FormValue("camera-mac")),
			OnActions:       r.FormValue("on-actions"),
			OffActions:      r.FormValue("off-actions"),
			SendMsg:         r.FormValue("report-sensor") == "Chat",
			UsingVidforward: r.FormValue("use-vidforward") == "using-vidforward",
			CamOn:           r.FormValue("cam-on"),
			CamOff:          r.FormValue("cam-off"),
			CheckingHealth:  r.FormValue("check-health") == "checking-health",
			Enabled:         r.FormValue("enabled") == "enabled",
		},
		Action:             r.FormValue("action"),
		ListingSecondaries: r.FormValue("list-secondaries") == "listing-secondaries",
	}

	ctx := r.Context()
	req.Users, err = getUsersForSiteMenu(w, r, ctx, profile, req)
	if err != nil {
		writeTemplate(w, r, "broadcast.html", &req, fmt.Sprintf("could not populate site menu: %v", err.Error()))
		return
	}

	cfg := &req.CurrentBroadcast

	// This is how we populate the time.Time representations of the start and end
	// times.
	if cfg.StartTimeUnix != "" {
		err = cfg.parseStartEnd()
		if err != nil {
			reportError(w, r, req, "could not parse start and end times: %v", err)
			return
		}
	}

	// Load config information for any existing broadcasts that have been saved.
	req.BroadcastVars, err = iotds.GetVariablesBySite(ctx, settingsStore, sKey, broadcastScope)
	switch err {
	case nil, iotds.ErrNoSuchEntity:
	default:
		reportError(w, r, req, "could not get broadcast configs variable: %v", err)
		return
	}

	// If we're not listing secondaries, we need to filter out any secondary broadcasts.
	if !req.ListingSecondaries {
		var filteredVars []iotds.Variable
		for _, v := range req.BroadcastVars {
			if !strings.Contains(v.Name, "secondary") && !strings.Contains(v.Name, "Secondary") {
				filteredVars = append(filteredVars, v)
			}
		}
		req.BroadcastVars = filteredVars
	}

	// Try to load existing broadcast settings for newly selected broadcast.
	var loaded bool
	action := stringToAction(req.Action, req)
	if action == broadcastSelect {
		loaded, err = loadExistingSettings(r, &req)
		if err != nil {
			reportError(w, r, req, "could not load existing settings for broadcast: %v", err)
			return
		}
	}

	// Get all macs from cameras that could be used on the stream.
	devices, err := iotds.GetDevicesBySite(ctx, settingsStore, sKey)
	if err != nil {
		reportError(w, r, req, "could not get sites devices: %v", err)
		return
	}

	var cam Camera
	for _, dev := range devices {
		if dev.Type == "Camera" {
			cam = Camera{Name: dev.Name, MAC: iotds.MacDecode(dev.Mac)}
			req.Cameras = append(req.Cameras, cam)
		}
	}

	// If we loaded prior settings, rewrite the template to fill the fields.
	if loaded {
		writeTemplate(w, r, "broadcast.html", &req, "")
		return
	}

	// Populate sensor list that contains sensors that will display values in
	// live chat.
	err = updateSensorList(ctx, &req, r, settingsStore)
	if err != nil {
		reportError(w, r, req, "could not update sensor list: %v", err)
		return
	}

	switch action {
	case broadcastSave:
		err := (&OceanBroadcastClient{}).SaveBroadcast(ctx, &req.CurrentBroadcast)
		if err != nil {
			reportError(w, r, req, "could not save broadcast: %v", err)
			return
		}

	case broadcastDelete:
		err = deleteBroadcast(ctx, &req, settingsStore)
		if err != nil {
			reportError(w, r, req, "could not delete broadcast: %v", err)
			return
		}
	case vidforwardSlateUpdate:
		const fieldName = "slate-file"
		file, header, err := r.FormFile(fieldName)
		if err != nil {
			reportError(w, r, req, "could not get file from request form: %v", err)
			return
		}
		defer file.Close()
		err = (NewVidforwardService()).UploadSlate(cfg, header.Filename, file)
		if err != nil {
			reportError(w, r, req, "could not upload slate: %v", err)
			return

		}
	}

	writeTemplate(w, r, "broadcast.html", &req, "")
}

func stringToAction(s string, req broadcastRequest) Action {
	buttonPress := func(s string) Action {
		res, ok := map[string]Action{
			"":                        none,
			"broadcast-start":         broadcastStart,
			"broadcast-stop":          broadcastStop,
			"broadcast-save":          broadcastSave,
			"broadcast-token":         broadcastToken,
			"broadcast-delete":        broadcastDelete,
			"broadcast-select":        broadcastSelect,
			"vidforward-create":       vidforwardCreate,
			"vidforward-play":         vidforwardPlay,
			"vidforward-slate":        vidforwardSlate,
			"vidforward-delete":       vidforwardDelete,
			"vidforward-slate-update": vidforwardSlateUpdate,
		}[s]
		if !ok {
			panic("button string not recognised")
		}
		return res
	}(req.Action)
	return buttonPress
}

// deleteBroadcast deletes a broadcast from the datastore and also updates the BroadcastVars
// list and CurrentBroadcast config to clear the form on next page write.
func deleteBroadcast(ctx context.Context, req *broadcastRequest, store iotds.Store) error {
	cfg := &req.CurrentBroadcast
	err := iotds.DeleteVariable(ctx, store, cfg.SKey, broadcastScope+"."+cfg.Name)
	if err != nil {
		return fmt.Errorf("could not delete broadcast: %v", err)
	}

	req.BroadcastVars, err = iotds.GetVariablesBySite(ctx, store, cfg.SKey, broadcastScope)
	switch err {
	case nil, iotds.ErrNoSuchEntity:
	default:
		return fmt.Errorf("could not get broadcast variables: %v", err)
	}

	req.CurrentBroadcast = BroadcastConfig{}
	return nil
}

func loadExistingSettings(r *http.Request, req *broadcastRequest) (bool, error) {
	// First check if a broadcast has been selected.
	selected := r.FormValue("broadcast-select")
	// If the selected value is nil, this means that we have selected the new
	// broadcast option. This should return a blank request.
	if selected == "" {
		req.CurrentBroadcast = BroadcastConfig{}
		return true, nil
	}
	log.Printf("existing broadcast selected: %s", selected)
	// Check that the broadcast name selected on the UI matches one of the
	// broadcast configs that we have loaded, and set current broadcast as that.
	cfg, err := broadcastFromVars(req.BroadcastVars, selected)
	if err != nil {
		return false, fmt.Errorf("could not get broadcast from vars: %w", err)
	}
	req.CurrentBroadcast = *cfg

	return true, nil
}

func updateSensorList(ctx context.Context, req *broadcastRequest, r *http.Request, store iotds.Store) error {
	devices, err := iotds.GetDevicesBySite(ctx, store, req.CurrentBroadcast.SKey)
	if err != nil {
		return fmt.Errorf("could no get devices: %w", err)
	}

	// Load the sensor entries for the ESP device.
	for _, dev := range devices {
		if dev.Type != "esp" && dev.Type != "Controller" {
			continue
		}
		sensors, err := iotds.GetSensorsV2(ctx, store, dev.Mac)
		if err != nil {
			return fmt.Errorf("could not get sensors: %w", err)
		}
		for _, sensor := range sensors {
			entry := SensorEntry{
				SendMsg:   r.FormValue(strings.ToLower(sensor.Name)) == sensor.Name,
				Sensor:    sensor,
				Name:      strings.ToLower(sensor.Name),
				DeviceMac: dev.Mac,
			}
			req.CurrentBroadcast.SensorList = append(req.CurrentBroadcast.SensorList, entry)
		}
	}
	return nil
}

// checkBroadcastsHandler checks the broadcasts for a single site.
// It is designed to be invoked via OceanCron rpc requests, not cron.yaml.
func checkBroadcastsHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	ctx := r.Context()
	claims, err := gauth.GetClaims(r.Header.Get("Authorization"), cronSecret)
	if err != nil {
		writeHttpError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if claims["iss"] != cronServiceAccount {
		writeHttpError(w, http.StatusUnauthorized, "invalid issuer")
		return
	}
	if _, ok := claims["skey"].(float64); !ok {
		writeHttpError(w, http.StatusBadRequest, "invalid site key")
		return
	}

	skey := int64(claims["skey"].(float64))
	site, err := iotds.GetSite(ctx, settingsStore, skey)
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "error getting site %d: %v", skey, err)
		return
	}
	log.Printf("checking broadcasts for site %d", skey)
	err = checkBroadcastsForSites(ctx, []iotds.Site{*site})
	if err != nil {
		writeHttpError(w, http.StatusInternalServerError, "error checking broadcasts for site %d: %v", skey, err)
		return
	}
	fmt.Fprint(w, "OK")
}

// checkBroadcastsForSites checks broadcasts for the given sites.
func checkBroadcastsForSites(ctx context.Context, sites []iotds.Site) error {
	var cfgVars []iotds.Variable
	for _, s := range sites {
		vars, err := iotds.GetVariablesBySite(ctx, settingsStore, s.Skey, broadcastScope)
		if err != nil {
			log.Printf("could not get broadcast entities for site, skey: %d, name: %s, %v", s.Skey, s.Name, err)
			continue
		}
		cfgVars = append(cfgVars, vars...)
	}

	// If there are no entities then we don't have anything to do.
	if len(cfgVars) == 0 {
		log.Println("no broadcast configurations in datastore, doing nothing")
		return nil
	}

	// Unmarshal all the configs.
	cfgs := make([]BroadcastConfig, len(cfgVars))
	for i, v := range cfgVars {
		err := json.Unmarshal([]byte(v.Value), &cfgs[i])
		if err != nil {
			return fmt.Errorf("could not unmarshal cfg entity no. %d: %w", i, err)
		}
	}

	for i := range cfgs {
		err := performChecks(ctx, &cfgs[i], settingsStore)
		if err != nil {
			return fmt.Errorf("could not perform checks for broadcast: %s, ID: %s: %w", cfgs[i].Name, cfgs[i].ID, err)
		}
	}
	return nil
}

// performChecksInternalThroughStateMachine performs several checks on the provided
// broadcast (if enabled) using a state machine model. This function is intended to
// be used "internally", and parameterises several interfaces through which we can
// inject test implementations.
func performChecksInternalThroughStateMachine(
	ctx context.Context,
	cfg *BroadcastConfig,
	timeNow func() time.Time,
	store iotds.Store,
	svc BroadcastService,
	man BroadcastManager,
) error {
	// Don't do anything if not enabled.
	if !cfg.Enabled {
		// Also make sure it's in the idle state when not enabled, so we're not starting, transitioning or active.
		err := updateConfigWithTransaction(
			context.Background(),
			store,
			cfg.SKey,
			cfg.Name,
			func(_cfg *BroadcastConfig) error {
				_cfg.AttemptingToStart = false
				_cfg.Transitioning = false
				_cfg.Active = false
				*cfg = *_cfg
				return nil
			},
		)
		if err != nil {
			log.Printf("could not update config with callback: %v", err)
		}
		log.Printf("broadcast: %s, ID: %s, not enabled, not doing anything", cfg.Name, cfg.ID)
		return nil
	}

	log.Printf("broadcast: %s, ID: %s, performing checks", cfg.Name, cfg.ID)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// This will get called in the case that events are published to
	// the event bus but our context is cancelled. This might happen if a routine
	// is used to do a broadcast start and this function returns. We'll save them
	// to the config and then load them next time we perform checks.
	storeEventsAfterCtx := func(event event) {
		log.Printf("broadcast: %s, ID: %s, storing event after cancel: %s", cfg.Name, cfg.ID, event.String())
		err := updateConfigWithTransaction(
			context.Background(),
			store,
			cfg.SKey,
			cfg.Name,
			func(_cfg *BroadcastConfig) error {
				_cfg.Events = append(_cfg.Events, event.String())
				*cfg = *_cfg
				return nil
			},
		)
		if err != nil {
			log.Printf("could not update config with callback: %v", err)
		}
	}

	// We'll provide a custom log wrapper to the bus so that we can add the
	// broadcast name and ID to the log messages.
	busLogFunc := func(msg string, args ...interface{}) {
		idArgs := []interface{}{cfg.Name, cfg.ID}
		idArgs = append(idArgs, args...)
		log.Printf("(name: %s, id: %s) "+msg, idArgs...)
	}

	bus := newBasicEventBus(ctx, storeEventsAfterCtx, busLogFunc)

	// This handler will subscribe to the event bus and perform checks corresponding
	// to health, status and chat message events. It will also publish events to the
	// event bus in the case that the broadcast needs to be stopped or the status
	// is complete or revoked.
	healthStatusChatHandler := func(event event) error {
		switch event.(type) {
		case healthCheckDueEvent:
			handleHealthWithCallback(
				context.Background(),
				cfg,
				store,
				svc,
				func(Ctx, *Cfg, Store, Svc) error {
					bus.publish(badHealthEvent{})
					return nil
				},
				func(Ctx, *Cfg, Store, Svc) error {
					bus.publish(goodHealthEvent{})
					return nil
				},
			)
		case statusCheckDueEvent:
			err := man.HandleStatus(
				context.Background(),
				cfg,
				store,
				svc,
				func(Ctx, *Cfg, Store, Svc) error {
					bus.publish(finishEvent{})
					return nil
				},
			)
			if err != nil {
				return fmt.Errorf("could not handle status: %w", err)
			}
		case chatMessageDueEvent:
			handleChatMessage(context.Background(), cfg)
		}
		return nil
	}

	bus.subscribe(healthStatusChatHandler)

	// This context will be used by the state machines for access to our bits and bobs.
	broadcastContext := &broadcastContext{cfg, man, store, svc, NewVidforwardService(), bus, &revidCameraClient{}}

	// The hardware state machine will be responsible for the external camera hardware
	// state.
	hsm := newHardwareStateMachine(broadcastContext)
	bus.subscribe(hsm.handleEvent)

	// The broadcast state machine will be responsible for higher level broadcast control.
	sm, err := getBroadcastStateMachine(broadcastContext)
	if err != nil {
		return fmt.Errorf("could not get broadcast state machine: %w", err)
	}
	bus.subscribe(sm.handleEvent)

	// Get any events stored in the cfg that haven't been published yet
	// publish them, and then remove them from the config.
	for _, event := range cfg.Events {
		e, err := stringToEvent(event)
		if err != nil {
			log.Printf("could not convert event string to event: %v", err)
			continue
		}
		log.Printf("broadcast: %s, ID: %s, publishing stored event: %s", cfg.Name, cfg.ID, e.String())
		bus.publish(e)
	}

	err = updateConfigWithTransaction(
		context.Background(),
		store,
		cfg.SKey,
		cfg.Name,
		func(_cfg *BroadcastConfig) error {
			_cfg.Events = nil
			*cfg = *_cfg
			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("could not clear config events: %w", err)
	}

	// Now send a time event to invoke the standard periodic time based actions e.g.
	// start, stop etc.
	bus.publish(timeEvent{time.Now()})

	log.Printf("broadcast: %s, ID: %s, finishing check", cfg.Name, cfg.ID)
	return nil
}

// performChecks wraps performChecksInternal and provides implementations of the
// broadcast operations. These broadcast implementations are built around the
// broadcast package, which employs the YouTube Live API.
func performChecks(ctx context.Context, cfg *BroadcastConfig, store iotds.Store) error {
	return performChecksInternalThroughStateMachine(
		ctx,
		cfg,
		func() time.Time { return time.Now() },
		store,
		&YouTubeBroadcastService{},
		&OceanBroadcastManager{},
	)
}

type BroadcastCallback func(context.Context, *BroadcastConfig, iotds.Store, BroadcastService) error

// handleChatMessage generates a message with sensor readings for the
// relevant site and posts the message to the broadcast chat. This works by
// searching the site for any registered ESP devices and looking at the latest
// signal values on sensors which have been marked true to send a message.
func handleChatMessage(ctx context.Context, cfg *BroadcastConfig) error {
	if !cfg.SendMsg {
		log.Printf("Broadcast: %s, ID: %s, ignoring sensors", cfg.Name, cfg.ID)
		return nil
	}

	log.Printf("Broadcast: %s, ID: %s, building message", cfg.Name, cfg.ID)
	var msg string

	for _, sensor := range cfg.SensorList {
		if !sensor.SendMsg {
			continue
		}
		// Get the latest signal for the sensor.
		var qty string
		pin := strings.Split(sensor.Sensor.Pin, ".")[1]

		scalar, err := getLatestScalar(ctx, mediaStore, iotds.ToSID(iotds.MacDecode(sensor.DeviceMac), pin))
		if err == iotds.ErrNoSuchEntity {
			continue
		} else if err != nil {
			return fmt.Errorf("could not get scalar for chat message: %v", err)
		}

		value, err := sensor.Sensor.Transform(float64(scalar.Value))
		if err != nil {
			return fmt.Errorf("could not transform scalar: %v", err)
		}

		for _, q := range defaultQuantities() {
			if q.Code == sensor.Sensor.Quantity {
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
		log.Printf("Broadcast: %s, ID: %s, chat message empty", cfg.Name, cfg.ID)
		return nil
	}

	err := broadcast.PostChatMessage(cfg.CID, msg)
	if err != nil {
		return fmt.Errorf("broadcast chat message post error: %w", err)
	}
	return nil
}

type ErrInvalidEndTime struct {
	start, end time.Time
}

func (e ErrInvalidEndTime) Error() string {
	return fmt.Sprintf("end time (%v) is invalid relative to start time (%v)", e.end, e.start)
}

// saveLinkFunc provides a closure for saving a broadcast link with a given key.
func saveLinkFunc() func(string, string) error {
	return func(key, link string) error {
		key = removeDate(key)
		return iotds.PutVariable(context.Background(), settingsStore, -1, liveScope+"."+key, link)
	}
}

// extStart uses the OnActions in the provided broadcast config to perform
// external streaming hardware startup. In addition, the RTMP key is obtained
// from the broadcast's associated stream object and used to set the devices
// RTMPKey variable.
func extStart(ctx context.Context, cfg *BroadcastConfig, svc BroadcastService) error {
	if cfg.OnActions == "" {
		return nil
	}

	onActions := cfg.OnActions + "," + cfg.RTMPVar + "=" + rtmpDestinationAddress + cfg.RTMPKey
	err := setActionVars(ctx, cfg.SKey, onActions, settingsStore)
	if err != nil {
		return fmt.Errorf("could not set device variables required to start stream: %w", err)
	}

	return nil
}

// extStop uses the OffActions in the provided broadcast config to perform
// external streaming hardware shutdown.
func extStop(ctx context.Context, cfg *BroadcastConfig) error {
	if cfg.OffActions == "" {
		return nil
	}

	err := setActionVars(ctx, cfg.SKey, cfg.OffActions, settingsStore)
	if err != nil {
		return fmt.Errorf("could not set device variables to end stream: %w", err)
	}

	return nil
}

// saveBroadcast saves a broadcast configuration to the datastore with the
// variable name as the broadcast name and if the broadcast uses vidforward
// we update the vidforward configuration with a control request.
func saveBroadcast(ctx context.Context, cfg *BroadcastConfig, store iotds.Store) error {
	d, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not marshal JSON for broadcast save: %w", err)
	}

	log.Printf("broadcast: %s, ID: %s, saving, cfg: %s", cfg.Name, cfg.ID, provideConfig(cfg))
	err = iotds.PutVariable(ctx, store, cfg.SKey, broadcastScope+"."+cfg.Name, string(d))
	if err != nil {
		return fmt.Errorf("could not put broadcast data in store: %w", err)
	}

	// Ensure that the CheckBroadcast cron exists.
	c := &iotds.Cron{Skey: cfg.SKey, ID: "Broadcast Check", TOD: "* * * * *", Repeat: false, Action: "rpc", Var: "https://vidgrind.ausocean.org/checkbroadcasts", Enabled: true}
	err = iotds.PutCron(ctx, store, c)
	if err != nil {
		return fmt.Errorf("failure verifying check broadcast cron: %w", err)
	}

	return nil
}

func performRequestWithRetries(dest string, data any, maxRetries int) error {
	var retries int
retry:
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(data)
	if err != nil {
		return fmt.Errorf("could not encode data struct: %w", err)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	httpReq, err := http.NewRequest(http.MethodPut, dest, &buf)
	if err != nil {
		return fmt.Errorf("could not create new http request: %w", err)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("could not do http request, but retrying: %v", err)
		if retries <= maxRetries {
			retries++
			goto retry
		}
		return fmt.Errorf("could not do http request: %w, resp: %v", err, resp)
	}

	return nil
}

// stopBroadcast performs all necessary operations to stop a broadcast.
// We first check if the status of the broadcast is complete (it shouldn't be
// in healthy operation) and if it is not, change to complete.
// Then we change the broadcast configuration Active field to false, save this
// and stop all external streaming hardware.
func stopBroadcast(ctx context.Context, cfg *BroadcastConfig, store iotds.Store, svc BroadcastService) error {
	log.Printf("Broadcast: %s, ID: %s, stopping", cfg.Name, cfg.ID)

	status, err := svc.BroadcastStatus(ctx, cfg.ID)
	if err != nil {
		return fmt.Errorf("could not get broadcast status: %w", err)
	}

	if status != broadcast.StatusComplete && status != "" {
		err := svc.CompleteBroadcast(ctx, cfg.ID)
		if err != nil {
			return fmt.Errorf("could not complete broadcast: %w", err)
		}
	}

	cfg.Active = false
	err = saveBroadcast(ctx, cfg, store)
	if err != nil {
		return fmt.Errorf("save broadcast error: %w", err)
	}

	return nil
}

// liveHandler handles requests to /live/<broadcast name>. This redirects to the
// livestream URL stored in a variable with name corresponding to the given broadcast name.
func liveHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	ctx := r.Context()
	setup(ctx)

	key := strings.ReplaceAll(r.URL.Path, r.URL.Host+"/live/", "")
	v, err := iotds.GetVariable(ctx, settingsStore, -1, liveScope+"."+key)
	if err != nil {
		fmt.Fprintf(w, "livestream %s does not exist", key)
		return
	}

	log.Printf("redirecting to livestream link, link: %s", v.Value)
	http.Redirect(w, r, v.Value, http.StatusFound)
}

// broadcastSaveHandler handles broadcast save requests from broadcast clients.
// This is here temporarily and will move to oceantv.
// TODO: Add JWT signing
func broadcastSaveHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	ctx := r.Context()
	setup(ctx)

	ct := r.Header.Get("Content-Type")
	if ct != "application/json" {
		writeHttpErrorAndLog(w, http.StatusBadRequest, fmt.Errorf("unexpected Content-Type: %s", ct))
		return
	}

	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeHttpErrorAndLog(w, http.StatusBadRequest, err)
		return
	}

	var cfg BroadcastConfig
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		writeHttpErrorAndLog(w, http.StatusBadRequest, err)
		return
	}

	err = (&OceanBroadcastManager{}).SaveBroadcast(ctx, &cfg, settingsStore)
	if err != nil {
		writeHttpErrorAndLog(w, http.StatusInternalServerError, err)
		return
	}

	log.Printf("broadcast %s saved", cfg.Name)
	w.WriteHeader(http.StatusOK)
}

// writeHttpErrorAndLog is a wrapper for writeHttpError that adds logging.
func writeHttpErrorAndLog(w http.ResponseWriter, code int, err error) {
	writeHttpError(w, code, err.Error())
	log.Printf(err.Error())
}