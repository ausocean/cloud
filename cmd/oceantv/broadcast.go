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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/cmd/oceantv/yt"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
)

type Action int

type (
	Cfg   = broadcast.Config
	Ctx   = context.Context
	Store = datastore.Store
	Key   = datastore.Key
	Ety   = datastore.Entity
	Svc   = yt.BroadcastService
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
	liveScope                 = "Live"                                // Scope under which live stream URLs are stored.
	defaultMessage            = "Welcome to the AusOcean livestream!" // Default message to be sent to the YouTube live chat.
	tempPin                   = "X60"                                 // Standard temperature pin value.
	scalar                    = 0.1                                   // Scalar for temperature conversions from int to float.
	absZero                   = -273.15                               // Offset for temperature conversions from int to float.
	longTermBroadcastDuration = 1                                     // The duration of the long term broadcast in years.
)

type Camera struct {
	Name string // Name of camera device.
	MAC  string // Encoded MAC address of associated camera device.
}

type oceanTVService struct {
	eventHooks []eventHook
	stateHooks []stateHook
}

type oceanTVOption func(*oceanTVService) error

type eventHook func(event.Event, *Cfg)
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
func checkBroadcastsForSites(ctx Ctx, sites []model.Site, eventHooks []eventHook, stateHooks []stateHook) error {
	var cfgVars []model.Variable
	for _, s := range sites {
		vars, err := model.GetVariablesBySite(ctx, store, s.Skey, broadcast.Scope)
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
	cfgs := make([]Cfg, len(cfgVars))
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
	ctx Ctx,
	cfg *Cfg,
	timeNow func() time.Time,
	store Store,
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
	var eventHandlers []event.Handler
	for _, hook := range eventHooks {
		eventHandlers = append(eventHandlers, func(e event.Event) error {
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
func performChecks(ctx Ctx, cfg *Cfg, store Store, eventHooks []eventHook, stateHooks []stateHook) error {
	return performChecksInternalThroughStateMachine(
		ctx,
		cfg,
		func() time.Time { return time.Now() },
		store,
		eventHooks,
		stateHooks,
	)
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
		return model.PutVariable(context.Background(), store, -1, liveScope+"."+key, link)
	}
}

// extStart uses the OnActions in the provided broadcast config to perform
// external streaming hardware startup. In addition, the RTMP key is obtained
// from the broadcast's associated stream object and used to set the devices
// RTMPKey variable.
func extStart(ctx Ctx, cfg *Cfg, log func(string, ...interface{})) error {
	if cfg.OnActions == "" {
		return nil
	}

	onActions := cfg.OnActions + "," + cfg.RTMPVar + "=" + broadcast.RTMPDestinationAddress + cfg.RTMPKey
	err := setActionVars(ctx, cfg.SKey, onActions, store, log)
	if err != nil {
		return fmt.Errorf("could not set device variables required to start stream: %w", err)
	}

	return nil
}

// errNoShutdownActions represents no shutdown actions being registered for the broadcast.
var errNoShutdownActions = errors.New("no shutdown actions provided")

// SkipAction is the placeholder used to represent that the action step should be skipped.
const SkipAction = "skip"

func extShutdown(ctx Ctx, cfg *Cfg, log func(string, ...interface{})) error {
	if cfg.ShutdownActions == SkipAction {
		return broadcast.WarnSkipShutdown
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
func extStop(ctx Ctx, cfg *Cfg, log func(string, ...interface{})) error {
	if cfg.OffActions == "" {
		return nil
	}

	err := setActionVars(ctx, cfg.SKey, cfg.OffActions, store, log)
	if err != nil {
		return fmt.Errorf("could not set device variables to end stream: %w", err)
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
