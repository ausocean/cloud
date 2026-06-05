package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/ausocean/cloud/cmd/oceantv/registry"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/notify"
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

type hardwareShutdownFailedEvent struct{ error }

var _ = registerEvent(hardwareShutdownFailedEvent{})

func (e hardwareShutdownFailedEvent) String() string { return "hardwareShutdownFailedEvent" }
func (e hardwareShutdownFailedEvent) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}
func (e hardwareShutdownFailedEvent) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return hardwareShutdownFailedEvent{err}, nil
}

// Kind implements the errorEvent interface.
func (e hardwareShutdownFailedEvent) Kind() notify.Kind {
	if errEvent, ok := e.error.(errorEvent); ok {
		return errEvent.Kind()
	}

	if unwrapped := unwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcastHardware
}

func (e hardwareShutdownFailedEvent) Unwrap() error { return e.error }

func (e hardwareShutdownFailedEvent) Is(target error) bool {
	if _, ok := target.(hardwareShutdownFailedEvent); ok {
		return true
	}
	return errors.Is(e.error, target)
}

type hardwareShutdownEvent struct{}

var _ = registerEvent(hardwareShutdownEvent{})

func (e hardwareShutdownEvent) String() string { return "hardwareShutdownEvent" }

type hardwarePowerOffFailedEvent struct{ error }

var _ = registerEvent(hardwarePowerOffFailedEvent{})

func (e hardwarePowerOffFailedEvent) String() string { return "hardwarePowerOffFailedEvent" }
func (e hardwarePowerOffFailedEvent) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}
func (e hardwarePowerOffFailedEvent) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return hardwarePowerOffFailedEvent{err}, nil
}

// Kind implements the errorEvent interface.
func (e hardwarePowerOffFailedEvent) Kind() notify.Kind {
	if errEvent, ok := e.error.(errorEvent); ok {
		return errEvent.Kind()
	}

	if unwrapped := unwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcastHardware
}

func (e hardwarePowerOffFailedEvent) Unwrap() error { return e.error }

func (e hardwarePowerOffFailedEvent) Is(target error) bool {
	if _, ok := target.(hardwarePowerOffFailedEvent); ok {
		return true
	}
	return errors.Is(e.error, target)
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
	case hardwareShutdownFailedEvent:
		sm.handleHardwareShutdownFailedEvent(event.(hardwareShutdownFailedEvent))
	case hardwarePowerOffFailedEvent:
		sm.handleHardwarePowerOffFailedEvent(event.(hardwarePowerOffFailedEvent))
	case hardwareStoppedEvent:
		sm.handleHardwareStoppedEvent(event.(hardwareStoppedEvent))
	case hardwareStartRequestEvent:
		sm.handleHardwareStartRequestEvent(event.(hardwareStartRequestEvent))
	case hardwareStopRequestEvent:
		sm.handleHardwareStopRequestEvent(event.(hardwareStopRequestEvent))
	case controllerFailureEvent:
		sm.handleControllerFailureEvent(event.(controllerFailureEvent))
	case lowVoltageEvent:
		sm.handleLowVoltageEvent(event.(lowVoltageEvent))
	case voltageRecoveredEvent:
		sm.handleVoltageRecoveredEvent(event.(voltageRecoveredEvent))
	default:
		// Do nothing.
	}
	return sm.saveHardwareStateToConfig()
}

func (sm *hardwareStateMachine) handleTimeEvent(t timeEvent) {
	sm.log("handling time event")
	eventIfStatus := func(e event, status bool) {
		sm.ctx.hardware.publishEventIfStatus(sm.ctx, e, status, sm.ctx.cfg.CameraMac, sm.ctx.store, sm.log, sm.ctx.bus.publish)
	}
	switch sm.currentState.(type) {
	case *hardwareStarting:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			sm.ctx.bus.publish(hardwareStartFailedEvent{errors.New("exceed timeout during hardware starting")})
			sm.transition(newHardwareOff())
			return
		}
		eventIfStatus(hardwareStartedEvent{}, true)
	case *hardwareStopping:
		sm.currentState.(*hardwareStopping).handleTimeEvent(t)
	case *hardwareRestarting:
		sm.currentState.(*hardwareRestarting).handleTimeEvent(t)
	case *hardwareRecoveringVoltage:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			sm.ctx.bus.publish(hardwareStartFailedEvent{errors.New("voltage recovery timed out")})
			sm.transition(newHardwareOff())
			return
		}

		voltage, err := sm.ctx.hardware.voltage(sm.ctx)
		if err != nil {
			errWrapped := fmt.Errorf("could not get hardware voltage: %v", err)
			sm.log(errWrapped.Error())
			sm.ctx.bus.publish(invalidConfigurationEvent{errWrapped})
			return
		}

		// If RequiredStreamingVoltage is not set, default to 24.5.
		if sm.ctx.cfg.RequiredStreamingVoltage == 0 {
			const defaultRequiredStreamingVoltage = 24.5
			sm.log("required streaming voltage is not set, defaulting to %f", defaultRequiredStreamingVoltage)
			try(
				sm.ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.RequiredStreamingVoltage = defaultRequiredStreamingVoltage }),
				"could not save default required streaming voltage to config",
				func(msg string, args ...interface{}) { sm.ctx.logAndNotify(broadcastSoftware, msg, args...) },
			)
		}

		if voltage >= sm.ctx.cfg.RequiredStreamingVoltage {
			sm.ctx.bus.publish(voltageRecoveredEvent{})
		}
	default:
		// Do nothing.
	}
}

func (sm *hardwareStateMachine) handleHardwareShutdownFailedEvent(event hardwareShutdownFailedEvent) {
	sm.log("handling hardware shutdown failed event")
	switch sm.currentState.(type) {
	case *hardwareStopping:
		sm.currentState.(*hardwareStopping).handleHardwareShutdownFailedEvent(event)
	case *hardwareRestarting:
		sm.currentState.(*hardwareRestarting).handleHardwareShutdownFailedEvent(event)
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStoppedEvent(event hardwareStoppedEvent) {
	sm.log("handling hardware stopped event")
	switch sm.currentState.(type) {
	case *hardwareStopping:
		sm.transition(newHardwareOff())
	case *hardwareStarting:
		sm.transition(newHardwareOff())
	case *hardwareRestarting:
		sm.currentState.(*hardwareRestarting).handleHardwareStoppedEvent(event)
	case *hardwareOn:
		sm.transition(newHardwareOff())
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStopFailedEvent(event hardwareStopFailedEvent) {
	switch sm.currentState.(type) {
	case *hardwareStopping, *hardwareRestarting:
		sm.transition(newHardwareFailure(sm.ctx, event))
	}
}

func (sm *hardwareStateMachine) handleHardwarePowerOffFailedEvent(event hardwarePowerOffFailedEvent) {
	switch sm.currentState.(type) {
	case *hardwareStopping:
		sm.currentState.(*hardwareStopping).handleHardwarePowerOffFailedEvent(event)
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStartFailedEvent(event hardwareStartFailedEvent) {
	switch sm.currentState.(type) {
	case *hardwareStarting, *hardwareRestarting:
		sm.log("handling hardware start failed event")
		sm.transition(newHardwareFailure(sm.ctx, event))
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
	case *hardwareOff, *hardwareRestarting:
		sm.transition(newHardwareStarting(sm.ctx))
	case *hardwareStarting:
		sm.ctx.hardware.publishEventIfStatus(sm.ctx, hardwareStartedEvent{}, true, sm.ctx.cfg.CameraMac, sm.ctx.store, sm.log, sm.ctx.bus.publish)
	case *hardwareStopping:
		// Ignore and log.
		sm.log("ignoring hardware start request event since hardware is still stopping")
	case *hardwareOn:
		// Ignore.
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleHardwareStopRequestEvent(event hardwareStopRequestEvent) {
	sm.log("handling hardware stop request event")
	switch sm.currentState.(type) {
	case *hardwareOn, *hardwareStarting, *hardwareRestarting:
		sm.transition(newHardwareStopping(sm.ctx))
	case *hardwareOff, *hardwareStopping:
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

func (sm *hardwareStateMachine) handleControllerFailureEvent(event controllerFailureEvent) {
	sm.transition(newHardwareFailure(sm.ctx, event))
}

func (sm *hardwareStateMachine) handleLowVoltageEvent(event lowVoltageEvent) {
	sm.log("handling low voltage event")
	switch sm.currentState.(type) {
	case *hardwareStarting:
		sm.transition(newHardwareRecoveringVoltage(sm.ctx))
	case *hardwareOn, *hardwareRestarting:
		sm.transition(newHardwareStopping(sm.ctx))
	case *hardwareOff, *hardwareStopping:
		// Ignore.
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
}

func (sm *hardwareStateMachine) handleVoltageRecoveredEvent(event voltageRecoveredEvent) {
	sm.log("handling voltage recovered event")
	switch sm.currentState.(type) {
	case *hardwareRecoveringVoltage:
		sm.transition(newHardwareStarting(sm.ctx))
	default:
		sm.unexpectedEvent(event, sm.currentState)
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

func (sm *hardwareStateMachine) unexpectedEvent(event event, state state) {
	sm.log("unexpected event %s in current state %s", event.String(), stateToString(state))
}

func (sm *hardwareStateMachine) log(format string, args ...interface{}) {
	sm.ctx.log("(hardware sm) "+format, args...)
}

type hardwareManager interface {
	voltage(ctx *broadcastContext) (float64, error)
	alarmVoltage(ctx *broadcastContext) (float64, error)
	isUp(ctx *broadcastContext, mac string) (bool, error)
	start(ctx *broadcastContext)
	shutdown(ctx *broadcastContext)
	stop(ctx *broadcastContext)
	publishEventIfStatus(ctx *broadcastContext, event event, status bool, mac int64, store Store, log func(format string, args ...interface{}), publish func(event event))
	error(ctx *broadcastContext) (error, error)
}

type revidCameraClient struct{}

type ControllerError string

const (
	None            ControllerError = ""
	LowVoltageAlarm ControllerError = "LowVoltage"
)

func (e ControllerError) Error() string {
	return string(e)
}

func (e ControllerError) Is(target error) bool {
	if target == nil {
		return false
	}
	if t, ok := target.(ControllerError); ok {
		return e == t
	}
	return false
}

func (c *revidCameraClient) voltage(ctx *broadcastContext) (float64, error) {
	// Get battery voltage sensor, which we'll use to get scale factor and current voltage value.
	sensor, err := model.GetSensorV2(context.Background(), ctx.store, ctx.cfg.ControllerMAC, ctx.cfg.BatteryVoltagePin)
	if err != nil {
		return 0, fmt.Errorf("could not get battery voltage sensor (%s.%s): %w", model.MacDecode(ctx.cfg.ControllerMAC), ctx.cfg.BatteryVoltagePin, err)
	}

	// Get current battery voltage.
	voltage, err := model.GetSensorValue(context.Background(), ctx.store, sensor)
	switch {
	case errors.Is(err, datastore.ErrNoSuchEntity):
		// We'll get this if the controller is off from low voltage, so just
		// assume we have alarm voltage.
		alarmVoltage, err := c.alarmVoltage(ctx)
		if err != nil {
			return 0, fmt.Errorf("could not get alarm voltage: %w", err)
		}
		return alarmVoltage, nil
	case err != nil:
		return 0, fmt.Errorf("could not get current battery voltage: %w", err)
	}

	return voltage, nil
}

func (c *revidCameraClient) alarmVoltage(ctx *broadcastContext) (float64, error) {
	// Get AlarmVoltage variable; if the voltage is above this we expect the controller to be on.
	// If the voltage is below this, we expect the controller to be off.
	controllerMACHex := (&model.Device{Mac: ctx.cfg.ControllerMAC}).Hex()
	alarmVoltageVar, err := model.GetVariable(context.Background(), ctx.store, ctx.cfg.SKey, controllerMACHex+".AlarmVoltage")
	if err != nil {
		return 0, fmt.Errorf("could not get alarm voltage variable: %w", err)
	}
	ctx.log("got AlarmVoltage for %s: %s", controllerMACHex, alarmVoltageVar.Value)

	uncalibratedAlarmVoltage, err := strconv.Atoi(alarmVoltageVar.Value)
	if err != nil {
		return 0, fmt.Errorf("could not convert uncalibrated alarm voltage from string: %w", err)
	}

	// Get battery voltage sensor, which we'll use to get scale factor and current voltage value.
	batteryVoltagePin := ctx.cfg.BatteryVoltagePin
	if batteryVoltagePin == "" {
		const defaultBatteryVoltagePin = "A4"
		batteryVoltagePin = defaultBatteryVoltagePin
	}
	sensor, err := model.GetSensorV2(context.Background(), ctx.store, ctx.cfg.ControllerMAC, batteryVoltagePin)
	if err != nil {
		return 0, fmt.Errorf("could not get battery voltage sensor: %w", err)
	}

	// Transform the alarm voltage to the actual voltage.
	alarmVoltage, err := sensor.Transform(float64(uncalibratedAlarmVoltage))
	if err != nil {
		return 0, fmt.Errorf("could not transform alarm voltage: %w", err)
	}

	return alarmVoltage, nil
}

func (c *revidCameraClient) isUp(ctx *broadcastContext, mac string) (bool, error) {
	deviceIsUp, err := model.DeviceIsUp(context.Background(), ctx.store, mac)
	if err != nil {
		return false, fmt.Errorf("could not get controller status: %w", err)
	}
	return deviceIsUp, nil
}

func (c *revidCameraClient) start(ctx *broadcastContext) {
	err := extStart(context.Background(), ctx.cfg, ctx.log)
	if err != nil {
		ctx.log("could not start external hardware: %v", err)
		ctx.bus.publish(hardwareStartFailedEvent{fmt.Errorf("external hardware start actions failed: %w", err)})
		return
	}
}

func (c *revidCameraClient) shutdown(ctx *broadcastContext) {
	err := extShutdown(context.Background(), ctx.cfg, ctx.log)
	if err != nil {
		ctx.bus.publish(hardwareShutdownFailedEvent{fmt.Errorf("could not perform shutdown actions: %w", err)})
		return
	}
}

func (c *revidCameraClient) stop(ctx *broadcastContext) {
	err := extStop(context.Background(), ctx.cfg, ctx.log)
	if err != nil {
		ctx.log("could not stop external hardware: %v", err)
		ctx.bus.publish(hardwareStopFailedEvent{fmt.Errorf("could not perform stop actions: %w", err)})
		return
	}
}

func (c *revidCameraClient) publishEventIfStatus(ctx *broadcastContext, event event, status bool, mac int64, store Store, log func(string, ...interface{}), publish func(event event)) {
	if mac == 0 {
		publish(invalidConfigurationEvent{errors.New("camera mac is empty")})
		return
	}
	log("checking status of device with mac: %d", mac)
	alive, err := model.DeviceIsUp(context.Background(), store, model.MacDecode(mac))
	if err != nil {
		log("could not get device status: %v", err)
		return
	}
	log("status from DeviceIsUp check: %v", alive)
	if alive == status {
		publish(event)
		return
	}
}

func (c *revidCameraClient) error(ctx *broadcastContext) (error, error) {
	controllerMACHex := (&model.Device{Mac: ctx.cfg.ControllerMAC}).Hex()
	devErr, err := model.GetVariable(context.Background(), ctx.store, ctx.cfg.SKey, controllerMACHex+".error")
	if err != nil {
		return nil, fmt.Errorf("could not get controller error variable: %w", err)
	}
	return ControllerError(devErr.Value), nil
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
