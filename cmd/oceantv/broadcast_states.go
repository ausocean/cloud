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
	cfg    *BroadcastConfig
	man    BroadcastManager
	store  Store
	svc    BroadcastService
	fwd    ForwardingService
	bus    eventBus
	camera hardwareManager

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
	broadcastGeneric       notify.Kind = "broadcast-generic"       // Problems where cause is unknown.
	broadcastForwarder     notify.Kind = "broadcast-forwarder"     // Problems related to our forwarding service i.e. can't stream slate.
	broadcastHardware      notify.Kind = "broadcast-hardware"      // Problems related to streaming hardware i.e. controllers and cameras.
	broadcastNetwork       notify.Kind = "broadcast-network"       // Problems related to bad bandwidth, generally indicated by bad health events.
	broadcastSoftware      notify.Kind = "broadcast-software"      // Problems related to the functioning of our broadcast software.
	broadcastConfiguration notify.Kind = "broadcast-configuration" // Problems related to the configuration of the broadcast.
)

func (ctx *broadcastContext) logAndNotify(kind notify.Kind, msg string, args ...interface{}) {
	ctx.log(msg, args...)
	// If context has nil notifier, use global notifier
	if ctx.notifier == nil {
		ctx.log("broadcast context notifier is nil, setting to global notifier")
		if notifier == nil {
			panic("global notifier is nil")
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

type fixableState interface {
	state
	fix()
}

type stateWithTimeout interface {
	state
	timedOut(time.Time) bool
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

type vidforwardPermanentStarting struct {
	*broadcastContext `json: "-"`
	LastEntered       time.Time
}

func newVidforwardPermanentStarting(ctx *broadcastContext) *vidforwardPermanentStarting {
	return &vidforwardPermanentStarting{broadcastContext: ctx}
}

func (s *vidforwardPermanentStarting) enter() {
	s.LastEntered = time.Now()

	// Use a copy of the config so that we can adjust the end date to +1 year
	// without affecting the original config.
	cfg := *s.cfg
	cfg.End = cfg.End.AddDate(1, 0, 0)

	if !try(s.man.SetupSecondary(context.Background(), s.cfg, s.store), "could not setup secondary broadcast", s.log) {
		s.bus.publish(startFailedEvent{})
		return
	}

	// We pass this to createAndStart so that it's run after broadcast creation, therefore
	// vidforward gets up to date RTMP endpoint information.
	onBroadcastCreation := func() error {
		err := s.fwd.Stream(&cfg)
		if err != nil {
			return fmt.Errorf("could not set vidforward mode to stream: %w", err)
		}
		return nil
	}

	createBroadcastAndRequestHardware(
		s.broadcastContext,
		&cfg,
		onBroadcastCreation,
	)
}
func (s *vidforwardPermanentStarting) exit() {}
func (s *vidforwardPermanentStarting) timedOut(t time.Time) bool {
	const timeout = 5 * time.Minute
	if t.Sub(s.LastEntered) > timeout {
		s.log("timed out starting broadcast, last entered: %v, time now: %v", s.LastEntered, t)
		return true
	}
	return false
}

type vidforwardPermanentLive struct {
	liveStateFields
}

func newVidforwardPermanentLive() *vidforwardPermanentLive { return &vidforwardPermanentLive{} }
func (s *vidforwardPermanentLive) enter()                  {}
func (s *vidforwardPermanentLive) exit()                   {}

type vidforwardPermanentTransitionLiveToSlate struct {
	*broadcastContext `json: "-"`
	HardwareStopped   bool
	LastEntered       time.Time
	stateWithHealthFields
}

func newVidforwardPermanentTransitionLiveToSlate(ctx *broadcastContext) *vidforwardPermanentTransitionLiveToSlate {
	return &vidforwardPermanentTransitionLiveToSlate{broadcastContext: ctx}
}
func (s *vidforwardPermanentTransitionLiveToSlate) enter() {
	s.LastEntered = time.Now()

	s.bus.publish(hardwareStopRequestEvent{})
	try(s.fwd.Slate(s.cfg), "could not set vidforward mode to slate", s.log)
}
func (s *vidforwardPermanentTransitionLiveToSlate) exit() {}
func (s *vidforwardPermanentTransitionLiveToSlate) isHardwareStopped() bool {
	return s.cfg.HardwareState == hardwareStateToString(&hardwareOff{})
}
func (s *vidforwardPermanentTransitionLiveToSlate) timedOut(t time.Time) bool {
	const timeout = 5 * time.Minute
	if t.Sub(s.LastEntered) > timeout {
		s.log("timed out transitioning from live to slate, last entered: %v, time now: %v", s.LastEntered, t)
		return true
	}
	return false
}

type vidforwardPermanentTransitionSlateToLive struct {
	*broadcastContext `json: "-"`
	HardwareStarted   bool
	LastEntered       time.Time
}

func newVidforwardPermanentTransitionSlateToLive(ctx *broadcastContext) *vidforwardPermanentTransitionSlateToLive {
	return &vidforwardPermanentTransitionSlateToLive{broadcastContext: ctx}
}
func (s *vidforwardPermanentTransitionSlateToLive) enter() {
	s.LastEntered = time.Now()
	s.bus.publish(hardwareStartRequestEvent{})
	try(s.fwd.Stream(s.cfg), "could not set vidforward mode to stream", s.log)
}
func (s *vidforwardPermanentTransitionSlateToLive) exit() {}
func (s *vidforwardPermanentTransitionSlateToLive) isHardwareStarted() bool {
	return s.cfg.HardwareState == hardwareStateToString(&hardwareOn{})
}
func (s *vidforwardPermanentTransitionSlateToLive) timedOut(t time.Time) bool {
	const timeout = 5 * time.Minute
	if t.Sub(s.LastEntered) > timeout {
		s.log("timed out transitioning from slate to live, last entered: %v, time now: %v", s.LastEntered, t)
		return true
	}
	return false
}

type vidforwardPermanentLiveUnhealthy struct {
	*broadcastContext `json: "-"`
	LastResetAttempt  time.Time
	Attempts          int
	liveStateFields
}

func newVidforwardPermanentLiveUnhealthy(ctx *broadcastContext) *vidforwardPermanentLiveUnhealthy {
	return &vidforwardPermanentLiveUnhealthy{broadcastContext: ctx}
}
func (s *vidforwardPermanentLiveUnhealthy) enter() {}
func (s *vidforwardPermanentLiveUnhealthy) exit()  {}
func (s *vidforwardPermanentLiveUnhealthy) fix() {
	const resetInterval = 5 * time.Minute
	if time.Since(s.LastResetAttempt) <= resetInterval {
		return
	}

	s.Attempts++

	var (
		e   event
		msg string
	)

	const maxAttempts = 3
	if s.Attempts > maxAttempts {
		msg = "failed to fix permanent broadcast, transitioning to slate (attempts: %d, max attempts: %d)"
		e = fixFailureEvent{}
	} else {
		msg = "attempting to fix permanent broadcast by hardware restart and forward stream re-request (attempts: %d, max attempts: %d)"
		try(s.fwd.Stream(s.cfg), "could not set vidforward mode to stream", s.log)
		e = hardwareResetRequestEvent{}
	}

	s.logAndNotify(broadcastGeneric, msg, s.Attempts, maxAttempts)
	s.bus.publish(e)
	s.LastResetAttempt = time.Now()
}

type vidforwardPermanentFailure struct {
	*broadcastContext `json: "-"`
}

func newVidforwardPermanentFailure(ctx *broadcastContext) *vidforwardPermanentFailure {
	return &vidforwardPermanentFailure{ctx}
}
func (s *vidforwardPermanentFailure) enter() { s.requestSlate() }
func (s *vidforwardPermanentFailure) exit()  {}
func (s *vidforwardPermanentFailure) fix()   { s.requestSlate() }
func (s *vidforwardPermanentFailure) requestSlate() {
	s.bus.publish(hardwareStopRequestEvent{})
	try(s.fwd.Slate(s.cfg), "could not set vidforward mode to slate", s.log)
}

type vidforwardPermanentSlate struct{}

func newVidforwardPermanentSlate() *vidforwardPermanentSlate { return &vidforwardPermanentSlate{} }
func (s *vidforwardPermanentSlate) enter()                   {}
func (s *vidforwardPermanentSlate) exit()                    {}

type vidforwardPermanentSlateUnhealthy struct {
	*broadcastContext `json: "-"`
	LastResetAttempt  time.Time
}

func newVidforwardPermanentSlateUnhealthy(ctx *broadcastContext) *vidforwardPermanentSlateUnhealthy {
	return &vidforwardPermanentSlateUnhealthy{ctx, time.Now()}
}
func (s *vidforwardPermanentSlateUnhealthy) enter() {}
func (s *vidforwardPermanentSlateUnhealthy) exit()  {}
func (s *vidforwardPermanentSlateUnhealthy) fix() {
	const resetInterval = 5 * time.Minute
	if time.Since(s.LastResetAttempt) > resetInterval {
		s.logAndNotify(broadcastForwarder, "slate is unhealthy, requesting vidforward reconfiguration")
		try(s.fwd.Slate(s.cfg), "could not set vidforward mode to slate", s.log)
		s.LastResetAttempt = time.Now()
	}
}

type vidforwardPermanentIdle struct{ *broadcastContext }

func newVidforwardPermanentIdle(ctx *broadcastContext) *vidforwardPermanentIdle {
	return &vidforwardPermanentIdle{ctx}
}
func (s *vidforwardPermanentIdle) enter() {
	s.bus.publish(hardwareStopRequestEvent{})
}
func (s *vidforwardPermanentIdle) exit() {}

type vidforwardSecondaryLive struct {
	*broadcastContext `json: "-"`
	liveStateFields
}

func newVidforwardSecondaryLive(ctx *broadcastContext) *vidforwardSecondaryLive {
	return &vidforwardSecondaryLive{broadcastContext: ctx}
}

func (s *vidforwardSecondaryLive) enter() {}
func (s *vidforwardSecondaryLive) exit() {
	try(s.man.StopBroadcast(context.Background(), s.cfg, s.store, s.svc), "could not stop broadcast exiting secondary live", s.log)
}

type vidforwardSecondaryLiveUnhealthy struct {
	liveStateFields
}

func newVidforwardSecondaryLiveUnhealthy() *vidforwardSecondaryLiveUnhealthy {
	return &vidforwardSecondaryLiveUnhealthy{}
}
func (s *vidforwardSecondaryLiveUnhealthy) enter() {}
func (s *vidforwardSecondaryLiveUnhealthy) exit()  {}

type vidforwardSecondaryStarting struct {
	*broadcastContext `json: "-"`
	LastEntered       time.Time
}

func newVidforwardSecondaryStarting(ctx *broadcastContext) *vidforwardSecondaryStarting {
	return &vidforwardSecondaryStarting{broadcastContext: ctx}
}
func (s *vidforwardSecondaryStarting) enter() {
	s.LastEntered = time.Now()
	// We pass this to createBroadcastAndRequestHardware so that it's run after
	// broadcast creation, therefore vidforward gets up to date RTMP endpoint
	// information.
	onBroadcastCreation := func() error {
		err := s.fwd.Stream(s.cfg)
		if err != nil {
			return fmt.Errorf("could not set vidforward mode to stream: %w", err)
		}
		return nil
	}
	createBroadcastAndRequestHardware(
		s.broadcastContext,
		s.cfg,
		onBroadcastCreation,
	)
}
func (s *vidforwardSecondaryStarting) exit() {}
func (s *vidforwardSecondaryStarting) timedOut(t time.Time) bool {
	const timeout = 5 * time.Minute
	if t.Sub(s.LastEntered) > timeout {
		s.log("timed out starting broadcast, last entered: %v, time now: %v", s.LastEntered, t)
		return true
	}
	return false
}

type vidforwardSecondaryIdle struct {
	*broadcastContext `json: "-"`
}

func newVidforwardSecondaryIdle(ctx *broadcastContext) *vidforwardSecondaryIdle {
	return &vidforwardSecondaryIdle{ctx}
}
func (s *vidforwardSecondaryIdle) enter() {
	s.bus.publish(hardwareStopRequestEvent{})
}
func (s *vidforwardSecondaryIdle) exit() {}

type directLive struct {
	*broadcastContext `json: "-"`
	liveStateFields
}

func newDirectLive(ctx *broadcastContext) *directLive {
	return &directLive{broadcastContext: ctx}
}
func (s *directLive) enter() {}
func (s *directLive) exit()  {}

type directLiveUnhealthy struct {
	*broadcastContext `json: "-"`
	LastResetAttempt  time.Time
	Attempts          int
	liveStateFields
}

func newDirectLiveUnhealthy(ctx *broadcastContext) *directLiveUnhealthy {
	return &directLiveUnhealthy{broadcastContext: ctx}
}
func (s *directLiveUnhealthy) enter() {}
func (s *directLiveUnhealthy) exit()  {}
func (s *directLiveUnhealthy) fix() {
	const resetInterval = 5 * time.Minute
	if time.Since(s.LastResetAttempt) <= resetInterval {
		return
	}

	s.Attempts++

	var (
		e   event
		msg string
	)

	const maxAttempts = 3
	if s.Attempts > maxAttempts {
		msg = "failed to fix broadcast, requesting broadcast finish (attempts: %d, max attempts: %d)"
		e = finishEvent{}
	} else {
		msg = "attempting to fix broadcast by hardware restart request (attempts: %d, max attempts: %d)"
		e = hardwareResetRequestEvent{}
	}

	s.logAndNotify(broadcastHardware, msg, s.Attempts, maxAttempts)
	s.bus.publish(e)
	s.LastResetAttempt = time.Now()
}

type directStarting struct {
	*broadcastContext `json: "-"`
	LastEntered       time.Time
}

func newDirectStarting(ctx *broadcastContext) *directStarting {
	return &directStarting{broadcastContext: ctx}
}
func (s *directStarting) enter() {
	s.LastEntered = time.Now()
	createBroadcastAndRequestHardware(s.broadcastContext, s.cfg, nil)
}
func (s *directStarting) exit() {}
func (s *directStarting) timedOut(t time.Time) bool {
	const timeout = 10 * time.Minute
	if t.Sub(s.LastEntered) > timeout {
		s.log("timed out starting broadcast, last entered: %v, time now: %v", s.LastEntered, t)
		return true
	}
	return false
}

type directIdle struct {
	*broadcastContext `json: "-"`
}

func newDirectIdle(ctx *broadcastContext) *directIdle { return &directIdle{ctx} }
func (s *directIdle) enter() {
	try(s.man.StopBroadcast(context.Background(), s.cfg, s.store, s.svc), "could not stop broadcast on direct idle entry", s.log)
	s.bus.publish(hardwareStopRequestEvent{})
}
func (s *directIdle) exit() {}

func updateBroadcastBasedOnState(state state, cfg *BroadcastConfig) {
	switch state.(type) {
	case *vidforwardPermanentLive:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
	case *vidforwardPermanentTransitionLiveToSlate:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = true
	case *vidforwardPermanentLiveUnhealthy:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = true
		cfg.Transitioning = false
	case *vidforwardPermanentSlate:
		cfg.Active = true
		cfg.Slate = true
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
	case *vidforwardPermanentTransitionSlateToLive:
		cfg.Active = true
		cfg.Slate = true
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = true
	case *vidforwardPermanentSlateUnhealthy:
		cfg.Active = true
		cfg.Slate = true
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = true
		cfg.Transitioning = false
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
	case *vidforwardPermanentStarting:
		cfg.Active = false
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = true
		cfg.Unhealthy = false
		cfg.Transitioning = false
	case *vidforwardSecondaryLive:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
	case *vidforwardSecondaryLiveUnhealthy:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = true
		cfg.Transitioning = false
	case *vidforwardSecondaryIdle:
		cfg.Active = false
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
	case *vidforwardSecondaryStarting:
		cfg.Active = false
		cfg.Slate = false
		cfg.UsingVidforward = true
		cfg.AttemptingToStart = true
		cfg.Unhealthy = false
		cfg.Transitioning = false
	case *directLive:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = false
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
	case *directLiveUnhealthy:
		cfg.Active = true
		cfg.Slate = false
		cfg.UsingVidforward = false
		cfg.AttemptingToStart = false
		cfg.Unhealthy = true
		cfg.Transitioning = false
	case *directIdle:
		cfg.Active = false
		cfg.Slate = false
		cfg.UsingVidforward = false
		cfg.AttemptingToStart = false
		cfg.Unhealthy = false
		cfg.Transitioning = false
	case *directStarting:
		cfg.Active = false
		cfg.Slate = false
		cfg.UsingVidforward = false
		cfg.AttemptingToStart = true
		cfg.Unhealthy = false
		cfg.Transitioning = false
	default:
		panic(fmt.Sprintf("unknown state: %v", stateToString(state)))
	}

	var err error
	cfg.StateData, err = json.Marshal(state)
	if err != nil {
		panic(fmt.Sprintf("could not marshal state data: %v", err))
	}
}

func broadcastCfgToState(ctx *broadcastContext) state {
	isSecondary := strings.Contains(ctx.cfg.Name, secondaryBroadcastPostfix)
	var (
		vid           = ctx.cfg.UsingVidforward
		active        = ctx.cfg.Active
		slate         = ctx.cfg.Slate
		unhealthy     = ctx.cfg.Unhealthy
		starting      = ctx.cfg.AttemptingToStart
		transitioning = ctx.cfg.Transitioning
		inFailure     = ctx.cfg.InFailure
	)
	var newState state
	switch {
	case vid && !slate && !unhealthy && starting && !isSecondary:
		newState = newVidforwardPermanentStarting(ctx)
	case vid && active && !slate && !unhealthy && !starting && !isSecondary && !transitioning:
		newState = newVidforwardPermanentLive()
	case vid && active && !slate && !unhealthy && !starting && !isSecondary && transitioning:
		newState = newVidforwardPermanentTransitionLiveToSlate(ctx)
	case vid && active && !slate && unhealthy && !starting && !isSecondary:
		newState = newVidforwardPermanentLiveUnhealthy(ctx)
	case vid && active && slate && !unhealthy && !starting && !isSecondary && !transitioning && !inFailure:
		newState = newVidforwardPermanentSlate()
	case vid && active && slate && !unhealthy && !starting && !isSecondary && transitioning:
		newState = newVidforwardPermanentTransitionSlateToLive(ctx)
	case vid && active && slate && unhealthy && !starting && !isSecondary && !inFailure:
		newState = newVidforwardPermanentSlateUnhealthy(ctx)
	case vid && !active && !slate && !unhealthy && !starting && !isSecondary:
		newState = newVidforwardPermanentIdle(ctx)
	case vid && active && slate && !unhealthy && !starting && !isSecondary && inFailure:
		newState = newVidforwardPermanentFailure(ctx)
	case !vid && active && !slate && !unhealthy && !starting && isSecondary:
		fallthrough
	case vid && active && !slate && !unhealthy && !starting && isSecondary:
		newState = newVidforwardSecondaryLive(ctx)
	case !vid && active && !slate && unhealthy && !starting && isSecondary:
		fallthrough
	case vid && active && !slate && unhealthy && !starting && isSecondary:
		newState = newVidforwardSecondaryLiveUnhealthy()
	case !vid && !active && !slate && !unhealthy && !starting && isSecondary:
		fallthrough
	case vid && !active && !slate && !unhealthy && !starting && isSecondary:
		newState = newVidforwardSecondaryIdle(ctx)
	case !vid && !slate && !unhealthy && starting && isSecondary:
		fallthrough
	case vid && !slate && !unhealthy && starting && isSecondary:
		newState = newVidforwardSecondaryStarting(ctx)
	case !vid && active && !slate && !unhealthy && !starting && !isSecondary:
		newState = newDirectLive(ctx)
	case !vid && active && !slate && unhealthy && !starting && !isSecondary:
		newState = newDirectLiveUnhealthy(ctx)
	case !vid && !active && !slate && !unhealthy && !starting && !isSecondary:
		newState = newDirectIdle(ctx)
	case !vid && !slate && !unhealthy && starting && !isSecondary:
		newState = newDirectStarting(ctx)
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
		ctx.bus.publish(startFailedEvent{})
		try(ctx.man.Save(nil, func(_cfg *BroadcastConfig) {
			const maxStartFailures = 3
			if disableOnFirstFail || _cfg.StartFailures >= maxStartFailures {
				_cfg.StartFailures = 0
				_cfg.Enabled = false
				ctx.logAndNotify(broadcastGeneric, "failed to start %d times, disabling (disabled on first start: %v, error: %v)", maxStartFailures, disableOnFirstFail, err)
			}
		}),
			"could not update config after failed start",
			ctx.log,
		)
	}
}

func stateToString(state state) string {
	return strings.TrimPrefix(reflect.TypeOf(state).String(), "*")
}
