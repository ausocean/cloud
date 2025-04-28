package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/notify"
	"github.com/ausocean/cloud/utils"
)

// broadcastSystem represents a video broadcasting control system.
type broadcastSystem struct {
	ctx          *broadcastContext
	sm           *broadcastStateMachine
	hsm          *hardwareStateMachine
	log          func(string, ...interface{})
	stateHandler func(state)
}

type broadcastSystemOption func(*broadcastSystem) error

func withBroadcastManager(bm BroadcastManager) broadcastSystemOption {
	return func(bs *broadcastSystem) error {
		bs.ctx.man = bm
		return nil
	}
}

func withBroadcastService(bs BroadcastService) broadcastSystemOption {
	return func(b *broadcastSystem) error {
		b.ctx.svc = bs
		return nil
	}
}

func withForwardingService(fs ForwardingService) broadcastSystemOption {
	return func(bs *broadcastSystem) error {
		bs.ctx.fwd = fs
		return nil
	}
}

func withHardwareManager(hm hardwareManager) broadcastSystemOption {
	return func(bs *broadcastSystem) error {
		bs.ctx.camera = hm
		return nil
	}
}

func withEventBus(bus eventBus) broadcastSystemOption {
	return func(bs *broadcastSystem) error {
		for _, h := range bs.ctx.bus.(*basicEventBus).handlers {
			bus.subscribe(h)
		}
		bs.ctx.bus = bus
		return nil
	}
}

func withNotifier(n notify.Notifier) broadcastSystemOption {
	return func(bs *broadcastSystem) error {
		bs.ctx.notifier = n
		return nil
	}
}

// withEventHandlers allows you to provide a set of handlers that will be called
// when an event is published to the event bus.
// This is useful if you wish to notify external systems of events e.g.
// add a webhook to notify a remote system of an event.
func withEventHandlers(h ...handler) broadcastSystemOption {
	return func(bs *broadcastSystem) error {
		for _, h_ := range h {
			bs.ctx.bus.subscribe(h_)
		}
		return nil
	}
}

// withStateHandler allows you to provide a set of handlers that will be called once
// a state has been transitioned into. The new state is provided as an argument
// to the handler.
// This is useful if you wish to notify external systems of state changes e.g.
// add a webhook to notify a remote system of a state change.
func withStateHandlers(h ...func(state)) broadcastSystemOption {
	return func(bs *broadcastSystem) error {
		for _, h_ := range h {
			bs.sm.registerStateHandler(h_)
		}
		return nil
	}
}

// newBroadcastSystem creates a new broadcast system.
// Default implementations for the various components are used, but can be overridden
// by passing options to this function.
func newBroadcastSystem(ctx context.Context, store Store, cfg *BroadcastConfig, logOutput func(v ...any), options ...broadcastSystemOption) (*broadcastSystem, error) {
	if ctx.Done() == nil {
		return nil, errors.New("context must be cancellable")
	}

	// Handy log wrapper that shims with interfaces that like the
	// classic func(string, ...interface{}) signature.
	// This can be used by a lot of the components here.
	log := func(msg string, args ...interface{}) {
		logForBroadcast(cfg, logOutput, msg, args...)
	}

	// Create the youtube broadcast service. This will deal with the YouTube API bindings.
	tokenURI := utils.TokenURIFromAccount(cfg.Account)
	svc := newYouTubeBroadcastService(tokenURI, log)

	// Create the broadcast manager. This will manage things between the broadcast, the
	// hardware and the YouTube broadcast service.
	man := newOceanBroadcastManager(svc, cfg, store, log)

	// This will get called in the case that events are published to
	// the event bus but our context is cancelled. This might happen if a routine
	// is used to do a broadcast start and this function returns. We'll save them
	// to the config and then load them next time we perform checks.
	storeEventsAfterCtx := func(event event) {
		log("storing event after cancel: %s", event.String())
		eventData := marshalEvent(event)
		try(
			man.Save(nil, func(_cfg *BroadcastConfig) {
				_cfg.Events = append(_cfg.Events, string(eventData))
			}),
			"could not update config with callback",
			log,
		)
	}

	bus := newBasicEventBus(ctx, storeEventsAfterCtx, log)

	// This context will be used by the state machines for access to our bits and bobs.
	broadcastContext := &broadcastContext{cfg, man, store, svc, NewVidforwardService(log), bus, &revidCameraClient{}, logOutput, nil}

	// Subscribe event handler that notifies on events that implement errorEvent.
	// Suppress if they satisfy the notification suppression rules.
	bus.subscribe(func(event event) error {
		if _, ok := event.(errorEvent); !ok {
			return nil
		}

		errEvent := event.(errorEvent)

		// Unmarshal the notification suppression rules from the broadcast configuration.
		// It is of format:
		// {
		//  "SuppressKinds": ["broadcast-kind1" , "broadcast-kind2"],
		// 	"SuppressContaining": ["shutdown failed", "failed to start"]
		// }
		suppressionRules := &struct {
			SuppressKinds      []string
			SuppressContaining []string
		}{}

		// Completely empty string indicates no suppression rules.
		if cfg.NotifySuppressRules != "" {
			err := json.Unmarshal([]byte(cfg.NotifySuppressRules), suppressionRules)
			if err != nil {
				broadcastContext.logAndNotify(errEvent.Kind(), "could not unmarshal notification suppression rules: %v", err)
				return nil
			}
		}

		for _, kind := range suppressionRules.SuppressKinds {
			if notify.Kind(kind) == errEvent.Kind() {
				broadcastContext.log("error event: %s", errEvent.Error())
				return nil
			}
		}

		for _, cont := range suppressionRules.SuppressContaining {
			if strings.Contains(errEvent.Error(), cont) {
				broadcastContext.log("error event: %s", errEvent.Error())
				return nil
			}
		}

		broadcastContext.logAndNotify(errEvent.Kind(), "error event: %s", errEvent.Error())
		return nil
	})

	// The broadcast state machine will be responsible for higher level broadcast control.
	sm, err := getBroadcastStateMachine(broadcastContext)
	if err != nil {
		return nil, fmt.Errorf("could not get broadcast state machine: %w", err)
	}
	bus.subscribe(sm.handleEvent)

	// The hardware state machine will be responsible for the external camera hardware
	// state.
	hsm := newHardwareStateMachine(broadcastContext)
	bus.subscribe(hsm.handleEvent)

	sys := &broadcastSystem{broadcastContext, sm, hsm, log, nil}

	// Apply any options to the system.
	for _, opt := range options {
		err := opt(sys)
		if err != nil {
			return nil, fmt.Errorf("could not apply option to broadcast system: %w", err)
		}
	}

	return sys, nil
}

// tick advances the broadcast system by one time step.
// This will publish any events that weren't dealt with after context
// cancellation the last time we ticked, and then publish a time event
// to advanced the state machines again.
func (bs *broadcastSystem) tick() error {
	// If not enabled make sure we're in the idle state and have no broadcasts still in progress.
	if !bs.ctx.cfg.Enabled {
		// Make sure it's in the idle state when not enabled, so we're not starting, transitioning or active.
		try(
			bs.ctx.man.Save(nil, func(_cfg *BroadcastConfig) {
				_cfg.AttemptingToStart = false
				_cfg.Transitioning = false
				_cfg.Active = false
			}),
			"could not update config with callback",
			log.Printf,
		)

		// If there's a broadcast ID, set to complete if live and then clear it.
		if bs.ctx.cfg.ID != "" {
			status, err := bs.ctx.svc.BroadcastStatus(context.Background(), bs.ctx.cfg.ID)
			if err != nil {
				bs.log("could not get broadcast status: %v", err)
			} else {
				if status == broadcast.StatusLive {
					err = bs.ctx.svc.CompleteBroadcast(context.Background(), bs.ctx.cfg.ID)
					if err != nil {
						bs.ctx.logAndNotify(broadcastService, "could not complete broadcast, please check this manually: %v", err)
					}
				}
			}
			try(bs.ctx.man.Save(nil, func(_cfg *BroadcastConfig) { _cfg.ID = "" }), "could not clear broadcast ID", bs.log)
		}

		bs.log("broadcast not enabled, not doing anything")

		return nil
	}

	for _, eventData := range bs.ctx.cfg.Events {
		event := unmarshalEvent([]byte(eventData))
		bs.log("publishing stored event: %s", event.String())
		bs.ctx.bus.publish(event)
	}

	// Remove stored events we just published from the config.
	err := bs.ctx.man.Save(nil, func(_cfg *BroadcastConfig) { _cfg.Events = nil })
	if err != nil {
		return fmt.Errorf("could not clear config events: %w", err)
	}

	bs.ctx.bus.publish(timeEvent{time.Now()})
	return nil
}
