package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type broadcastContext struct {
	cfg    *BroadcastConfig
	man    BroadcastManager
	store  Store
	svc    BroadcastService
	fwd    ForwardingService
	bus    eventBus
	camera hardwareManager
}

func (ctx *broadcastContext) log(msg string, args ...interface{}) {
	logForBroadcast(ctx.cfg, msg, args...)
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

	// Make sure secondary broadcast is set up.
	err := s.man.SetupSecondary(context.Background(), s.cfg, s.store)
	if err != nil {
		s.log("could not setup secondary broadcast: %v", err)
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

type vidforwardPermanentLive struct{}

func newVidforwardPermanentLive() *vidforwardPermanentLive { return &vidforwardPermanentLive{} }
func (s *vidforwardPermanentLive) enter()                  {}
func (s *vidforwardPermanentLive) exit()                   {}

type vidforwardPermanentTransitionLiveToSlate struct {
	*broadcastContext `json: "-"`
	HardwareStopped   bool
	LastEntered       time.Time
}

func newVidforwardPermanentTransitionLiveToSlate(ctx *broadcastContext) *vidforwardPermanentTransitionLiveToSlate {
	return &vidforwardPermanentTransitionLiveToSlate{broadcastContext: ctx}
}
func (s *vidforwardPermanentTransitionLiveToSlate) enter() {
	s.LastEntered = time.Now()

	s.bus.publish(hardwareStopRequestEvent{})
	err := s.fwd.Slate(s.cfg)
	if err != nil {
		s.log("could not set vidforward mode to stream: %v", err)
	}
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
	err := s.fwd.Stream(s.cfg)
	if err != nil {
		s.log("could not set vidforward mode to stream: %v", err)
	}
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
}

func newVidforwardPermanentLiveUnhealthy(ctx *broadcastContext) *vidforwardPermanentLiveUnhealthy {
	return &vidforwardPermanentLiveUnhealthy{ctx, time.Now()}
}
func (s *vidforwardPermanentLiveUnhealthy) enter() {}
func (s *vidforwardPermanentLiveUnhealthy) exit()  {}
func (s *vidforwardPermanentLiveUnhealthy) fix() {
	const resetInterval = 5 * time.Minute
	if time.Since(s.LastResetAttempt) > resetInterval {
		notifier.Send(
			context.Background(),
			s.cfg.SKey,
			"health",
			fmt.Sprintf("Broadcast %s is unhealthy, attempting hardware restart", s.cfg.Name),
		)
		err := s.fwd.Stream(s.cfg)
		if err != nil {
			s.log("could not set vidforward mode to slate: %v", err)
		}
		s.bus.publish(hardwareResetRequestEvent{})
		s.LastResetAttempt = time.Now()
	}
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
		notifier.Send(
			context.Background(),
			s.cfg.SKey,
			"health",
			fmt.Sprintf("Broadcast %s slate is unhealthy, vidforward reconfiguration", s.cfg.Name),
		)
		err := s.fwd.Slate(s.cfg)
		if err != nil {
			s.log("could not set vidforward mode to slate: %v", err)
		}
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
	*broadcastContext `json:"-"`
}

func newVidforwardSecondaryLive(ctx *broadcastContext) *vidforwardSecondaryLive {
	return &vidforwardSecondaryLive{ctx}
}

func (s *vidforwardSecondaryLive) enter() {}
func (s *vidforwardSecondaryLive) exit() {
	err := s.man.StopBroadcast(context.Background(), s.cfg, s.store, s.svc)
	if err != nil {
		s.log("could not stop broadcast")
	}
}

type vidforwardSecondaryLiveUnhealthy struct{}

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
}

func newDirectLive(ctx *broadcastContext) *directLive {
	return &directLive{ctx}
}
func (s *directLive) enter() {}
func (s *directLive) exit()  {}

type directLiveUnhealthy struct {
	*broadcastContext `json: "-"`
	LastResetAttempt  time.Time
	Attempts          int
}

func newDirectLiveUnhealthy(ctx *broadcastContext) *directLiveUnhealthy {
	return &directLiveUnhealthy{broadcastContext: ctx}
}
func (s *directLiveUnhealthy) enter() {}
func (s *directLiveUnhealthy) exit()  {}
func (s *directLiveUnhealthy) fix() {
	notify := func(msg string) {
		notifier.Send(context.Background(), s.cfg.SKey, "health", msg)
	}

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

	s.log(msg, s.Attempts)
	notify(fmt.Sprintf("broadcast: %s, id: %s) "+msg, s.cfg.Name, s.cfg.ID, s.Attempts, maxAttempts))
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
	err := s.man.StopBroadcast(context.Background(), s.cfg, s.store, s.svc)
	if err != nil {
		s.log("could not stop broadcast: %v", err)
	}
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
	vid, active, slate, unhealthy, starting, transitioning := ctx.cfg.UsingVidforward, ctx.cfg.Active, ctx.cfg.Slate, ctx.cfg.Unhealthy, ctx.cfg.AttemptingToStart, ctx.cfg.Transitioning
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
	case vid && active && slate && !unhealthy && !starting && !isSecondary && !transitioning:
		newState = newVidforwardPermanentSlate()
	case vid && active && slate && !unhealthy && !starting && !isSecondary && transitioning:
		newState = newVidforwardPermanentTransitionSlateToLive(ctx)
	case vid && active && slate && unhealthy && !starting && !isSecondary:
		newState = newVidforwardPermanentSlateUnhealthy(ctx)
	case vid && !active && !slate && !unhealthy && !starting && !isSecondary:
		newState = newVidforwardPermanentIdle(ctx)
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
	controllerIsOn, err := getDeviceStatus(context.Background(), cfg.ControllerMac, settingsStore)
	if err != nil {
		disableBroadcast(cfg, 1, fmt.Errorf("failed to get controller status: %v", err))
		return
	} else if !controllerIsOn {
		disableBroadcast(cfg, 1, fmt.Errorf("controller not reporting, likely due to low voltage"))
		return
	}
	err = ctx.man.CreateBroadcast(
		cfg,
		ctx.store,
		ctx.svc,
	)
	if err != nil {
		onFailureClosure(ctx, cfg)(fmt.Errorf("could not create broadcast: %v", err))
		return
	}
	if onCreation != nil {
		err = onCreation()
		if err != nil {
			onFailureClosure(ctx, cfg)(fmt.Errorf("could not create broadcast: %v", err))
			return
		}
	}
	ctx.bus.publish(hardwareStartRequestEvent{})
}

func startBroadcast(ctx *broadcastContext, cfg *BroadcastConfig) {
	onSuccess := func() {
		ctx.bus.publish(startedEvent{})
		err := updateConfigWithTransaction(
			context.Background(),
			ctx.store,
			cfg.SKey,
			cfg.Name,
			func(_cfg *BroadcastConfig) error {
				_cfg.StartFailures = 0
				*cfg = *_cfg
				return nil
			},
		)
		if err != nil {
			ctx.log("could not update config after successful start: %v", err)
		}
	}

	go ctx.man.StartBroadcast(
		context.Background(),
		cfg,
		ctx.store,
		ctx.svc,
		nil,
		onSuccess,
		onFailureClosure(ctx, cfg),
	)
}

func onFailureClosure(ctx *broadcastContext, cfg *BroadcastConfig) func(err error) {
	return func(err error) {
		ctx.log("failed to start broadcast: %v", err)
		ctx.bus.publish(startFailedEvent{})
		err = updateConfigWithTransaction(
			context.Background(),
			ctx.store,
			cfg.SKey,
			cfg.Name,
			func(_cfg *BroadcastConfig) error {
				_cfg.StartFailures++
				// TODO: make this configurable in config.
				const maxStartFailures = 3
				if _cfg.StartFailures >= maxStartFailures {
					disableBroadcast(_cfg, maxStartFailures, err)
				}
				*cfg = *_cfg
				return nil
			},
		)
		if err != nil {
			ctx.log("could not update config after failed start: %v", err)
		}
	}
}

func disableBroadcast(cfg *BroadcastConfig, maxStartFailures int, reason error) {
	cfg.StartFailures = 0
	cfg.Enabled = false
	notifier.Send(
		context.Background(),
		cfg.SKey,
		"health",
		fmt.Sprintf("Broadcast %s has failed to start %d times so it has been disabled. Error: %v", cfg.Name, maxStartFailures, reason),
	)
}

func stateToString(state state) string {
	return strings.TrimPrefix(reflect.TypeOf(state).String(), "*")
}
