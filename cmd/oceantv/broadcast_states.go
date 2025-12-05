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

	"github.com/ausocean/cloud/notify"
)

type broadcastContext struct {
	cfg      *BroadcastConfig
	man      BroadcastManager
	store    Store
	svc      BroadcastService
	fwd      ForwardingService
	bus      eventBus
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
	logForBroadcast(ctx.cfg, ctx.logOutput, msg, args...)
}

const (
	broadcastGeneric       notify.Kind = "broadcast-generic"       // Problems where cause is unknown or un-categorized.
	broadcastForwarder     notify.Kind = "broadcast-forwarder"     // Problems related to our forwarding service i.e. can't stream slate.
	broadcastHardware      notify.Kind = "broadcast-hardware"      // Problems related to streaming hardware i.e. controllers and cameras.
	broadcastNetwork       notify.Kind = "broadcast-network"       // Problems related to bad bandwidth, generally indicated by bad health events.
	broadcastSoftware      notify.Kind = "broadcast-software"      // Problems related to the functioning of our broadcast software.
	broadcastConfiguration notify.Kind = "broadcast-configuration" // Problems related to the configuration of the broadcast.
	broadcastService       notify.Kind = "broadcast-service"       // Problems related to the broadcast service e.g. YouTube API issues.
)

var errNoGlobalNotifier = errors.New("global notifier is nil")

func (ctx *broadcastContext) logAndNotify(kind notify.Kind, msg string, args ...interface{}) {
	ctx.log(msg, args...)
	// If context has nil notifier, use global notifier
	if ctx.notifier == nil {
		ctx.log("broadcast context notifier is nil, setting to global notifier")
		if notifier == nil {
			panic(errNoGlobalNotifier)
		}
		ctx.notifier = notifier
	}
	err := ctx.notifier.Send(context.Background(), ctx.cfg.SKey, kind, fmtForBroadcastLog(ctx.cfg, msg, args...))
	if err != nil {
		ctx.log("could not send health notification: %v", err)
	}
}

type state interface {
	enter()
	exit()
}

type stateFields struct{}

func (s *stateFields) enter() {}
func (s *stateFields) exit()  {}

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

func updateBroadcastBasedOnState(state state, cfg *BroadcastConfig) {
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
	isSecondary := strings.Contains(ctx.cfg.Name, secondaryBroadcastPostfix)
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
		panic(fmt.Sprintf("unknown state for broadcast, vid: %v, active: %v, slate: %v, unhealthy: %v, starting: %v, secondary: %v, transitioning: %v", vid, active, slate, unhealthy, starting, isSecondary, transitioning))
	}

	err := json.Unmarshal(ctx.cfg.StateData, &newState)
	if err != nil {
		ctx.log("unexpected error when unmarshaling state data; this could mean we have an unexpected state: %v", err)
		return newState
	}
	return newState
}

func createBroadcastAndRequestHardware(ctx *broadcastContext, cfg *BroadcastConfig, onCreation func() error) {
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
	ctx.bus.publish(hardwareStartRequestEvent{})
}

func startBroadcast(ctx *broadcastContext, cfg *BroadcastConfig) {
	onSuccess := func() {
		ctx.bus.publish(startedEvent{})
		err := ctx.man.Save(nil, func(_cfg *BroadcastConfig) { _cfg.StartFailures = 0; *cfg = *_cfg })
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

func onFailureClosure(ctx *broadcastContext, cfg *BroadcastConfig, disableOnFirstFail bool) func(err error) {
	return func(err error) {
		ctx.log("failed to start broadcast: %v", err)
		var e event
		try(ctx.man.Save(nil, func(_cfg *BroadcastConfig) {
			const maxStartFailures = 3
			_cfg.StartFailures++
			if disableOnFirstFail || _cfg.StartFailures > maxStartFailures {
				// Critical start failure event. This means we've tried too many times (which could be even once).
				e = criticalFailureEvent{fmt.Errorf("exceeded broadcast start failure limit: %w", err)}
				_cfg.StartFailures = 0
				return
			}

			// Less critical start failure event; this will give us another chance to broadcast
			// if disableOnFirstFail is false.
			e = startFailedEvent{fmt.Errorf("failed to start broadcast: %w", err)}
		}),
			"could not update config after failed start",
			ctx.log,
		)
		ctx.bus.publish(e)
	}
}

func stateToString(state state) string {
	return strings.TrimPrefix(reflect.TypeOf(state).String(), "*")
}
