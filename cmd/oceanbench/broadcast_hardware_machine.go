package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

type hardwareRestarting struct{ *broadcastContext }

func newHardwareRestarting(ctx *broadcastContext) *hardwareRestarting {
	return &hardwareRestarting{ctx}
}

func (s *hardwareRestarting) enter() {
	s.camera.stop(s.broadcastContext)
}
func (s *hardwareRestarting) exit() {}

type hardwareStarting struct{ *broadcastContext }

func newHardwareStarting(ctx *broadcastContext) *hardwareStarting {
	return &hardwareStarting{ctx}
}
func (s *hardwareStarting) enter() {
	s.camera.start(s.broadcastContext)
}
func (s *hardwareStarting) exit() {}

type hardwareStopping struct{ *broadcastContext }

func newHardwareStopping(ctx *broadcastContext) *hardwareStopping { return &hardwareStopping{ctx} }
func (s *hardwareStopping) enter() {
	s.camera.stop(s.broadcastContext)
}
func (s *hardwareStopping) exit() {}

type hardwareOn struct{}

func newHardwareOn() *hardwareOn { return &hardwareOn{} }
func (s *hardwareOn) enter()     {}
func (s *hardwareOn) exit()      {}

type hardwareOff struct{}

func newHardwareOff() *hardwareOff { return &hardwareOff{} }
func (s *hardwareOff) enter()      {}
func (s *hardwareOff) exit()       {}

type hardwareStateMachine struct {
	currentState state
	ctx          *broadcastContext
}

func getHardwareState(ctx *broadcastContext) state {
	var _state state
	switch ctx.cfg.HardwareState {
	case "hardwareOn":
		_state = newHardwareOn()
	case "hardwareOff", "": // Also account for "" in case we haven't set the hardware state yet.
		_state = newHardwareOff()
	case "hardwareStarting":
		_state = newHardwareStarting(ctx)
	case "hardwareStopping":
		_state = newHardwareStopping(ctx)
	case "hardwareRestarting":
		_state = newHardwareRestarting(ctx)
	default:
		panic(fmt.Sprintf("invalid hardware state: %s", ctx.cfg.HardwareState))
	}
	return _state
}

func hardwareStateToString(state state) string {
	return strings.TrimPrefix(reflect.TypeOf(state).String(), "*main.")
}

func newHardwareStateMachine(ctx *broadcastContext) *hardwareStateMachine {
	sm := &hardwareStateMachine{getHardwareState(ctx), ctx}
	return sm
}

func (sm *hardwareStateMachine) handleEvent(event event) error {
	switch event.(type) {
	case timeEvent:
		sm.handleTimeEvent(event.(timeEvent))
	case hardwareStartFailedEvent:
		sm.handleHardwareStartFailedEvent(event.(hardwareStartFailedEvent))
	case hardwareStopFailedEvent:
		sm.handleHardwareStopFailedEvent(event.(hardwareStopFailedEvent))
	case hardwareStartedEvent:
		sm.handleHardwareStartedEvent(event.(hardwareStartedEvent))
	case hardwareResetRequestEvent:
		sm.handleHardwareResetRequestEvent(event.(hardwareResetRequestEvent))
	case hardwareStoppedEvent:
		sm.handleHardwareStoppedEvent(event.(hardwareStoppedEvent))
	case hardwareStartRequestEvent:
		sm.handleHardwareStartRequestEvent(event.(hardwareStartRequestEvent))
	case hardwareStopRequestEvent:
		sm.handleHardwareStopRequestEvent(event.(hardwareStopRequestEvent))
	default:
		// Do nothing.
	}
	return nil
}

func (sm *hardwareStateMachine) handleTimeEvent(_ timeEvent) {
	sm.log("handling time event")
	eventIfStatus := func(e event, status bool) {
		sm.ctx.camera.publishEventIfStatus(e, status, sm.ctx.cfg.CameraMac, sm.ctx.store, sm.log, sm.ctx.bus.publish)
	}
	switch sm.currentState.(type) {
	case *hardwareStarting:
		eventIfStatus(hardwareStartedEvent{}, true)
	case *hardwareStopping:
		eventIfStatus(hardwareStoppedEvent{}, false)
	case *hardwareRestarting:
		eventIfStatus(hardwareStartRequestEvent{}, false)

	default:
		// Do nothing.
	}
}

func (sm *hardwareStateMachine) handleHardwareStoppedEvent(event hardwareStoppedEvent) {
	sm.log("handling hardware stopped event")
	switch sm.currentState.(type) {
	case *hardwareStopping:
		sm.transition(newHardwareOff())
	case *hardwareStarting:
		sm.transition(newHardwareOff())
	case *hardwareOn:
		sm.transition(newHardwareOff())
	case *hardwareRestarting:
		// Ignore.
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStopFailedEvent(event hardwareStopFailedEvent) {
	sm.log("handling hardware stop failed event")
	switch sm.currentState.(type) {
	case *hardwareStopping:
		sm.transition(newHardwareOn())
	case *hardwareRestarting:
		sm.transition(newHardwareOn())
	case *hardwareStarting:
		// Ignore.
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStartFailedEvent(event hardwareStartFailedEvent) {
	sm.log("handling hardware start failed event")
	switch sm.currentState.(type) {
	case *hardwareStarting:
		sm.transition(newHardwareOff())
	case *hardwareRestarting:
		sm.transition(newHardwareOff())
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStartedEvent(event hardwareStartedEvent) {
	sm.log("handling hardware started event")
	switch sm.currentState.(type) {
	case *hardwareStarting:
		sm.transition(newHardwareOn())
	case *hardwareRestarting:
		sm.transition(newHardwareOn())
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStartRequestEvent(event hardwareStartRequestEvent) {
	sm.log("handling hardware start request event")
	switch sm.currentState.(type) {
	case *hardwareOff:
		sm.transition(newHardwareStarting(sm.ctx))
	case *hardwareStarting:
		sm.ctx.camera.publishEventIfStatus(hardwareStartedEvent{}, true, sm.ctx.cfg.CameraMac, sm.ctx.store, sm.log, sm.ctx.bus.publish)
	case *hardwareOn, *hardwareStopping, *hardwareRestarting:
		// Ignore.
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStopRequestEvent(event hardwareStopRequestEvent) {
	sm.log("handling hardware stop request event")
	switch sm.currentState.(type) {
	case *hardwareOn:
		sm.transition(newHardwareStopping(sm.ctx))
	case *hardwareOff, *hardwareStarting, *hardwareStopping, *hardwareRestarting:
		// Ignore.
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareResetRequestEvent(event hardwareResetRequestEvent) {
	sm.log("handling hardware reset request event")
	switch sm.currentState.(type) {
	case *hardwareOn:
		sm.transition(newHardwareRestarting(sm.ctx))
	case *hardwareOff:
		sm.transition(newHardwareStarting(sm.ctx))
	case *hardwareRestarting, *hardwareStarting, *hardwareStopping:
		// Ignore.
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *hardwareStateMachine) transition(newState state) {
	err := updateConfigWithTransaction(
		context.Background(),
		sm.ctx.store,
		sm.ctx.cfg.SKey,
		sm.ctx.cfg.Name,
		func(_cfg *BroadcastConfig) error {
			_cfg.HardwareState = hardwareStateToString(newState)
			*sm.ctx.cfg = *_cfg
			return nil
		},
	)
	if err != nil {
		sm.log("could not update hardware state in config to transition: %v", err)
		return
	}
	sm.log("transitioning from %s to %s", stateToString(sm.currentState), stateToString(newState))
	sm.currentState.exit()
	sm.currentState = newState
	sm.currentState.enter()
}

func (sm *hardwareStateMachine) unexpectedEvent(event event, state state) {
	sm.log("unexpected event %s in current state %s", event.String(), stateToString(state))
}

func (sm *hardwareStateMachine) log(format string, args ...interface{}) {
	sm.ctx.log("(hardware) "+format, args...)
}

type hardwareManager interface {
	start(ctx *broadcastContext)
	stop(ctx *broadcastContext)
	publishEventIfStatus(event event, status bool, mac int64, store Store, log func(format string, args ...interface{}), publish func(event event))
}

type revidCameraClient struct{}

func (c *revidCameraClient) start(ctx *broadcastContext) {
	err := extStart(context.Background(), ctx.cfg, ctx.svc)
	if err != nil {
		ctx.log("could not start external hardware: %v", err)
		ctx.bus.publish(hardwareStartFailedEvent{})
		return
	}
}

func (c *revidCameraClient) stop(ctx *broadcastContext) {
	err := extStop(context.Background(), ctx.cfg)
	if err != nil {
		ctx.log("could not stop external hardware: %v", err)
		ctx.bus.publish(hardwareStopFailedEvent{})
		return
	}
}

func (c *revidCameraClient) publishEventIfStatus(event event, status bool, mac int64, store Store, log func(format string, args ...interface{}), publish func(event event)) {
	alive, err := getDeviceStatus(context.Background(), mac, store)
	if err != nil {
		log("could not get device status: %v", err)
		return
	}
	if alive == status {
		publish(event)
		return
	}
}
