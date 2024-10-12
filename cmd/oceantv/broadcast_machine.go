package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ausocean/cloud/notify"
)

type broadcastStateMachine struct {
	currentState state
	ctx          *broadcastContext
}

func getBroadcastStateMachine(ctx *broadcastContext) (*broadcastStateMachine, error) {
	// First make sure the times of the config are set to today's year and day date
	// but we want to preserve the hour and min etc.
	loc, err := time.LoadLocation(locationID)
	if err != nil {
		return nil, fmt.Errorf("could not load location: %w", err)
	}

	// Get local times.
	nowInLoc := time.Now().In(loc)
	startInLoc := ctx.cfg.Start.In(loc)
	endInLoc := ctx.cfg.End.In(loc)

	// Do the day adjustment in local time because it's easier
	ctx.cfg.Start = time.Date(nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(), startInLoc.Hour(), startInLoc.Minute(), startInLoc.Second(), startInLoc.Nanosecond(), startInLoc.Location())
	ctx.cfg.End = time.Date(nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(), endInLoc.Hour(), endInLoc.Minute(), endInLoc.Second(), endInLoc.Nanosecond(), endInLoc.Location())

	// Store in UTC
	ctx.cfg.Start = ctx.cfg.Start.In(time.UTC)
	ctx.cfg.End = ctx.cfg.End.In(time.UTC)

	err = ctx.man.Save(nil, func(_cfg *BroadcastConfig) { _cfg.Start = ctx.cfg.Start; _cfg.End = ctx.cfg.End })
	if err != nil {
		return nil, fmt.Errorf("could not update config start and end times in transaction: %w", err)
	}

	sm := &broadcastStateMachine{currentState: broadcastCfgToState(ctx), ctx: ctx}
	sm.log("got broadcast state machine; initial state: %s, start: %v, end: %v, cfg: %v", stateToString(sm.currentState), ctx.cfg.Start, ctx.cfg.End, provideConfig(ctx.cfg))
	return sm, nil
}

func (sm *broadcastStateMachine) handleEvent(event event) error {
	switch event.(type) {
	case timeEvent:
		sm.handleTimeEvent(event.(timeEvent))
	case finishEvent:
		sm.handleFinishEvent(event.(finishEvent))
	case startEvent:
		sm.handleStartEvent(event.(startEvent))
	case hardwareStartedEvent:
		sm.handleHardwareStartedEvent(event.(hardwareStartedEvent))
	case hardwareStoppedEvent:
		sm.handleHardwareStoppedEvent(event.(hardwareStoppedEvent))
	case startedEvent:
		sm.handleStartedEvent(event.(startedEvent))
	case startFailedEvent:
		sm.handleStartFailedEvent(event.(startFailedEvent))
	case hardwareStartFailedEvent:
		sm.handleHardwareStartFailedEvent(event.(hardwareStartFailedEvent))
	case badHealthEvent:
		sm.handleBadHealthEvent(event.(badHealthEvent))
	case goodHealthEvent:
		sm.handleGoodHealthEvent(event.(goodHealthEvent))
	case fixFailureEvent:
		sm.handleFixFailureEvent(event.(fixFailureEvent))
	case controllerFailureEvent:
		sm.handleControllerFailureEvent(event.(controllerFailureEvent))
	case invalidConfigurationEvent:
		sm.handleInvalidConfigurationEvent(event.(invalidConfigurationEvent))
	case healthCheckDueEvent:
		sm.handleHealthCheckDueEvent(event.(healthCheckDueEvent))
	case statusCheckDueEvent:
		sm.handleStatusCheckDueEvent(event.(statusCheckDueEvent))
	case chatMessageDueEvent:
		sm.handleChatMessageDueEvent(event.(chatMessageDueEvent))
	case lowVoltageEvent:
		sm.handleLowVoltageEvent(event.(lowVoltageEvent))
	case voltageRecoveredEvent:
		sm.handleVoltageRecoveredEvent(event.(voltageRecoveredEvent))
	}

	// After handling of the event, we may have some changes in substates of the current state.
	// So we need to update the config based on this state and possibly save some state data.
	return sm.ctx.man.Save(nil, func(_cfg *BroadcastConfig) { updateBroadcastBasedOnState(sm.currentState, _cfg) })
}

func (sm *broadcastStateMachine) handleLowVoltageEvent(event lowVoltageEvent) error {
	sm.log("handling low voltage event")
	switch sm.currentState.(type) {
	case *directStarting, *vidforwardPermanentStarting, *vidforwardSecondaryStarting:
		// If we're in the starting state we need to reset the timeout to allow for
		// hardware voltage recovery (remembering that this is not our primary timeout
		// mechanism, which is handled by the hardware SM but a rather a contingency that
		// we shouldn't hit with normal behaviour).
		const broadcastVoltageRecoveryOffset = 10 * time.Minute
		sm.currentState.(stateWithTimeout).reset(time.Duration(sanatisedVoltageRecoveryTimeout(sm.ctx))*time.Hour + broadcastVoltageRecoveryOffset)
	case *vidforwardPermanentTransitionSlateToLive:
		sm.transition(newVidforwardPermanentVoltageRecoverySlate(sm.ctx))
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
	return nil
}

func (sm *broadcastStateMachine) handleVoltageRecoveredEvent(event voltageRecoveredEvent) error {
	sm.log("handling voltage recovered event")
	switch sm.currentState.(type) {
	case *directStarting, *vidforwardPermanentStarting, *vidforwardSecondaryStarting:
		sm.currentState.(stateWithTimeout).reset(5 * time.Minute)
	case *vidforwardPermanentVoltageRecoverySlate:
		sm.transition(newVidforwardPermanentTransitionSlateToLive(sm.ctx))
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
	return nil
}

func (sm *broadcastStateMachine) handleStatusCheckDueEvent(event statusCheckDueEvent) {
	err := sm.ctx.man.HandleStatus(
		context.Background(),
		sm.ctx.cfg,
		sm.ctx.store,
		sm.ctx.svc,
		func(Ctx, *Cfg, Store, Svc) error {
			sm.ctx.bus.publish(finishEvent{})
			return nil
		},
	)
	if err != nil {
		sm.logAndNotifySoftware("could not handle health check: %v", err)
	}
}

func (sm *broadcastStateMachine) handleHealthCheckDueEvent(event healthCheckDueEvent) {
	err := sm.ctx.man.HandleHealth(
		context.Background(),
		sm.ctx.cfg,
		sm.ctx.store,
		func() { sm.ctx.bus.publish(goodHealthEvent{}) },
		func(issue string) {
			sm.ctx.bus.publish(badHealthEvent{})
			sm.ctx.logAndNotify(broadcastNetwork, "poor stream health, status: %s", issue)
		},
	)
	if err != nil {
		sm.logAndNotifySoftware("could not handle health check: %v", err)
	}
}

func (sm *broadcastStateMachine) handleChatMessageDueEvent(event chatMessageDueEvent) {
	sm.ctx.man.HandleChatMessage(context.Background(), sm.ctx.cfg)
}

func (sm *broadcastStateMachine) handleInvalidConfigurationEvent(event invalidConfigurationEvent) {
	sm.logAndNotifyConfiguration("got invalid configuration event, disabling broadcast: %v", event.Error())
	try(
		sm.ctx.man.Save(nil, func(_cfg *BroadcastConfig) { _cfg.Enabled = false }),
		"could not disable broadcast after invalid configuration",
		sm.logAndNotifySoftware,
	)

	switch sm.currentState.(type) {
	case
		*vidforwardPermanentStarting,
		*vidforwardPermanentLive,
		*vidforwardPermanentLiveUnhealthy,
		*vidforwardPermanentSlate,
		*vidforwardPermanentSlateUnhealthy,
		*vidforwardPermanentTransitionLiveToSlate,
		*vidforwardPermanentTransitionSlateToLive,
		*vidforwardPermanentFailure:

		sm.transition(newVidforwardPermanentIdle(sm.ctx))

	case
		*vidforwardSecondaryStarting,
		*vidforwardSecondaryLive,
		*vidforwardSecondaryLiveUnhealthy:

		sm.transition(newVidforwardSecondaryIdle(sm.ctx))

	case
		*directStarting,
		*directLive,
		*directLiveUnhealthy:

		sm.transition(newDirectIdle(sm.ctx))

	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *broadcastStateMachine) handleStartFailedEvent(event startFailedEvent) error {
	sm.log("handling start failed event")
	switch sm.currentState.(type) {
	case *vidforwardPermanentStarting:
		sm.transition(newVidforwardPermanentIdle(sm.ctx))
	case *vidforwardSecondaryStarting:
		sm.transition(newVidforwardSecondaryIdle(sm.ctx))
	case *directStarting:
		sm.transition(newDirectIdle(sm.ctx))
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
	return nil
}

func (sm *broadcastStateMachine) handleHardwareStartFailedEvent(event hardwareStartFailedEvent) error {
	sm.log("handling hardware start failed event")
	switch sm.currentState.(type) {
	case *vidforwardPermanentStarting, *vidforwardSecondaryStarting, *directStarting:
		onFailureClosure(sm.ctx, sm.ctx.cfg, false)(errors.New("hardware start failed"))
	case *vidforwardPermanentLive, *vidforwardPermanentLiveUnhealthy:
		sm.logAndNotify(broadcastHardware, "hardware failure event in permanent live state, moving to failure slate state")
		sm.transition(newVidforwardPermanentFailure(sm.ctx))
	case *vidforwardPermanentTransitionSlateToLive:
		sm.logAndNotify(broadcastHardware, "hardware failure event in transition from slate to live, moving to failure slate state")
		sm.transition(newVidforwardPermanentFailure(sm.ctx))
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
	return nil
}

func (sm *broadcastStateMachine) handleBadHealthEvent(event badHealthEvent) error {
	sm.log("handling bad health event")
	switch sm.currentState.(type) {
	case *vidforwardPermanentLive:
		sm.transition(newVidforwardPermanentLiveUnhealthy(sm.ctx))
	case *vidforwardPermanentSlate:
		sm.transition(newVidforwardPermanentSlateUnhealthy(sm.ctx))
	case *vidforwardSecondaryLive:
		sm.transition(newVidforwardSecondaryLiveUnhealthy())
	case *directLive:
		sm.transition(newDirectLiveUnhealthy(sm.ctx))
	case *vidforwardPermanentFailure:
		sm.logAndNotify(broadcastNetwork, "getting bad health event in permanent failure state")
	case *vidforwardPermanentLiveUnhealthy, *vidforwardPermanentSlateUnhealthy, *vidforwardSecondaryLiveUnhealthy, *directLiveUnhealthy:
		// Do nothing.
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
	return nil
}

func (sm *broadcastStateMachine) handleGoodHealthEvent(event goodHealthEvent) error {
	sm.log("handling good health event")
	switch sm.currentState.(type) {
	case *vidforwardPermanentLiveUnhealthy:
		sm.transition(newVidforwardPermanentLive())
	case *vidforwardPermanentSlateUnhealthy:
		sm.transition(newVidforwardPermanentSlate())
	case *vidforwardSecondaryLiveUnhealthy:
		sm.transition(newVidforwardSecondaryLive(sm.ctx))
	case *directLiveUnhealthy:
		sm.transition(newDirectLive(sm.ctx))
	case *vidforwardPermanentTransitionLiveToSlate:
		if sm.currentState.(*vidforwardPermanentTransitionLiveToSlate).isHardwareStopped() {
			sm.transition(newVidforwardPermanentSlate())
		}
	case *vidforwardPermanentTransitionSlateToLive:
		if sm.currentState.(*vidforwardPermanentTransitionSlateToLive).isHardwareStarted() {
			sm.transition(newVidforwardPermanentLive())
		}
	default:
		// Do nothing.
	}
	return nil
}

var errControllerFailure = errors.New("controller not reporting")

func (sm *broadcastStateMachine) handleControllerFailureEvent(event controllerFailureEvent) error {
	sm.log("handling controller failure event")
	switch sm.currentState.(type) {
	case *directStarting:
		onFailureClosure(sm.ctx, sm.ctx.cfg, true)(errControllerFailure)
		sm.transition(newDirectIdle(sm.ctx))
	case *vidforwardPermanentStarting:
		onFailureClosure(sm.ctx, sm.ctx.cfg, true)(errControllerFailure)
		sm.transition(newVidforwardPermanentIdle(sm.ctx))
	case *vidforwardSecondaryStarting:
		onFailureClosure(sm.ctx, sm.ctx.cfg, true)(errControllerFailure)
		sm.transition(newVidforwardSecondaryIdle(sm.ctx))
	default:
		// Do nothing.
	}
	return nil
}

func (sm *broadcastStateMachine) handleTimeEvent(event timeEvent) {
	sm.log("handling time event: %v", event.Time)
	switch sm.currentState.(type) {
	case *vidforwardPermanentLive, *vidforwardSecondaryLive, *directLive:
		if sm.finishIsDue(event) {
			sm.ctx.bus.publish(finishEvent{})
			return
		}
		sm.publishHealthStatusOrChatEvents(event)
	case *vidforwardPermanentLiveUnhealthy, *vidforwardSecondaryLiveUnhealthy, *directLiveUnhealthy:
		if sm.finishIsDue(event) {
			sm.ctx.bus.publish(finishEvent{})
			return
		}
		sm.publishHealthStatusOrChatEvents(event)
		sm.tryToFixCurrentState()

	case *vidforwardPermanentSlateUnhealthy:
		if sm.startIsDue(event) {
			sm.ctx.bus.publish(startEvent{})
			return
		}
		sm.tryToFixCurrentState()

	case *vidforwardSecondaryIdle, *vidforwardPermanentIdle, *vidforwardPermanentSlate, *directIdle:
		if sm.startIsDue(event) {
			sm.ctx.bus.publish(startEvent{})
			return
		}
	case *vidforwardPermanentTransitionLiveToSlate:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(event.Time) {
			sm.logAndNotify(broadcastForwarder, "transition from live to slate timed out, staying in live state, check forwarding service")
			sm.transition(newVidforwardPermanentLive())
		}
		sm.publishHealthEvent(event)
	case *vidforwardSecondaryStarting:
		sm.transitionIfTimedOut(sm.currentState, newVidforwardSecondaryIdle(sm.ctx), event)
	case *directStarting:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(event.Time) {
			onFailureClosure(sm.ctx, sm.ctx.cfg, false)(errors.New("direct starting timed out"))
		}
	case *vidforwardPermanentStarting:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(event.Time) {
			onFailureClosure(sm.ctx, sm.ctx.cfg, false)(errors.New("permanent starting timed out"))
		}
	case *vidforwardPermanentTransitionSlateToLive:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(event.Time) {
			sm.ctx.logAndNotify(broadcastGeneric, "transition from slate to live timed out, transitioning to failure slate state")
			sm.transition(newVidforwardPermanentFailure(sm.ctx))
		}
		sm.publishHealthStatusOrChatEvents(event)
	case *vidforwardPermanentVoltageRecoverySlate:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(event.Time) {
			sm.transition(newVidforwardPermanentFailure(sm.ctx))
		}
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *broadcastStateMachine) handleFixFailureEvent(event fixFailureEvent) error {
	sm.log("handling fix failure event")
	switch sm.currentState.(type) {
	case *vidforwardPermanentLiveUnhealthy:
		sm.transition(newVidforwardPermanentFailure(sm.ctx))
	default:
		sm.log("unhandled event %s in current state %s", event.String(), stateToString(sm.currentState))
	}
	return nil
}

func (sm *broadcastStateMachine) transitionIfTimedOut(s state, to state, t timeEvent) {
	withTimeout := s.(stateWithTimeout)
	if withTimeout.timedOut(t.Time) {
		sm.transition(to)
	}
}

func (sm *broadcastStateMachine) tryToFixCurrentState() {
	if fixable, ok := sm.currentState.(fixableState); ok {
		sm.log("state %s is fixable, trying to fix", stateToString(sm.currentState))
		fixable.fix()
	} else {
		sm.log("state %s is not fixable", stateToString(sm.currentState))
	}
}

func (sm *broadcastStateMachine) finishIsDue(event timeEvent) bool {
	if event.Time.After(sm.ctx.cfg.End) || event.Time.Before(sm.ctx.cfg.Start) {
		return true
	}
	return false
}

func (sm *broadcastStateMachine) startIsDue(event timeEvent) bool {
	if event.Time.After(sm.ctx.cfg.Start) && event.Time.Before(sm.ctx.cfg.End) {
		return true
	}
	return false
}

func (sm *broadcastStateMachine) publishHealthStatusOrChatEvents(event timeEvent) {
	const (
		statusInterval = 1 * time.Minute
		chatInterval   = 30 * time.Minute
	)
	sm.publishHealthEvent(event)
	now := event.Time
	if liveState, ok := sm.currentState.(liveState); ok && now.Sub(liveState.lastStatusCheck()) > statusInterval {
		liveState.setLastStatusCheck(now)
		sm.ctx.bus.publish(statusCheckDueEvent{})
	}
	if liveState, ok := sm.currentState.(liveState); ok && now.Sub(liveState.lastChatMsg()) > chatInterval {
		liveState.setLastChatMsg(now)
		sm.ctx.bus.publish(chatMessageDueEvent{})
	}
}

func (sm *broadcastStateMachine) publishHealthEvent(event timeEvent) {
	const healthInterval = 1 * time.Minute
	now := event.Time
	if stateWithHealth, ok := sm.currentState.(stateWithHealth); ok && sm.ctx.cfg.CheckingHealth && now.Sub(stateWithHealth.lastHealthCheck()) > healthInterval {
		stateWithHealth.setLastHealthCheck(now)
		sm.ctx.bus.publish(healthCheckDueEvent{})
	}
}

func (sm *broadcastStateMachine) handleFinishEvent(event finishEvent) error {
	sm.log("handling finish event")
	switch sm.currentState.(type) {
	case *vidforwardPermanentLive, *vidforwardPermanentLiveUnhealthy:
		sm.transition(newVidforwardPermanentTransitionLiveToSlate(sm.ctx))
	case *vidforwardSecondaryLive, *vidforwardSecondaryLiveUnhealthy:
		sm.transition(newVidforwardSecondaryIdle(sm.ctx))
	case *directLive, *directLiveUnhealthy:
		sm.transition(newDirectIdle(sm.ctx))
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
	return nil
}

func (sm *broadcastStateMachine) handleStartEvent(event startEvent) error {
	sm.log("handling start event")
	switch sm.currentState.(type) {
	case *vidforwardPermanentIdle:
		sm.transition(newVidforwardPermanentStarting(sm.ctx))
	case *vidforwardPermanentSlate:
		sm.transition(newVidforwardPermanentTransitionSlateToLive(sm.ctx))
	case *vidforwardSecondaryIdle:
		sm.transition(newVidforwardSecondaryStarting(sm.ctx))
	case *directIdle:
		sm.transition(newDirectStarting(sm.ctx))
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
	return nil
}

func (sm *broadcastStateMachine) handleHardwareStartedEvent(event hardwareStartedEvent) error {
	sm.log("handling hardware started event")
	switch sm.currentState.(type) {
	case *directStarting, *vidforwardPermanentStarting, *vidforwardSecondaryStarting:
		startBroadcast(sm.ctx, sm.ctx.cfg)
	default: // Do nothing.
	}
	return nil
}

func (sm *broadcastStateMachine) handleHardwareStoppedEvent(event hardwareStoppedEvent) error {
	sm.log("handling hardware stopped event")
	switch sm.currentState.(type) {
	default: // Do nothing.
	}
	return nil
}

func (sm *broadcastStateMachine) handleStartedEvent(event startedEvent) error {
	sm.log("handling started event")
	switch sm.currentState.(type) {
	case *vidforwardPermanentStarting:
		sm.transition(newVidforwardPermanentLive())
	case *vidforwardSecondaryStarting:
		sm.transition(newVidforwardSecondaryLive(sm.ctx))
	case *directStarting:
		sm.transition(&directLive{})
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
	return nil
}

func (sm *broadcastStateMachine) transition(newState state) {
	if !try(
		sm.ctx.man.Save(nil, func(_cfg *BroadcastConfig) { updateBroadcastBasedOnState(newState, _cfg) }),
		"could not update config for transition",
		sm.logAndNotifySoftware,
	) {
		return
	}
	sm.log("transitioning from %s to %s", stateToString(sm.currentState), stateToString(newState))
	sm.currentState.exit()
	sm.currentState = newState
	sm.currentState.enter()
}

func (sm *broadcastStateMachine) unexpectedEvent(event event, state state) {
	sm.log("unexpected event %s in current state %s", event.String(), stateToString(state))
}

func (sm *broadcastStateMachine) log(msg string, args ...interface{}) {
	sm.ctx.log("(broadcast sm) "+msg, args...)
}

func (sm *broadcastStateMachine) logAndNotify(k notify.Kind, msg string, args ...interface{}) {
	sm.ctx.logAndNotify(k, "(broadcast sm) "+msg, args...)
}

func (sm *broadcastStateMachine) logAndNotifySoftware(msg string, args ...interface{}) {
	sm.ctx.logAndNotify(broadcastSoftware, msg, args...)
}

func (sm *broadcastStateMachine) logAndNotifyConfiguration(msg string, args ...interface{}) {
	sm.ctx.logAndNotify(broadcastConfiguration, msg, args...)
}
