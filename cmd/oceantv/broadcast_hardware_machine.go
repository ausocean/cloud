package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/cmd/oceantv/notifier"
	"github.com/ausocean/cloud/cmd/oceantv/registry"
)

func register(state registry.Named) struct{} {
	err := registry.Register(state)
	if err != nil {
		panic(fmt.Errorf("could not register state: %v", err))
	}
	return struct{}{}
}

func newableWithContext(new func(ctx *broadcastContext) any, args ...interface{}) (any, error) {
	var ctx *broadcastContext
	for _, arg := range args {
		if _ctx, ok := arg.(*broadcastContext); ok {
			ctx = _ctx
			break
		}
	}
	if ctx == nil {
		return nil, errors.New("init args did not contain required broadcast context")
	}
	return new(ctx), nil
}

type hardwareStateMachine struct {
	currentState state
	ctx          *broadcastContext
}

func getHardwareState(ctx *broadcastContext) state {
	// If hardware state is not set, default to hardwareOff.
	// This will happen with fresh configurations.
	if ctx.cfg.HardwareState == "" {
		return newHardwareOff()
	}

	obj, err := registry.Get(ctx.cfg.HardwareState, ctx)
	if err != nil {
		panic(fmt.Sprintf("could not get hardware state from registry: %v", err))
	}

	_state, ok := obj.(state)
	if !ok {
		panic(fmt.Sprintf("could not cast hardware state for %s to state: %v", ctx.cfg.HardwareState, obj))
	}

	err = json.Unmarshal(ctx.cfg.HardwareStateData, &_state)
	if err != nil {
		ctx.log("unexpected error when unmarshaling hardware state data; this could mean we have an unexpected state: %v", err)
	}
	return _state
}

func hardwareStateToString(state state) string {
	return state.(registry.Named).Name()
}

func newHardwareStateMachine(ctx *broadcastContext) *hardwareStateMachine {
	sm := &hardwareStateMachine{getHardwareState(ctx), ctx}
	return sm
}

func (sm *hardwareStateMachine) handleEvent(e event.Event) error {
	switch e.(type) {
	case event.Time:
		sm.handleTimeEvent(e.(event.Time))
	case event.HardwareStartFailed:
		sm.handleHardwareStartFailedEvent(e.(event.HardwareStartFailed))
	case event.HardwareStopFailed:
		sm.handleHardwareStopFailedEvent(e.(event.HardwareStopFailed))
	case event.HardwareStarted:
		sm.handleHardwareStartedEvent(e.(event.HardwareStarted))
	case event.HardwareResetRequest:
		sm.handleHardwareResetRequestEvent(e.(event.HardwareResetRequest))
	case event.HardwareShutdownFailed:
		sm.handleHardwareShutdownFailedEvent(e.(event.HardwareShutdownFailed))
	case event.HardwarePowerOffFailed:
		sm.handleHardwarePowerOffFailedEvent(e.(event.HardwarePowerOffFailed))
	case event.HardwareStopped:
		sm.handleHardwareStoppedEvent(e.(event.HardwareStopped))
	case event.HardwareStartRequest:
		sm.handleHardwareStartRequestEvent(e.(event.HardwareStartRequest))
	case event.HardwareStopRequest:
		sm.handleHardwareStopRequestEvent(e.(event.HardwareStopRequest))
	case event.ControllerFailure:
		sm.handleControllerFailureEvent(e.(event.ControllerFailure))
	case event.LowVoltage:
		sm.handleLowVoltageEvent(e.(event.LowVoltage))
	case event.VoltageRecovered:
		sm.handleVoltageRecoveredEvent(e.(event.VoltageRecovered))
	default:
		// Do nothing.
	}
	return sm.saveHardwareStateToConfig()
}

func (sm *hardwareStateMachine) handleTimeEvent(t event.Time) {
	sm.log("handling time event")
	eventIfStatus := func(e event.Event, status bool) {
		sm.ctx.hardware.PublishEventIfStatus(sm.ctx, e, status, sm.ctx.cfg.CameraMac, sm.ctx.store, sm.log, sm.ctx.bus.Publish)
	}
	switch sm.currentState.(type) {
	case *hardwareStarting:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			sm.ctx.bus.Publish(event.HardwareStartFailed{Err: errors.New("exceed timeout during hardware starting")})
			sm.transition(newHardwareOff())
			return
		}
		eventIfStatus(event.HardwareStarted{}, true)
	case *hardwareStopping:
		sm.currentState.(*hardwareStopping).handleTimeEvent(t)
	case *hardwareRestarting:
		sm.currentState.(*hardwareRestarting).handleTimeEvent(t)
	case *hardwareRecoveringVoltage:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			sm.ctx.bus.Publish(event.HardwareStartFailed{errors.New("voltage recovery timed out")})
			sm.transition(newHardwareOff())
			return
		}

		voltage, err := sm.ctx.hardware.Voltage(sm.ctx)
		if err != nil {
			errWrapped := fmt.Errorf("could not get hardware voltage: %v", err)
			sm.log(errWrapped.Error())
			sm.ctx.bus.Publish(event.InvalidConfiguration{errWrapped})
			return
		}

		// If RequiredStreamingVoltage is not set, default to 24.5.
		if sm.ctx.cfg.RequiredStreamingVoltage == 0 {
			const defaultRequiredStreamingVoltage = 24.5
			sm.log("required streaming voltage is not set, defaulting to %f", defaultRequiredStreamingVoltage)
			try(
				sm.ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.RequiredStreamingVoltage = defaultRequiredStreamingVoltage }),
				"could not save default required streaming voltage to config",
				func(msg string, args ...interface{}) { sm.ctx.logAndNotify(notifier.KindSoftware, msg, args...) },
			)
		}

		if voltage >= sm.ctx.cfg.RequiredStreamingVoltage {
			sm.ctx.bus.Publish(event.VoltageRecovered{})
		}
	default:
		// Do nothing.
	}
}

func (sm *hardwareStateMachine) handleHardwareShutdownFailedEvent(e event.HardwareShutdownFailed) {
	sm.log("handling hardware shutdown failed event")
	switch sm.currentState.(type) {
	case *hardwareStopping:
		sm.currentState.(*hardwareStopping).handleHardwareShutdownFailedEvent(e)
	case *hardwareRestarting:
		sm.currentState.(*hardwareRestarting).handleHardwareShutdownFailedEvent(e)
	default:
		sm.unexpectedEvent(e, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStoppedEvent(e event.HardwareStopped) {
	sm.log("handling hardware stopped event")
	switch sm.currentState.(type) {
	case *hardwareStopping:
		sm.transition(newHardwareOff())
	case *hardwareStarting:
		sm.transition(newHardwareOff())
	case *hardwareRestarting:
		sm.currentState.(*hardwareRestarting).handleHardwareStoppedEvent(e)
	case *hardwareOn:
		sm.transition(newHardwareOff())
	default:
		sm.unexpectedEvent(e, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStopFailedEvent(e event.HardwareStopFailed) {
	switch sm.currentState.(type) {
	case *hardwareStopping, *hardwareRestarting:
		sm.transition(newHardwareFailure(sm.ctx, e))
	}
}

func (sm *hardwareStateMachine) handleHardwarePowerOffFailedEvent(e event.HardwarePowerOffFailed) {
	switch sm.currentState.(type) {
	case *hardwareStopping:
		sm.currentState.(*hardwareStopping).handleHardwarePowerOffFailedEvent(e)
	default:
		sm.unexpectedEvent(e, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStartFailedEvent(e event.HardwareStartFailed) {
	switch sm.currentState.(type) {
	case *hardwareStarting, *hardwareRestarting:
		sm.log("handling hardware start failed event")
		sm.transition(newHardwareFailure(sm.ctx, e))
	}
}

func (sm *hardwareStateMachine) handleHardwareStartedEvent(e event.HardwareStarted) {
	sm.log("handling hardware started event")
	switch sm.currentState.(type) {
	case *hardwareStarting:
		sm.transition(newHardwareOn())
	case *hardwareRestarting:
		sm.transition(newHardwareOn())
	default:
		sm.unexpectedEvent(e, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStartRequestEvent(e event.HardwareStartRequest) {
	sm.log("handling hardware start request event")
	switch sm.currentState.(type) {
	case *hardwareOff, *hardwareRestarting:
		sm.transition(newHardwareStarting(sm.ctx))
	case *hardwareStarting:
		sm.ctx.hardware.PublishEventIfStatus(sm.ctx, event.HardwareStarted{}, true, sm.ctx.cfg.CameraMac, sm.ctx.store, sm.log, sm.ctx.bus.Publish)
	case *hardwareStopping:
		// Ignore and log.
		sm.log("ignoring hardware start request event since hardware is still stopping")
	case *hardwareOn:
		// Ignore.
	default:
		sm.unexpectedEvent(e, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStopRequestEvent(e event.HardwareStopRequest) {
	sm.log("handling hardware stop request event")
	switch sm.currentState.(type) {
	case *hardwareOn, *hardwareStarting, *hardwareRestarting:
		sm.transition(newHardwareStopping(sm.ctx))
	case *hardwareOff, *hardwareStopping:
		// Ignore.
	default:
		sm.unexpectedEvent(e, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareResetRequestEvent(e event.HardwareResetRequest) {
	sm.log("handling hardware reset request event")
	switch sm.currentState.(type) {
	case *hardwareOn:
		sm.transition(newHardwareRestarting(sm.ctx))
	case *hardwareOff:
		sm.transition(newHardwareStarting(sm.ctx))
	case *hardwareRestarting, *hardwareStarting, *hardwareStopping:
		// Ignore.
	default:
		sm.unexpectedEvent(e, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleControllerFailureEvent(e event.ControllerFailure) {
	sm.transition(newHardwareFailure(sm.ctx, e))
}

func (sm *hardwareStateMachine) handleLowVoltageEvent(e event.LowVoltage) {
	sm.log("handling low voltage event")
	switch sm.currentState.(type) {
	case *hardwareStarting:
		sm.transition(newHardwareRecoveringVoltage(sm.ctx))
	case *hardwareOn, *hardwareRestarting:
		sm.transition(newHardwareStopping(sm.ctx))
	case *hardwareOff, *hardwareStopping:
		// Ignore.
	default:
		sm.unexpectedEvent(e, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleVoltageRecoveredEvent(e event.VoltageRecovered) {
	sm.log("handling voltage recovered event")
	switch sm.currentState.(type) {
	case *hardwareRecoveringVoltage:
		sm.transition(newHardwareStarting(sm.ctx))
	default:
		sm.unexpectedEvent(e, sm.currentState)
	}
}

func (sm *hardwareStateMachine) transition(newState state) {
	err := sm.saveHardwareStateToConfig()
	if err != nil {
		sm.log("could not update hardware state in config to transition: %v", err)
		return
	}
	sm.log("transitioning from %s to %s", stateToString(sm.currentState), stateToString(newState))
	sm.currentState.exit()
	sm.currentState = newState
	sm.currentState.enter()
}

func (sm *hardwareStateMachine) unexpectedEvent(e event.Event, state state) {
	sm.log("unexpected event %s in current state %s", e.String(), stateToString(state))
}

func (sm *hardwareStateMachine) log(format string, args ...interface{}) {
	sm.ctx.log("(hardware sm) "+format, args...)
}

func (sm *hardwareStateMachine) saveHardwareStateToConfig() error {
	sm.log("saving hardware state to config: %v", hardwareStateToString(sm.currentState))
	hardwareState := hardwareStateToString(sm.currentState)
	hardwareStateData, err := json.Marshal(sm.currentState)
	if err != nil {
		return fmt.Errorf("could not marshal hardware state data: %v", err)
	}
	return sm.ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.HardwareState = hardwareState; _cfg.HardwareStateData = hardwareStateData })
}
