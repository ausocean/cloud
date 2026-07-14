package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/cmd/oceantv/forwarding"
	"github.com/ausocean/cloud/cmd/oceantv/notifier"
	"github.com/ausocean/cloud/notify"
)

type broadcastContext struct {
	cfg      *Cfg
	man      BroadcastManager
	store    Store
	svc      Svc
	fwd      forwarding.Service
	bus      event.EventBus
	hardware hardwareManager

	// When nil, defaults to log.Println. Useful to plug in test implementation.
	logOutput func(v ...any)

	// When nil, global notifier will be used. Useful to plug in test implementation.
	notifier notify.Notifier
}

func (ctx *broadcastContext) log(msg string, args ...interface{}) {
	// If context has nil log output, use standard logger log.Println.
	if ctx.logOutput == nil {
		ctx.logOutput = log.Println
	}
	broadcast.LogForBroadcast(ctx.cfg, ctx.logOutput, msg, args...)
}

var errNoGlobalNotifier = errors.New("global notifier is nil")

func (ctx *broadcastContext) logAndNotify(kind notify.Kind, msg string, args ...interface{}) {
	ctx.log(msg, args...)

	formattedMsg := fmt.Sprintf(msg, args...)

	// Unmarshal the notifier suppression rules from the broadcast configuration.
	// It is of format (case-insensitive for both kinds and containing strings):
	// {
	//  "SuppressKinds": ["broadcast-network" , "broadcast-hardware"],
	// 	"SuppressContaining": ["shutdown failed", "failed to start"]
	// }
	suppressionRules := &struct {
		SuppressKinds      []string
		SuppressContaining []string
	}{}

	if ctx.cfg.NotifySuppressRules != "" {
		err := json.Unmarshal([]byte(ctx.cfg.NotifySuppressRules), suppressionRules)
		if err != nil {
			ctx.log("could not unmarshal notifier suppression rules: %v", err)
		} else {
			for _, k := range suppressionRules.SuppressKinds {
				if strings.EqualFold(k, string(kind)) {
					ctx.log("suppressing notifier of kind %s: %s", kind, formattedMsg)
					return
				}
			}

			for _, cont := range suppressionRules.SuppressContaining {
				if strings.Contains(strings.ToLower(formattedMsg), strings.ToLower(cont)) {
					ctx.log("suppressing notifier containing %q: %s", cont, formattedMsg)
					return
				}
			}
		}
	}

	// If context has nil notifier, use global notifier
	if ctx.notifier == nil {
		ctx.log("broadcast context notifier is nil, setting to global notifier")
		if notifier.N == nil {
			panic(errNoGlobalNotifier)
		}
		ctx.notifier = notifier.N
	}
	err := ctx.notifier.Send(context.Background(), ctx.cfg.SKey, kind, broadcast.FmtForBroadcastLog(ctx.cfg, msg, args...))
	if err != nil {
		ctx.log("could not send health notifier: %v", err)
	}
}

type state interface {
	enter()
	exit()
}

type stateFields struct{}

func (s *stateFields) enter() {}
func (s *stateFields) exit()  {}

type stateWithBroadcastEventHandler interface {
	handleGlobalEvents(sm *broadcastStateMachine, e event.Event)
	handleEvent(sm *broadcastStateMachine, e event.Event)
}

func (b *stateFields) handleGlobalEvents(sm *broadcastStateMachine, e event.Event) {
	switch e.(type) {
	case event.StatusCheckDue:
		err := sm.ctx.man.HandleStatus(
			context.Background(),
			sm.ctx.cfg,
			sm.ctx.store,
			sm.ctx.svc,
			func(Ctx, *Cfg, Store, Svc) error {
				sm.ctx.bus.Publish(event.Finish{})
				return nil
			},
		)
		if err != nil {
			sm.logAndNotifySoftware("could not handle health check: %v", err)
		}
	case event.HealthCheckDue:
		err := sm.ctx.man.HandleHealth(
			context.Background(),
			sm.ctx.cfg,
			sm.ctx.store,
			func() { sm.ctx.bus.Publish(event.GoodHealth{}) },
			func(issue string) {
				sm.ctx.bus.Publish(event.BadHealth{})
				sm.ctx.logAndNotify(notifier.KindNetwork, "poor stream health, status: %s", issue)
			},
		)
		if err != nil {
			sm.logAndNotifySoftware("could not handle health check: %v", err)
		}
	case event.ChatMessageDue:
		sm.ctx.man.HandleChatMessage(context.Background(), sm.ctx.cfg)
	}
}

type fixableState interface {
	state
	fix()
}

type stateWithTimeout interface {
	state
	timedOut(time.Time) bool
	reset(time.Duration)
}

type stateWithTimeoutFields struct {
	*broadcastContext `json: "-"`
	LastEntered       time.Time
	Timeout           time.Duration
}

func newStateWithTimeoutFields(ctx *broadcastContext) stateWithTimeoutFields {
	const defaultTimeout = 5 * time.Minute
	return stateWithTimeoutFields{broadcastContext: ctx, Timeout: defaultTimeout}
}

func newStateWithTimeoutFieldsWithLastEntered(ctx *broadcastContext, lastEntered time.Time) stateWithTimeoutFields {
	const defaultTimeout = 5 * time.Minute
	return stateWithTimeoutFields{broadcastContext: ctx, LastEntered: lastEntered, Timeout: defaultTimeout}
}

func newStateWithTimeoutFieldsWithTimeout(ctx *broadcastContext, timeout time.Duration) stateWithTimeoutFields {
	return stateWithTimeoutFields{broadcastContext: ctx, Timeout: timeout}
}

func (s *stateWithTimeoutFields) timedOut(t time.Time) bool {
	if s.LastEntered.IsZero() {
		panic("last entered time is not being updated")
	}
	if t.Sub(s.LastEntered) > s.Timeout {
		s.log("timed out, last entered: %v, time now: %v", s.LastEntered, t)
		return true
	}
	return false
}

func (s *stateWithTimeoutFields) reset(d time.Duration) {
	s.LastEntered = time.Now()
	s.Timeout = d
}

type stateWithHealth interface {
	lastHealthCheck() time.Time
	setLastHealthCheck(time.Time)
}

type liveState interface {
	stateWithHealth
	lastStatusCheck() time.Time
	lastChatMsg() time.Time
	setLastStatusCheck(time.Time)
	setLastChatMsg(time.Time)
}

type stateWithHealthFields struct {
	LastHealthCheck time.Time
}

func (s *stateWithHealthFields) lastHealthCheck() time.Time     { return s.LastHealthCheck }
func (s *stateWithHealthFields) setLastHealthCheck(t time.Time) { s.LastHealthCheck = t }

type liveStateFields struct {
	stateWithHealthFields
	LastStatusCheck time.Time
	LastChatMsg     time.Time
}

func (s *liveStateFields) lastStatusCheck() time.Time     { return s.LastStatusCheck }
func (s *liveStateFields) lastChatMsg() time.Time         { return s.LastChatMsg }
func (s *liveStateFields) setLastStatusCheck(t time.Time) { s.LastStatusCheck = t }
func (s *liveStateFields) setLastChatMsg(t time.Time)     { s.LastChatMsg = t }

func updateBroadcastBasedOnState(state state, cfg *Cfg) {
	switch state.(type) {
	case *vidforwardPermanentLive:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
		cfg.InFailure = false
	case *vidforwardPermanentTransitionLiveToSlate:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = true
		cfg.InFailure = false
	case *vidforwardPermanentLiveUnhealthy:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = true
		cfg.Transitioning = false
		cfg.InFailure = false
	case *vidforwardPermanentSlate:
		cfg.Active = true
		cfg.Slate = true
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
		cfg.InFailure = false
	case *vidforwardPermanentTransitionSlateToLive:
		cfg.Active = true
		cfg.Slate = true
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = true
		cfg.RecoveringVoltage = false
		cfg.InFailure = false
	case *vidforwardPermanentVoltageRecoverySlate:
		cfg.Active = true
		cfg.Slate = true
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = true
		cfg.RecoveringVoltage = true
		cfg.InFailure = false
	case *vidforwardPermanentSlateUnhealthy:
		cfg.Active = true
		cfg.Slate = true
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = true
		cfg.Transitioning = false
		cfg.InFailure = false
	case *vidforwardPermanentFailure:
		cfg.Active = true
		cfg.Slate = true
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
		cfg.InFailure = true
	case *vidforwardPermanentIdle:
		cfg.Active = false
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
		cfg.InFailure = false
	case *vidforwardPermanentStarting:
		cfg.Active = false
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = true
		cfg.Unhealthy = false
		cfg.Transitioning = false
		cfg.InFailure = false
	case *vidforwardSecondaryLive:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
		cfg.InFailure = false
	case *vidforwardSecondaryLiveUnhealthy:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = true
		cfg.Transitioning = false
		cfg.InFailure = false
	case *vidforwardSecondaryIdle:
		cfg.Active = false
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
		cfg.InFailure = false
	case *vidforwardSecondaryStarting:
		cfg.Active = false
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = true
		cfg.Unhealthy = false
		cfg.Transitioning = false
		cfg.InFailure = false
	case *directLive:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = false
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
		cfg.InFailure = false
	case *directLiveUnhealthy:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = false
		cfg.AttemptingToStart = false
		cfg.Unhealthy = true
		cfg.Transitioning = false
		cfg.InFailure = false
	case *directIdle:
		cfg.Active = false
		cfg.Slate = false
		cfg.UsingVidforward = false
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
		cfg.InFailure = false
	case *directFailure:
		cfg.Active = false
		cfg.Slate = false
		cfg.UsingVidforward = false
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
		cfg.InFailure = true
	case *directStarting:
		cfg.Active = false
		cfg.Slate = false
		cfg.UsingVidforward = false
		cfg.AttemptingToStart = true
		cfg.Unhealthy = false
		cfg.Transitioning = false
		cfg.InFailure = false
	default:
		panic(fmt.Sprintf("unknown state: %v", stateToString(state)))
	}

	var err error
	cfg.StateData, err = json.Marshal(state)
	if err != nil {
		panic(fmt.Sprintf("could not marshal state data: %v", err))
	}

	cfg.BroadcastState = stateToString(state)
}

func broadcastCfgToState(ctx *broadcastContext) state {
	isSecondary := strings.Contains(ctx.cfg.Name, broadcast.SecondaryPostfix)
	var (
		vid               = ctx.cfg.UsingVidforward
		active            = ctx.cfg.Active
		slate             = ctx.cfg.Slate
		unhealthy         = ctx.cfg.Unhealthy
		starting          = ctx.cfg.AttemptingToStart
		transitioning     = ctx.cfg.Transitioning
		inFailure         = ctx.cfg.InFailure
		recoveringVoltage = ctx.cfg.RecoveringVoltage
	)
	var newState state
	switch {
	case vid && !slate && !unhealthy && starting && !isSecondary && !inFailure:
		newState = newVidforwardPermanentStarting(ctx)
	case vid && active && !slate && !unhealthy && !starting && !isSecondary && !transitioning && !inFailure:
		newState = newVidforwardPermanentLive()
	case vid && active && !slate && !unhealthy && !starting && !isSecondary && transitioning && !inFailure:
		newState = newVidforwardPermanentTransitionLiveToSlate(ctx)
	case vid && active && !slate && unhealthy && !starting && !isSecondary && !inFailure:
		newState = newVidforwardPermanentLiveUnhealthy(ctx)
	case vid && active && slate && !unhealthy && !starting && !isSecondary && !transitioning && !inFailure:
		newState = newVidforwardPermanentSlate()
	case vid && active && slate && !unhealthy && !starting && !isSecondary && transitioning && !recoveringVoltage && !inFailure:
		newState = newVidforwardPermanentTransitionSlateToLive(ctx)
	case vid && active && slate && !unhealthy && !starting && !isSecondary && transitioning && recoveringVoltage && !inFailure:
		newState = newVidforwardPermanentVoltageRecoverySlate(ctx)
	case vid && active && slate && unhealthy && !starting && !isSecondary && !inFailure:
		newState = newVidforwardPermanentSlateUnhealthy(ctx)
	case vid && !active && !slate && !unhealthy && !starting && !isSecondary && !inFailure:
		newState = newVidforwardPermanentIdle(ctx)
	case vid && active && slate && !unhealthy && !starting && !isSecondary && inFailure:
		newState = newVidforwardPermanentFailure(ctx)
	case !vid && active && !slate && !unhealthy && !starting && isSecondary && !inFailure:
		fallthrough
	case vid && active && !slate && !unhealthy && !starting && isSecondary && !inFailure:
		newState = newVidforwardSecondaryLive(ctx)
	case !vid && active && !slate && unhealthy && !starting && isSecondary && !inFailure:
		fallthrough
	case vid && active && !slate && unhealthy && !starting && isSecondary && !inFailure:
		newState = newVidforwardSecondaryLiveUnhealthy()
	case !vid && !active && !slate && !unhealthy && !starting && isSecondary && !inFailure:
		fallthrough
	case vid && !active && !slate && !unhealthy && !starting && isSecondary && !inFailure:
		newState = newVidforwardSecondaryIdle(ctx)
	case !vid && !slate && !unhealthy && starting && isSecondary && !inFailure:
		fallthrough
	case vid && !slate && !unhealthy && starting && isSecondary && !inFailure:
		newState = newVidforwardSecondaryStarting(ctx)
	case !vid && active && !slate && !unhealthy && !starting && !isSecondary && !inFailure:
		newState = newDirectLive(ctx)
	case !vid && active && !slate && unhealthy && !starting && !isSecondary && !inFailure:
		newState = newDirectLiveUnhealthy(ctx)
	case !vid && !active && !slate && !unhealthy && !starting && !isSecondary && !inFailure:
		newState = newDirectIdle(ctx)
	case !vid && !slate && !unhealthy && starting && !isSecondary && !inFailure:
		newState = newDirectStarting(ctx)
	case !vid && !slate && !unhealthy && !starting && !isSecondary && inFailure:
		newState = newDirectFailure(ctx, nil)
	default:
		panic(fmt.Sprintf("unknown state for broadcast %v, vid: %v, active: %v, slate: %v, unhealthy: %v, starting: %v, secondary: %v, transitioning: %v", ctx.cfg.Name, vid, active, slate, unhealthy, starting, isSecondary, transitioning))
	}

	err := json.Unmarshal(ctx.cfg.StateData, &newState)
	if err != nil {
		ctx.log("unexpected error when unmarshaling state data; this could mean we have an unexpected state: %v", err)
		return newState
	}
	return newState
}

func createBroadcastAndRequestHardware(ctx *broadcastContext, cfg *Cfg, onCreation func() error) {
	err := ctx.man.CreateBroadcast(
		cfg,
		ctx.store,
		ctx.svc,
	)
	if errors.Is(err, ErrRequestLimitExceeded) {
		onFailureClosure(ctx, cfg, true)(fmt.Errorf("could not create broadcast: %w", err))
		return
	}
	if err != nil {
		onFailureClosure(ctx, cfg, false)(fmt.Errorf("could not create broadcast: %w", err))
		return
	}
	if onCreation != nil {
		err = onCreation()
		if err != nil {
			onFailureClosure(ctx, cfg, false)(fmt.Errorf("could not create broadcast: %v", err))
			return
		}
	}
	ctx.bus.Publish(event.HardwareStartRequest{})
}

func startBroadcast(ctx *broadcastContext, cfg *Cfg) {
	onSuccess := func() {
		ctx.bus.Publish(event.Started{})
		err := ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.StartFailures = 0; *cfg = *_cfg })
		if err != nil {
			ctx.log("could not update config after successful start: %v", err)
		}
	}

	ctx.man.StartBroadcast(
		context.Background(),
		cfg,
		ctx.store,
		ctx.svc,
		nil,
		onSuccess,
		onFailureClosure(ctx, cfg, false),
	)
}

func onFailureClosure(ctx *broadcastContext, cfg *Cfg, disableOnFirstFail bool) func(err error) {
	return func(err error) {
		ctx.log("failed to start broadcast: %v", err)
		var e event.Event
		try(ctx.man.Save(nil, func(_cfg *Cfg) {
			const maxStartFailures = 3
			_cfg.StartFailures++
			if disableOnFirstFail || _cfg.StartFailures > maxStartFailures {
				// Critical start failure event. This means we've tried too many times (which could be even once).
				e = event.CriticalFailure{fmt.Errorf("exceeded broadcast start failure limit: %w", err)}
				_cfg.StartFailures = 0
				return
			}

			// Less critical start failure event; this will give us another chance to broadcast
			// if disableOnFirstFail is false.
			e = event.StartFailed{fmt.Errorf("failed to start broadcast: %w", err)}
		}),
			"could not update config after failed start",
			ctx.log,
		)
		ctx.bus.Publish(e)
	}
}

func stateToString(state state) string {
	return strings.TrimPrefix(reflect.TypeOf(state).String(), "*")
}
