package main

import (
	"fmt"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/cmd/oceantv/notifier"
	"github.com/ausocean/cloud/notify"
)

type broadcastStateMachine struct {
	currentState state
	ctx          *broadcastContext
	stateHandler func(state)
}

func (sm *broadcastStateMachine) registerStateHandler(handler func(state)) {
	prev := sm.stateHandler
	sm.stateHandler = func(s state) {
		prev(s)
		handler(s)
	}
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

	err = ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.Start = ctx.cfg.Start; _cfg.End = ctx.cfg.End })
	if err != nil {
		return nil, fmt.Errorf("could not update config start and end times in transaction: %w", err)
	}

	sm := &broadcastStateMachine{currentState: broadcastCfgToState(ctx), ctx: ctx, stateHandler: func(s state) {}}
	sm.log("got broadcast state machine; initial state: %s, start: %v, end: %v, cfg: %v", stateToString(sm.currentState), ctx.cfg.Start, ctx.cfg.End, provideConfig(ctx.cfg))
	return sm, nil
}

func (sm *broadcastStateMachine) handleEvent(e event.Event) error {

	s, ok := sm.currentState.(stateWithBroadcastEventHandler)
	if !ok {
		panic(fmt.Sprintf("(bsm) current state (%T) does not implement stateWithBroadcastEventHandler", sm.currentState))
	}
	s.handleGlobalEvents(sm, e)
	s.handleEvent(sm, e)

	// After handling of the event, we may have some changes in substates of the current state.
	// So we need to update the config based on this state and possibly save some state data.
	return sm.ctx.man.Save(nil, func(_cfg *Cfg) { updateBroadcastBasedOnState(sm.currentState, _cfg) })
}

func (sm *broadcastStateMachine) transitionIfTimedOut(s state, to state, t event.Time) {
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

func (sm *broadcastStateMachine) finishIsDue(event event.Time) bool {
	if event.Time.After(sm.ctx.cfg.End) || event.Time.Before(sm.ctx.cfg.Start) {
		return true
	}
	return false
}

func (sm *broadcastStateMachine) startIsDue(event event.Time) bool {
	if event.Time.After(sm.ctx.cfg.Start) && event.Time.Before(sm.ctx.cfg.End) {
		return true
	}
	return false
}

func (sm *broadcastStateMachine) publishHealthStatusOrChatEvents(e event.Time) {
	const (
		statusInterval = 1 * time.Minute
		chatInterval   = 30 * time.Minute
	)
	sm.publishHealthEvent(e)
	now := e.Time
	if liveState, ok := sm.currentState.(liveState); ok && now.Sub(liveState.lastStatusCheck()) > statusInterval {
		liveState.setLastStatusCheck(now)
		sm.ctx.bus.Publish(event.StatusCheckDue{})
	}
	if liveState, ok := sm.currentState.(liveState); ok && now.Sub(liveState.lastChatMsg()) > chatInterval {
		liveState.setLastChatMsg(now)
		sm.ctx.bus.Publish(event.ChatMessageDue{})
	}
}

func (sm *broadcastStateMachine) publishHealthEvent(e event.Time) {
	const healthInterval = 1 * time.Minute
	now := e.Time
	if stateWithHealth, ok := sm.currentState.(stateWithHealth); ok && sm.ctx.cfg.CheckingHealth && now.Sub(stateWithHealth.lastHealthCheck()) > healthInterval {
		stateWithHealth.setLastHealthCheck(now)
		sm.ctx.bus.Publish(event.HealthCheckDue{})
	}
}

func (sm *broadcastStateMachine) transition(newState state) {
	if !try(
		sm.ctx.man.Save(nil, func(_cfg *Cfg) { updateBroadcastBasedOnState(newState, _cfg) }),
		"could not update config for transition",
		sm.logAndNotifySoftware,
	) {
		return
	}
	sm.log("transitioning from %s to %s", stateToString(sm.currentState), stateToString(newState))
	sm.currentState.exit()
	sm.currentState = newState
	sm.currentState.enter()
	sm.stateHandler(newState)
}

func (sm *broadcastStateMachine) unexpectedEvent(e event.Event, state state) {
	sm.log("unexpected event %s in current state %s", e.String(), stateToString(state))
}

func (sm *broadcastStateMachine) log(msg string, args ...interface{}) {
	sm.ctx.log("(broadcast sm) "+msg, args...)
}

func (sm *broadcastStateMachine) logAndNotify(k notify.Kind, msg string, args ...interface{}) {
	sm.ctx.logAndNotify(k, "(broadcast sm) "+msg, args...)
}

func (sm *broadcastStateMachine) logAndNotifySoftware(msg string, args ...interface{}) {
	sm.ctx.logAndNotify(notifier.KindSoftware, msg, args...)
}

func (sm *broadcastStateMachine) logAndNotifyConfiguration(msg string, args ...interface{}) {
	sm.ctx.logAndNotify(notifier.KindConfiguration, msg, args...)
}
