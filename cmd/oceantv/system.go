package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/cmd/oceantv/forwarding"
	"github.com/ausocean/cloud/cmd/oceantv/manager"
	"github.com/ausocean/cloud/cmd/oceantv/notifier"
	"github.com/ausocean/cloud/cmd/oceantv/yt"
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

func withBroadcastManager(bm manager.Broadcast) broadcastSystemOption {
	return func(bs *broadcastSystem) error {
		bs.ctx.man = bm
		return nil
	}
}

func withBroadcastService(bs Svc) broadcastSystemOption {
	return func(b *broadcastSystem) error {
		b.ctx.svc = bs
		return nil
	}
}

func withForwardingService(fs forwarding.Service) broadcastSystemOption {
	return func(bs *broadcastSystem) error {
		bs.ctx.fwd = fs
		return nil
	}
}

func withHardwareManager(hm hardwareManager) broadcastSystemOption {
	return func(bs *broadcastSystem) error {
		bs.ctx.hardware = hm
		return nil
	}
}

func withEventBus(bus event.EventBus) broadcastSystemOption {
	return func(bs *broadcastSystem) error {
		for _, h := range bs.ctx.bus.(*event.BasicEventBus).Handlers {
			bus.Subscribe(h)
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
func withEventHandlers(h ...event.Handler) broadcastSystemOption {
	return func(bs *broadcastSystem) error {
		for _, h_ := range h {
			bs.ctx.bus.Subscribe(h_)
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
func newBroadcastSystem(ctx Ctx, store Store, cfg *Cfg, logOutput func(v ...any), options ...broadcastSystemOption) (*broadcastSystem, error) {
	if ctx.Done() == nil {
		return nil, errors.New("context must be cancellable")
	}

	// Handy log wrapper that shims with interfaces that like the
	// classic func(string, ...interface{}) signature.
	// This can be used by a lot of the components here.
	log := func(msg string, args ...interface{}) {
		broadcast.LogForBroadcast(cfg, logOutput, msg, args...)
	}

	// Create the youtube broadcast service. This will deal with the YouTube API bindings.
	tokenURI := utils.TokenURIFromAccount(cfg.Account)
	svc := yt.NewYouTubeBroadcastService(tokenURI, log)

	// Create the broadcast manager. This will manage things between the broadcast, the
	// hardware and the YouTube broadcast service.
	man := manager.NewOceanBroadcast(svc, cfg, store, log, setVar, broadcastByName)

	// This will get called in the case that events are published to
	// the event bus but our context is cancelled. This might happen if a routine
	// is used to do a broadcast start and this function returns. We'll save them
	// to the config and then load them next time we perform checks.
	storeEventsAfterCtx := func(e event.Event) {
		log("storing event after cancel: %s", e.String())
		eventData := event.MarshalEvent(e)
		try(
			man.Save(nil, func(_cfg *Cfg) {
				_cfg.Events = append(_cfg.Events, string(eventData))
			}),
			"could not update config with callback",
			log,
		)
	}

	bus := event.NewBasicEventBus(ctx, storeEventsAfterCtx, log)

	// This context will be used by the state machines for access to our bits and bobs.
	broadcastContext := &broadcastContext{cfg, man, store, svc, forwarding.NewVidforwardService(log, broadcastByName), bus, &revidCameraClient{}, logOutput, nil}

	// Subscribe event handler that notifies on events that implement errorEvent.
	bus.Subscribe(func(e event.Event) error {
		if _, ok := e.(event.Error); !ok {
			return nil
		}

		errEvent := e.(event.Error)

		broadcastContext.logAndNotify(errEvent.Kind(), "error event: %s", errEvent.Error())
		return nil
	})

	// The broadcast state machine will be responsible for higher level broadcast control.
	sm, err := getBroadcastStateMachine(broadcastContext)
	if err != nil {
		return nil, fmt.Errorf("could not get broadcast state machine: %w", err)
	}
	bus.Subscribe(sm.handleEvent)

	// The hardware state machine will be responsible for the external camera hardware
	// state.
	hsm := newHardwareStateMachine(broadcastContext)
	bus.Subscribe(hsm.handleEvent)

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
			bs.ctx.man.Save(nil, func(_cfg *Cfg) {
				_cfg.AttemptingToStart = false
				_cfg.Transitioning = false
				_cfg.Active = false
			}),
			"could not update config with callback",
			log.Printf,
		)

		// If there's a broadcast ID, set to complete if live and then clear it.
		if bs.ctx.cfg.BID != "" {
			status, err := bs.ctx.svc.BroadcastStatus(context.Background(), bs.ctx.cfg.BID)
			if err != nil {
				bs.log("could not get broadcast status: %v", err)
			} else {
				if status == yt.StatusLive {
					err = bs.ctx.svc.CompleteBroadcast(context.Background(), bs.ctx.cfg.BID)
					if err != nil {
						bs.ctx.logAndNotify(notifier.KindService, "could not complete broadcast, please check this manually: %v", err)
					}
				}
			}
			try(bs.ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.BID = "" }), "could not clear broadcast ID", bs.log)
		}

		bs.log("broadcast not enabled, not doing anything")

		return nil
	}

	for _, eventData := range bs.ctx.cfg.Events {
		event := event.UnmarshalEvent([]byte(eventData))
		bs.log("publishing stored event: %s", event.String())
		bs.ctx.bus.Publish(event)
	}

	// Remove stored events we just published from the config.
	err := bs.ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.Events = nil })
	if err != nil {
		return fmt.Errorf("could not clear config events: %w", err)
	}

	bs.ctx.bus.Publish(event.Time{time.Now()})
	return nil
}
