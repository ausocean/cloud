package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/model"
)

type hardwareRestarting struct {
	*broadcastContext `json:"-"`
}

func newHardwareRestarting(ctx *broadcastContext) *hardwareRestarting {
	return &hardwareRestarting{ctx}
}

func (s *hardwareRestarting) enter() {
	s.camera.stop(s.broadcastContext)
}
func (s *hardwareRestarting) exit() {}

type hardwareStarting struct {
	*broadcastContext `json:"-"`
	LastEntered       time.Time
}

func newHardwareStarting(ctx *broadcastContext) *hardwareStarting {
	return &hardwareStarting{broadcastContext: ctx}
}
func (s *hardwareStarting) enter() {
	s.LastEntered = time.Now()
	// A MAC of 0 indicates it is invalid or unset, proceed with starting the camera.
	if s.cfg.ControllerMAC == 0 {
		s.camera.start(s.broadcastContext)
		return
	}

	voltage, err := s.camera.voltage(s.broadcastContext)
	if err != nil {
		msg := fmt.Sprintf("could not get hardware voltage: %v", err)
		s.log(msg)
		s.bus.publish(invalidConfigurationEvent{msg})
		return
	}

	alarmVoltage, err := s.camera.alarmVoltage(s.broadcastContext)
	if err != nil {
		msg := fmt.Sprintf("could not get alarm voltage: %v", err)
		s.log(msg)
		s.bus.publish(invalidConfigurationEvent{msg})
		return
	}

	controllerIsOn, err := s.camera.isUp(s.broadcastContext)
	if err != nil {
		msg := fmt.Sprintf("could not get controller status: %v", err)
		s.log(msg)
		s.bus.publish(invalidConfigurationEvent{msg})
		return
	}

	if voltage <= alarmVoltage {
		if controllerIsOn {
			s.log("voltage less than alarm voltage but controller is on, something is configured incorrectly")
			s.bus.publish(invalidConfigurationEvent{"voltage less than alarm voltage but controller is on"})
			return
		}
		s.log("controller voltage is low, waiting for recovery before starting")
		s.bus.publish(lowVoltageEvent{})
		return
	}

	// Not below alarm voltage, but controller is not responding.
	// This is a critical failure.
	if !controllerIsOn {
		s.log("controller not responding above alarm voltage")
		s.bus.publish(controllerFailureEvent{})
		return
	}

	// Controller is reporting, but we're not above streaming voltage. Need
	// to wait for recovery.
	if voltage < s.cfg.RequiredStreamingVoltage {
		s.log("controller voltage is below required streaming voltage, waiting for recovery before starting")
		s.bus.publish(lowVoltageEvent{})
		return
	}

	// Controller is reporting and we're above streaming voltage, let's power
	// on the camera.
	s.camera.start(s.broadcastContext)
}

func (s *hardwareStarting) exit() {}

func (s *hardwareStarting) timedOut(t time.Time) bool {
	const timeout = 5 * time.Minute
	if t.Sub(s.LastEntered) > timeout {
		s.log("timed out starting hardware, last entered: %v, time now: %v", s.LastEntered, t)
		return true
	}
	return false
}

func (s *hardwareStarting) reset(time.Duration) {}

type hardwareRecoveringVoltage struct {
	stateFields
	stateWithTimeoutFields
}

func newHardwareRecoveringVoltage(ctx *broadcastContext) *hardwareRecoveringVoltage {
	s := newStateWithTimeoutFields(ctx)
	s.Timeout = time.Duration(sanatisedVoltageRecoveryTimeout(ctx)) * time.Hour
	return &hardwareRecoveringVoltage{
		stateWithTimeoutFields: s,
	}
}

func (s *hardwareRecoveringVoltage) enter() {
	s.LastEntered = time.Now()
}

func sanatisedVoltageRecoveryTimeout(ctx *broadcastContext) int {
	// If VoltageRecoveryTimeout is not set, default to 4 hours.
	if ctx.cfg.VoltageRecoveryTimeout == 0 {
		const defaultRechargeTimeoutHours = 4
		ctx.log("recharge timeout hours is not set, defaulting to %d", defaultRechargeTimeoutHours)
		try(
			ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.VoltageRecoveryTimeout = defaultRechargeTimeoutHours }),
			"could not save default recharge timeout hours to config",
			func(msg string, args ...interface{}) { ctx.logAndNotify(broadcastSoftware, msg, args...) },
		)
	}
	return ctx.cfg.VoltageRecoveryTimeout
}

type hardwareStopping struct {
	*broadcastContext `json:"-"`
}

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
	case "hardwareRecoveringVoltage":
		_state = newHardwareRecoveringVoltage(ctx)
	default:
		panic(fmt.Sprintf("invalid hardware state: %s", ctx.cfg.HardwareState))
	}

	err := json.Unmarshal(ctx.cfg.HardwareStateData, &_state)
	if err != nil {
		ctx.log("unexpected error when unmarshaling hardware state data; this could mean we have an unexpected state: %v", err)
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
		sm.ctx.camera.publishEventIfStatus(e, status, sm.ctx.cfg.CameraMac, sm.ctx.store, sm.log, sm.ctx.bus.publish)
	}
	switch sm.currentState.(type) {
	case *hardwareStarting:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			sm.ctx.bus.publish(hardwareStartFailedEvent{})
			sm.transition(newHardwareOff())
			return
		}
		eventIfStatus(hardwareStartedEvent{}, true)
	case *hardwareStopping:
		eventIfStatus(hardwareStoppedEvent{}, false)
	case *hardwareRestarting:
		eventIfStatus(hardwareStartRequestEvent{}, false)
	case *hardwareRecoveringVoltage:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			sm.ctx.logAndNotify(broadcastHardware, "voltage recovery timed out")
			sm.ctx.bus.publish(hardwareStartFailedEvent{})
			sm.transition(newHardwareOff())
			return
		}

		voltage, err := sm.ctx.camera.voltage(sm.ctx)
		if err != nil {
			msg := fmt.Sprintf("could not get hardware voltage: %v", err)
			sm.log(msg)
			sm.ctx.bus.publish(invalidConfigurationEvent{msg})
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
	case *hardwareOff, *hardwareRestarting:
		sm.transition(newHardwareStarting(sm.ctx))
	case *hardwareStarting:
		sm.ctx.camera.publishEventIfStatus(hardwareStartedEvent{}, true, sm.ctx.cfg.CameraMac, sm.ctx.store, sm.log, sm.ctx.bus.publish)
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
	sm.log("handling controller failure event")
	switch sm.currentState.(type) {
	case *hardwareOn, *hardwareRestarting, *hardwareStopping, *hardwareStarting:
		sm.transition(newHardwareOff())
	default:
		sm.unexpectedEvent(event, sm.currentState)
	}
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
	isUp(ctx *broadcastContext) (bool, error)
	start(ctx *broadcastContext)
	stop(ctx *broadcastContext)
	publishEventIfStatus(event event, status bool, mac int64, store Store, log func(format string, args ...interface{}), publish func(event event))
}

type revidCameraClient struct{}

func (c *revidCameraClient) voltage(ctx *broadcastContext) (float64, error) {
	// Get battery voltage sensor, which we'll use to get scale factor and current voltage value.
	const batteryVoltagePin = "A0"
	sensor, err := model.GetSensorV2(context.Background(), ctx.store, ctx.cfg.ControllerMAC, batteryVoltagePin)
	if err != nil {
		return 0, fmt.Errorf("could not get battery voltage sensor: %v", err)
	}

	// Get current battery voltage.
	voltage, err := model.GetSensorValue(context.Background(), ctx.store, sensor)
	if err != nil {
		return 0, fmt.Errorf("could not get current battery voltage: %v", err)
	}
	return voltage, nil
}

func (c *revidCameraClient) alarmVoltage(ctx *broadcastContext) (float64, error) {
	// Get AlarmVoltage variable; if the voltage is above this we expect the controller to be on.
	// If the voltage is below this, we expect the controller to be off.
	controllerMACHex := (&model.Device{Mac: ctx.cfg.ControllerMAC}).Hex()
	alarmVoltageVar, err := model.GetVariable(context.Background(), ctx.store, ctx.cfg.SKey, controllerMACHex+".AlarmVoltage")
	if err != nil {
		return 0, fmt.Errorf("could not get alarm voltage variable: %v", err)
	}
	ctx.log("got AlarmVoltage for %s: %s", controllerMACHex, alarmVoltageVar.Value)

	uncalibratedAlarmVoltage, err := strconv.Atoi(alarmVoltageVar.Value)
	if err != nil {
		return 0, fmt.Errorf("could not convert uncalibrated alarm voltage from string: %v", err)
	}

	// Get battery voltage sensor, which we'll use to get scale factor and current voltage value.
	const batteryVoltagePin = "A0"
	sensor, err := model.GetSensorV2(context.Background(), ctx.store, ctx.cfg.ControllerMAC, batteryVoltagePin)
	if err != nil {
		return 0, fmt.Errorf("could not get battery voltage sensor: %v", err)
	}

	// Transform the alarm voltage to the actual voltage.
	alarmVoltage, err := sensor.Transform(float64(uncalibratedAlarmVoltage))
	if err != nil {
		return 0, fmt.Errorf("could not transform alarm voltage: %v", err)
	}

	return alarmVoltage, nil
}

func (c *revidCameraClient) isUp(ctx *broadcastContext) (bool, error) {
	controllerIsOn, err := model.DeviceIsUp(context.Background(), ctx.store, model.MacDecode(ctx.cfg.ControllerMAC))
	if err != nil {
		return false, fmt.Errorf("could not get controller status: %v", err)
	}
	return controllerIsOn, nil
}

func (c *revidCameraClient) start(ctx *broadcastContext) {
	err := extStart(context.Background(), ctx.cfg, ctx.log)
	if err != nil {
		ctx.log("could not start external hardware: %v", err)
		ctx.bus.publish(hardwareStartFailedEvent{})
		return
	}
}

func (c *revidCameraClient) stop(ctx *broadcastContext) {
	err := extStop(context.Background(), ctx.cfg, ctx.log)
	if err != nil {
		ctx.log("could not stop external hardware: %v", err)
		ctx.bus.publish(hardwareStopFailedEvent{})
		return
	}
}

func (c *revidCameraClient) publishEventIfStatus(event event, status bool, mac int64, store Store, log func(string, ...interface{}), publish func(event event)) {
	if mac == 0 {
		log("camera is not set in configuration")
		publish(invalidConfigurationEvent{"camera mac is empty"})
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

func (sm *hardwareStateMachine) saveHardwareStateToConfig() error {
	sm.log("saving hardware state to config: %v", hardwareStateToString(sm.currentState))
	hardwareState := hardwareStateToString(sm.currentState)
	hardwareStateData, err := json.Marshal(sm.currentState)
	if err != nil {
		return fmt.Errorf("could not marshal hardware state data: %v", err)
	}
	return sm.ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.HardwareState = hardwareState; _cfg.HardwareStateData = hardwareStateData })
}
