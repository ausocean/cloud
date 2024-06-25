package main

import (
	"errors"
	"fmt"
	"time"

	"context"
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

	err = updateConfigWithTransaction(
		context.Background(),
		ctx.store,
		ctx.cfg.SKey,
		ctx.cfg.Name,
		func(_cfg *BroadcastConfig) error {
			_cfg.Start = ctx.cfg.Start
			_cfg.End = ctx.cfg.End
			*ctx.cfg = *_cfg
			return nil
		},
	)
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
	}

	// After handling of the event, we may have some changes in substates of the current state.
	// So we need to update the config based on this state and possibly save some state data.
	return updateConfigWithTransaction(
		context.Background(),
		sm.ctx.store,
		sm.ctx.cfg.SKey,
		sm.ctx.cfg.Name,
		func(_cfg *BroadcastConfig) error {
			updateBroadcastBasedOnState(sm.currentState, _cfg)
			*sm.ctx.cfg = *_cfg
			return nil
		},
	)
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
		onFailureClosure(sm.ctx, sm.ctx.cfg)(errors.New("hardware start failed"))
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
		msg := "getting bad health event in permanent failure state, check forwarder"
		sm.log(msg)
		notifier.Send(context.Background(), sm.ctx.cfg.SKey, "health", fmtForBroadcastLog(sm.ctx.cfg, msg))
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
	case *vidforwardPermanentStarting, *vidforwardPermanentTransitionLiveToSlate, *vidforwardPermanentTransitionSlateToLive:
		sm.transitionIfTimedOut(sm.currentState, newVidforwardPermanentIdle(sm.ctx), event)
		sm.publishHealthEvent(event)
	case *vidforwardSecondaryStarting:
		sm.transitionIfTimedOut(sm.currentState, newVidforwardSecondaryIdle(sm.ctx), event)
	case *directStarting:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(event.Time) {
			onFailureClosure(sm.ctx, sm.ctx.cfg)(errors.New("direct starting timed out"))
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
	if now.Sub(sm.ctx.cfg.LastStatusCheck) > statusInterval {
		sm.ctx.bus.publish(statusCheckDueEvent{})
		sm.ctx.cfg.LastStatusCheck = now
	}
	if now.Sub(sm.ctx.cfg.LastChatMsg) > chatInterval {
		sm.ctx.bus.publish(chatMessageDueEvent{})
		sm.ctx.cfg.LastChatMsg = now
	}
}

func (sm *broadcastStateMachine) publishHealthEvent(event timeEvent) {
	const healthInterval = 1 * time.Minute
	now := event.Time
	if now.Sub(sm.ctx.cfg.LastHealthCheck) > healthInterval && sm.ctx.cfg.CheckingHealth {
		sm.ctx.bus.publish(healthCheckDueEvent{})
		sm.ctx.cfg.LastHealthCheck = event.Time
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
	err := updateConfigWithTransaction(
		context.Background(),
		sm.ctx.store,
		sm.ctx.cfg.SKey,
		sm.ctx.cfg.Name,
		func(_cfg *BroadcastConfig) error {
			updateBroadcastBasedOnState(newState, _cfg)
			*sm.ctx.cfg = *_cfg
			return nil
		},
	)
	if err != nil {
		sm.log("could not update config for transition: %v", err)
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
	sm.ctx.log(msg, args...)
}
