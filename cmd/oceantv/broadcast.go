/*
DESCRIPTION
  broadcast.go provides youtube broadcast scheduling request handling.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Russell Stanley <russell@ausocean.org>

LICENSE
  Copyright (C) 2021 the Australian Ocean Lab (AusOcean)

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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

type Action int

type (
	Cfg   = BroadcastConfig
	Ctx   = context.Context
	Store = datastore.Store
	Key   = datastore.Key
	Ety   = datastore.Entity
	Svc   = BroadcastService
)

const (
	none Action = iota

	// Actions related to broadcast control.
	broadcastStart
	broadcastStop
	broadcastSave
	broadcastToken
	broadcastDelete

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

// BroadcastConfig holds configuration data for a YouTube broadcast.
type BroadcastConfig struct {
	UUID                     string        // The immutable unique key of the broadcast.
	SKey                     int64         // The key of the site this broadcast belongs to.
	Name                     string        // The name of the broadcast.
	BID                      string        // Broadcast identification.
	SID                      string        // Stream ID for any currently associated stream.
	CID                      string        // ID of associated chat.
	StreamName               string        // The name of the stream we'll bind to the broadcast.
	Description              string        // The broadcast description shown below viewing window.
	LivePrivacy              string        // Privacy of the broadcast whilst live i.e. public, private or unlisted.
	PostLivePrivacy          string        // Privacy of the broadcast after it has ended i.e. public, private or unlisted.
	Resolution               string        // Resolution of the stream e.g. 1080p.
	StartTimestamp           string        // Start time of the broadcast in unix format.
	Start                    time.Time     // Start time in native go format for easy operations.
	EndTimestamp             string        // End time of the broadcast in unix format.
	End                      time.Time     // End time in native go format for easy operations.
	VidforwardHost           string        // Host address of vidforward service.
	CameraMac                int64         // Camera hardware's MAC address.
	ControllerMAC            int64         // Controller hardware's MAC adress (controller used to power camera).
	OnActions                string        // A series of actions to be used for power up of camera hardware.
	ShutdownActions          string        // A series of actions to be used for shutdown of camera hardware.
	OffActions               string        // A series of actions to be used for power down of camera hardware.
	RTMPVar                  string        // The variable name that holds the RTMP URL and key.
	Active                   bool          // This is true if the broadcast is currently active i.e. waiting for data or currently streaming.
	Slate                    bool          // This is true if the broadcast is currently in slate mode i.e. no camera.
	Issues                   int           // The number of successive stream issues currently experienced. Reset when good health seen.
	SendMsg                  bool          // True if sensor data will be sent to the YouTube live chat.
	SensorList               []SensorEntry // List of sensors which can be reported to the YouTube live chat.
	RTMPKey                  string        // The RTMP key corresponding to the newly created broadcast.
	UsingVidforward          bool          // Indicates if we're using vidforward i.e. doing long term broadcast.
	CheckingHealth           bool          // Are we performing health checks for the broadcast? Having this false is useful for dodgy testing streams.
	AttemptingToStart        bool          // Indicates if we're currently attempting to start the broadcast.
	Enabled                  bool          // Is the broadcast enabled? If not, it will not be started.
	Events                   []string      // Holds names of events that are yet to be handled.
	Unhealthy                bool          // True if the broadcast is unhealthy.
	BroadcastState           string        // Holds the current state of the broadcast.
	HardwareState            string        // Holds the current state of the hardware.
	StartFailures            int           // The number of times the broadcast has failed to start.
	Transitioning            bool          // If the broadcast is transition from live to slate, or vice versa.
	StateData                []byte        // States will be marshalled and their data stored here.
	HardwareStateData        []byte        // Hardware states will be marshalled and their data stored here.
	Account                  string        // The YouTube account email that this broadcast is associated with.
	InFailure                bool          // True if the broadcast is in a failure state.
	BatteryVoltagePin        string        // The pin that the battery voltage is read from.
	RecoveringVoltage        bool          // True if the broadcast is currently recovering voltage.
	RequiredStreamingVoltage float64       // The required battery voltage for the camera to stream.
	VoltageRecoveryTimeout   int           // Max allowable hours for voltage recovery before failure.
	RegisterOpenFish         bool          // True if the video should be registered with openfish for annotation.
	OpenFishCaptureSource    string        // The capture source to register the stream to.
	NotifySuppressRules      string        // Suppression rules for notifications.
}

func (b *BroadcastConfig) PrettyHardwareStateData() string {
	return string(b.HardwareStateData)
}

// SensorEntry contains the information for each sensor.
type SensorEntry struct {
	SendMsg   bool
	Sensor    model.SensorV2
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
	sInt, err := strconv.ParseInt(c.StartTimestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse unix start time: %w", err)
	}
	eInt, err := strconv.ParseInt(c.EndTimestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse unix end time: %w", err)
	}
	c.Start, c.End = time.Unix(sInt, 0), time.Unix(eInt, 0)
	return nil
}

type oceanTVService struct {
	eventHooks []eventHook
	stateHooks []stateHook
}

type oceanTVOption func(*oceanTVService) error

type eventHook func(event, *Cfg)
type stateHook func(state, *Cfg)

func withEventHooks(hooks ...eventHook) oceanTVOption {
	return func(s *oceanTVService) error {
		s.eventHooks = hooks
		return nil
	}
}

func withStateHooks(hooks ...stateHook) oceanTVOption {
	return func(s *oceanTVService) error {
		s.stateHooks = hooks
		return nil
	}
}

func newOceanTVService(opts ...oceanTVOption) (*oceanTVService, error) {
	otv := &oceanTVService{}
	for i, opt := range opts {
		err := opt(otv)
		if err != nil {
			return nil, fmt.Errorf("could not apply option (%d) to oceanTV service: %w", i, err)
		}
	}
	return otv, nil
}

// checkBroadcastsHandler checks the broadcasts for a single site.
// It is designed to be invoked via OceanCron rpc requests, not cron.yaml.
func (s *oceanTVService) checkBroadcastsHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	ctx := r.Context()
	claims, err := gauth.GetClaims(r.Header.Get("Authorization"), cronSecret)
	if err != nil {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("request from %s has invalid claims: %v", r.RemoteAddr, err))
		return
	}
	if claims["iss"] != cronServiceAccount {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("request from %s has invalid issuer: %q", r.RemoteAddr, claims["iss"]))
		return
	}
	if _, ok := claims["skey"].(float64); !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("request from %s has invalid skey: %q", r.RemoteAddr, claims["skey"]))
		return
	}

	skey := int64(claims["skey"].(float64))
	site, err := model.GetSite(ctx, store, skey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("error getting site %d: %v", skey, err))
		return
	}
	log.Printf("checking broadcasts for site %d", skey)
	err = checkBroadcastsForSites(ctx, []model.Site{*site}, s.eventHooks, s.stateHooks)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("error checking broadcasts for site %d: %v", skey, err))
		return
	}
	fmt.Fprint(w, "OK")
}

// checkBroadcastsForSites checks broadcasts for the given sites.
func checkBroadcastsForSites(ctx context.Context, sites []model.Site, eventHooks []eventHook, stateHooks []stateHook) error {
	var cfgVars []model.Variable
	for _, s := range sites {
		vars, err := model.GetVariablesBySite(ctx, store, s.Skey, broadcastScope)
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
		err := performChecks(ctx, &cfgs[i], store, eventHooks, stateHooks)
		if err != nil {
			return fmt.Errorf("could not perform checks for broadcast: %s, BID: %s: %w", cfgs[i].Name, cfgs[i].BID, err)
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
	store datastore.Store,
	eventHooks []eventHook,
	stateHooks []stateHook,
) error {
	// We'll use this context to determine if anything happens after the handler
	// has returned (we might need to store states for next time).
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Construct the event handlers from the hooks.
	// We have to do this because event handlers are not on a per config basic like
	// the hooks are, so we use a closure to capture the config.
	var eventHandlers []handler
	for _, hook := range eventHooks {
		eventHandlers = append(eventHandlers, func(e event) error {
			hook(e, cfg)
			return nil
		})
	}

	// Similarly, we construct the state handlers from the hooks.
	var stateHandlers []func(state)
	for _, hook := range stateHooks {
		stateHandlers = append(stateHandlers, func(s state) {
			hook(s, cfg)
		})
	}

	sys, err := newBroadcastSystem(ctx, store, cfg, log.Println, withEventHandlers(eventHandlers...), withStateHandlers(stateHandlers...))
	if err != nil {
		return fmt.Errorf("could not create broadcast system: %w", err)
	}

	err = sys.tick()
	if err != nil {
		return fmt.Errorf("could not tick broadcast system: %w", err)
	}

	return nil
}

// performChecks wraps performChecksInternal and provides implementations of the
// broadcast operations. These broadcast implementations are built around the
// broadcast package, which employs the YouTube Live API.
func performChecks(ctx context.Context, cfg *BroadcastConfig, store datastore.Store, eventHooks []eventHook, stateHooks []stateHook) error {
	return performChecksInternalThroughStateMachine(
		ctx,
		cfg,
		func() time.Time { return time.Now() },
		store,
		eventHooks,
		stateHooks,
	)
}

type BroadcastCallback func(context.Context, *BroadcastConfig, datastore.Store, BroadcastService) error

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
		return model.PutVariable(context.Background(), store, -1, liveScope+"."+key, link)
	}
}

// extStart uses the OnActions in the provided broadcast config to perform
// external streaming hardware startup. In addition, the RTMP key is obtained
// from the broadcast's associated stream object and used to set the devices
// RTMPKey variable.
func extStart(ctx context.Context, cfg *BroadcastConfig, log func(string, ...interface{})) error {
	if cfg.OnActions == "" {
		return nil
	}

	onActions := cfg.OnActions + "," + cfg.RTMPVar + "=" + rtmpDestinationAddress + cfg.RTMPKey
	err := setActionVars(ctx, cfg.SKey, onActions, store, log)
	if err != nil {
		return fmt.Errorf("could not set device variables required to start stream: %w", err)
	}

	return nil
}

// errNoShutdownActions represents no shutdown actions being registered for the broadcast.
var errNoShutdownActions = errors.New("no shutdown actions provided")

// warnSkipShutdown is a pseudo-error which represents that shutdown was skipped.
var warnSkipShutdown = errors.New("shutdown set to skip")

// SkipAction is the placeholder used to represent that the action step should be skipped.
const SkipAction = "skip"

func extShutdown(ctx context.Context, cfg *BroadcastConfig, log func(string, ...interface{})) error {
	if cfg.ShutdownActions == SkipAction {
		return warnSkipShutdown
	}
	if cfg.ShutdownActions == "" {
		return errNoShutdownActions
	}

	err := setActionVars(ctx, cfg.SKey, cfg.ShutdownActions, store, log)
	if err != nil {
		return fmt.Errorf("could not set device variables to end stream: %w", err)
	}

	return nil
}

// extStop uses the OffActions in the provided broadcast config to perform
// external streaming hardware shutdown.
func extStop(ctx context.Context, cfg *BroadcastConfig, log func(string, ...interface{})) error {
	if cfg.OffActions == "" {
		return nil
	}

	err := setActionVars(ctx, cfg.SKey, cfg.OffActions, store, log)
	if err != nil {
		return fmt.Errorf("could not set device variables to end stream: %w", err)
	}

	return nil
}

func performRequestWithRetries(dest string, data any, maxRetries int, log func(string, ...interface{})) error {
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
		log("could not do http request, but retrying: %v", err)
		if retries <= maxRetries {
			retries++
			goto retry
		}
		return fmt.Errorf("could not do http request: %w, resp: %v", err, resp)
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
	v, err := model.GetVariable(ctx, store, -1, liveScope+"."+key)
	if err != nil {
		fmt.Fprintf(w, "livestream %s does not exist", key)
		return
	}

	log.Printf("redirecting to livestream link, link: %s", v.Value)
	http.Redirect(w, r, v.Value, http.StatusFound)
}
