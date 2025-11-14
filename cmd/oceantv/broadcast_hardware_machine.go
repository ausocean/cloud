package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/registry"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/notify"
	"github.com/ausocean/openfish/datastore"
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

type hardwareRestarting struct {
	stateWithTimeoutFields
	Substate state
}

var _ = register(hardwareRestarting{})

func (s hardwareRestarting) Name() string { return "hardwareRestarting" }

// New implements registry.Newable for creating a fresh value of
// hardwareRestarting from an existing value.
func (s hardwareRestarting) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareRestarting(ctx) }, args...)
}

func newHardwareRestarting(ctx *broadcastContext) *hardwareRestarting {
	return &hardwareRestarting{newStateWithTimeoutFields(ctx), nil}
}

// For Marshaling/Unmarshaling.
type hardwareRestartingStateWrapper struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (s *hardwareRestarting) MarshalJSON() ([]byte, error) {
	substateType := ""
	substateData := []byte("null")

	if s.Substate != nil {
		substateType = s.Substate.(registry.Named).Name()
		data, err := json.Marshal(s.Substate)
		if err != nil {
			return nil, fmt.Errorf("could not marshal substate %s in hardwareRestarting: %w", substateType, err)
		}
		substateData = data
	}

	alias := struct {
		StateWithTimeoutFields stateWithTimeoutFields         `json:",inline"`
		Substate               hardwareRestartingStateWrapper `json:"substate"`
	}{
		StateWithTimeoutFields: s.stateWithTimeoutFields,
		Substate: hardwareRestartingStateWrapper{
			Type: substateType,
			Data: substateData,
		},
	}

	data, err := json.Marshal(alias)
	if err != nil {
		return nil, fmt.Errorf("could not marshal alias in hardwareRestarting: %w", err)
	}

	return data, nil
}

func (s *hardwareRestarting) UnmarshalJSON(data []byte) error {
	if s.broadcastContext == nil {
		return errors.New("hardwareRestarting broadcastContext is nil")
	}

	alias := struct {
		StateWithTimeoutFields stateWithTimeoutFields         `json:",inline"`
		Substate               hardwareRestartingStateWrapper `json:"substate"`
	}{StateWithTimeoutFields: stateWithTimeoutFields{broadcastContext: s.broadcastContext}}

	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("could not unmarshal alias in hardwareRestarting: %w", err)
	}

	s.stateWithTimeoutFields = alias.StateWithTimeoutFields

	// Unmarshal substate.
	if alias.Substate.Type != "" {
		substate, err := registry.Get(alias.Substate.Type, s.broadcastContext)
		if err != nil {
			return fmt.Errorf("could not get substate from registry for type %s in hardwareRestarting: %w", substate, err)
		}

		_substate, ok := substate.(state)
		if !ok {
			panic(fmt.Sprintf("could not assert substate that should be %s in hardwareRestarting", alias.Substate.Type))
		}

		if err := json.Unmarshal(alias.Substate.Data, _substate); err != nil {
			return fmt.Errorf("could not unmarshal data for substate %s in hardwareRestarting: %w", alias.Substate.Type, err)
		}

		s.Substate = _substate
	}

	return nil
}

func (s *hardwareRestarting) enter() {
	s.LastEntered = time.Now()
	s.Substate = newHardwareStopping(s.broadcastContext)
	s.Substate.enter()
}
func (s *hardwareRestarting) exit() {}
func (s *hardwareRestarting) transition() {
	switch s.Substate.(type) {
	case *hardwareStopping:
		s.log("(hardwareRestarting) transitioning from substate hardwareStopping to hardwareStarting")
		s.Substate.exit()
		s.Substate = newHardwareStarting(s.broadcastContext)
		s.Substate.enter()
	default:
		panic("hardwareRestarting: unexpected transition")
	}
}

func (s *hardwareRestarting) handleTimeEvent(t timeEvent) {
	switch s.Substate.(type) {
	case *hardwareStopping:
		withTimeout := s.Substate.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			s.bus.publish(hardwareStopFailedEvent{errors.New("hardware stop timed out")})
			return
		}

		s.Substate.(*hardwareStopping).handleTimeEvent(t)
	case *hardwareStarting:
		withTimeout := s.Substate.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			s.bus.publish(hardwareStartFailedEvent{errors.New("exceeded starting timeout during hardware restart")})
			return
		}

		// If the camera is reporting then the start has completed.
		if s.cameraIsReporting() {
			s.bus.publish(hardwareStartedEvent{})
			return
		}
	default:
		// This is unexpected and probably means we haven't saved a substate properly.
		// So perform a notify log and default to a sensible state.
		s.logAndNotify(broadcastSoftware, "unexpected substate in hardwareRestarting: %v, re-entering state to initialise substate", s.Substate)
		s.enter()
	}
}

func (s *hardwareRestarting) handleHardwareStoppedEvent(event hardwareStoppedEvent) {
	s.log("handling hardware stopped event")
	switch s.Substate.(type) {
	case *hardwareStopping:
		s.transition()
	default:
		// For any other state ignore.
	}
}

func (s *hardwareRestarting) handleHardwareShutdownFailedEvent(event hardwareShutdownFailedEvent) {
	switch s.Substate.(type) {
	case *hardwareStopping:
		s.Substate.(*hardwareStopping).handleHardwareShutdownFailedEvent(event)
	default:
		// Ignore.
	}
}

func (s *hardwareRestarting) cameraIsReporting() bool {
	up, err := s.camera.isUp(s.broadcastContext, model.MacDecode(s.cfg.CameraMac))
	if err != nil {
		s.bus.publish(invalidConfigurationEvent{fmt.Errorf("could not get camera reporting status: %w", err)})
		return false
	}
	return up
}

type hardwareStarting struct {
	stateWithTimeoutFields
	*broadcastContext `json:"-"`
}

var _ = register(hardwareStarting{})

func (s hardwareStarting) Name() string { return "hardwareStarting" }

// New implements registry.Newable for creating a fresh value of
// hardwareStarting from an existing value.
func (s hardwareStarting) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareStarting(ctx) }, args...)
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

	// The first check for any known hardware error states.
	hwErr, err := s.camera.error(s.broadcastContext)
	if err != nil {
		errWrapped := fmt.Errorf("could not get hardware error state: %w", err)
		s.log(errWrapped.Error())
		// NOTE here we could do this, however it's not certain that all ESPs
		// will have the latest firmware that supports this, and so it's not
		// necessarily a showstopper.
		// s.bus.publish(invalidConfigurationEvent{errWrapped})
		// return
	}

	switch {
	case errors.Is(hwErr, LowVoltageAlarm):
		s.log("controller voltage is low, waiting for recovery before starting")
		s.bus.publish(lowVoltageEvent{})
		return
	case errors.Is(hwErr, None):
		// Continue other checks, this is good.
	case hwErr != nil:
		errWrapped := fmt.Errorf("unhandled controller hardware error: %w", hwErr)
		s.log(errWrapped.Error())
		s.bus.publish(invalidConfigurationEvent{errWrapped})
		return
	default:
		// This means we failed to get hwErr, which at this stage just means
		// we have a controller that doesn't have the latest firmware.
	}

	voltage, err := s.camera.voltage(s.broadcastContext)
	if err != nil {
		errWrapped := fmt.Errorf("could not get hardware voltage: %w", err)
		s.log(errWrapped.Error())
		s.bus.publish(invalidConfigurationEvent{errWrapped})
		return
	}

	alarmVoltage, err := s.camera.alarmVoltage(s.broadcastContext)
	if err != nil {
		errWrapped := fmt.Errorf("could not get alarm voltage: %w", err)
		s.log(errWrapped.Error())
		s.bus.publish(invalidConfigurationEvent{errWrapped})
		return
	}

	controllerIsOn, err := s.camera.isUp(s.broadcastContext, model.MacDecode(s.cfg.ControllerMAC))
	if err != nil {
		errWrapped := fmt.Errorf("could not get controller status: %w", err)
		s.log(errWrapped.Error())
		s.bus.publish(invalidConfigurationEvent{errWrapped})
		return
	}

	if voltage <= alarmVoltage {
		if controllerIsOn {
			s.log("voltage less than alarm voltage but controller is on, something is configured incorrectly")
			s.bus.publish(invalidConfigurationEvent{errors.New("voltage less than alarm voltage but controller is on")})
			return
		}
		s.log("controller voltage is low, waiting for recovery before starting")
		s.bus.publish(lowVoltageEvent{})
		return
	}

	// Not below alarm voltage, but controller is not responding.
	// This is a critical failure.
	if !controllerIsOn {
		s.bus.publish(controllerFailureEvent{errors.New("controller not responding above alarm voltage")})
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

var _ = register(hardwareRecoveringVoltage{})

func (s hardwareRecoveringVoltage) Name() string { return "hardwareRecoveringVoltage" }

func (s hardwareRecoveringVoltage) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareRecoveringVoltage(ctx) }, args...)
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

type hardwareShuttingDown struct {
	stateWithTimeoutFields
}

var _ = register(hardwareShuttingDown{})

func (s hardwareShuttingDown) Name() string { return "hardwareShuttingDown" }

// New implements registry.Newable for creating a fresh value of
// hardwareShuttingDown from an existing value.
func (s hardwareShuttingDown) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareShuttingDown(ctx) }, args...)
}

func newHardwareShuttingDown(ctx *broadcastContext) *hardwareShuttingDown {
	return &hardwareShuttingDown{stateWithTimeoutFields: newStateWithTimeoutFields(ctx)}
}
func (s *hardwareShuttingDown) enter() {
	s.LastEntered = time.Now()
	s.camera.shutdown(s.broadcastContext)
}
func (s *hardwareShuttingDown) exit() {}

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

type hardwarePoweringOff struct {
	stateWithTimeoutFields
}

var _ = register(hardwarePoweringOff{})

func (s hardwarePoweringOff) Name() string { return "hardwarePoweringOff" }

// New implements registry.Newable for creating a fresh value of
// hardwarePoweringOff from an existing value.
func (s hardwarePoweringOff) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwarePoweringOff(ctx) }, args...)
}

func newHardwarePoweringOff(ctx *broadcastContext) *hardwarePoweringOff {
	return &hardwarePoweringOff{stateWithTimeoutFields: newStateWithTimeoutFields(ctx)}
}
func (s *hardwarePoweringOff) enter() {
	s.LastEntered = time.Now()
	s.camera.stop(s.broadcastContext)
}
func (s *hardwarePoweringOff) exit() {}

type hardwareStopping struct {
	stateWithTimeoutFields
	Substate state
}

var _ = register(hardwareStopping{})

func (s hardwareStopping) Name() string { return "hardwareStopping" }

// New implements registry.Newable for creating a fresh value of
// hardwareStopping from an existing value.
func (s hardwareStopping) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareStopping(ctx) }, args...)
}

func newHardwareStopping(ctx *broadcastContext) *hardwareStopping {
	return &hardwareStopping{newStateWithTimeoutFields(ctx), nil}
}

// For Marshaling/Unmarshaling.
type hardwareStoppingStateWrapper struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (s *hardwareStopping) MarshalJSON() ([]byte, error) {
	substateType := ""
	substateData := []byte("null")

	if s.Substate != nil {
		substateType = s.Substate.(registry.Named).Name()
		data, err := json.Marshal(s.Substate)
		if err != nil {
			return nil, fmt.Errorf("could not marshal substate %s in hardwareStopping: %w", substateType, err)
		}
		substateData = data
	}

	alias := struct {
		StateWithTimeoutFields stateWithTimeoutFields       `json:",inline"`
		Substate               hardwareStoppingStateWrapper `json:"substate"`
	}{
		StateWithTimeoutFields: s.stateWithTimeoutFields,
		Substate: hardwareStoppingStateWrapper{
			Type: substateType,
			Data: substateData,
		},
	}

	data, err := json.Marshal(alias)
	if err != nil {
		return nil, fmt.Errorf("could not marshal alias in hardwareStopping: %w", err)
	}

	return data, nil
}

func (s *hardwareStopping) UnmarshalJSON(data []byte) error {
	if s.broadcastContext == nil {
		return errors.New("hardwareStopping broadcastContext is nil")
	}

	alias := struct {
		StateWithTimeoutFields stateWithTimeoutFields       `json:",inline"`
		Substate               hardwareStoppingStateWrapper `json:"substate"`
	}{StateWithTimeoutFields: stateWithTimeoutFields{broadcastContext: s.broadcastContext}}

	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("could not unmarshal data for alias in hardwareStopping: %w", err)
	}

	s.stateWithTimeoutFields = alias.StateWithTimeoutFields

	// Unmarshal substate.
	if alias.Substate.Type != "" {
		substate, err := registry.Get(alias.Substate.Type, s.broadcastContext)
		if err != nil {
			return fmt.Errorf("could not get substate from registry for type %s in hardwareStopping: %w", substate, err)
		}

		_substate, ok := substate.(state)
		if !ok {
			panic(fmt.Sprintf("could not assert substate that should be %s in hardwareStopping", alias.Substate.Type))
		}

		if err := json.Unmarshal(alias.Substate.Data, _substate); err != nil {
			return fmt.Errorf("could not unmarshal data for substate %s in hardwareStopping: %w", alias.Substate.Type, err)
		}

		s.Substate = _substate
	}

	return nil
}

func (s *hardwareStopping) enter() {
	s.LastEntered = time.Now()
	s.Substate = newHardwareShuttingDown(s.broadcastContext)
	s.Substate.enter()
}
func (s *hardwareStopping) exit() {}

func (s *hardwareStopping) transition() {
	// This should only be called once.
	switch s.Substate.(type) {
	case *hardwareShuttingDown:
		s.log("(hardwareStopping) transitioning from substate hardwareShuttingDown to hardwarePoweringOff")
		s.Substate.exit()
		s.Substate = newHardwarePoweringOff(s.broadcastContext)
		s.Substate.enter()
	default:
		panic("hardwareStopping: unexpected transition")
	}
}

func (s *hardwareStopping) handleTimeEvent(t timeEvent) {
	switch s.Substate.(type) {
	case *hardwareShuttingDown:
		s.log("(hardwareStopping) handling timeEvent in hardwareStopping state: substate is hardwareShuttingDown")
		withTimeout := s.Substate.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			s.bus.publish(hardwareShutdownFailedEvent{errors.New("hardware shutdown timed out")})
			return
		}

		if !s.cameraIsReporting() {
			s.bus.publish(hardwareShutdownEvent{})
			s.transition()
			return
		}
		s.log("(hardwareStopping) camera is still reporting, waiting for shutdown to complete")

	case *hardwarePoweringOff:
		s.log("(hardwareStopping) handling timeEvent in hardwareStopping state: substate is hardwarePoweringOff")
		withTimeout := s.Substate.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			s.bus.publish(hardwarePowerOffFailedEvent{errors.New("hardware power off timed out")})
			return
		}

		if !s.cameraIsReporting() {
			s.bus.publish(hardwareStoppedEvent{})
			return
		}
		s.log("(hardwareStopping) camera is still reporting, waiting for power off to complete")
	default:
		// This is unexpected and probably means we haven't saved a substate properly.
		// So perform a notify log and default to a sensible state.
		s.logAndNotify(broadcastSoftware, "unexpected substate in hardwareStopping: %v, re-entering state to initialise substate", s.Substate)
		s.enter()
	}
}

func (s *hardwareStopping) handleHardwareShutdownFailedEvent(event hardwareShutdownFailedEvent) {
	switch s.Substate.(type) {
	case *hardwareShuttingDown:
		// We want to get notified for failures and misconfigured configs, and log
		// when shutdown is skipped.
		if errors.Is(event, warnSkipShutdown) {
			s.log("skipping shutdown: %v:", event.Error)
		} else if errors.Is(event, errNoShutdownActions) {
			s.logAndNotify(broadcastHardware, "shutdown skipped: %v", event.Error())
		}
		s.transition()
	default:
		// Ignore.
	}
}

func (s *hardwareStopping) handleHardwarePowerOffFailedEvent(event hardwarePowerOffFailedEvent) {
	switch s.Substate.(type) {
	case *hardwarePoweringOff:
		s.bus.publish(hardwareStopFailedEvent{event})
	default:
		// Ignore.
	}
}

func (s *hardwareStopping) cameraIsReporting() bool {
	up, err := s.camera.isUp(s.broadcastContext, model.MacDecode(s.cfg.CameraMac))
	if err != nil {
		s.bus.publish(invalidConfigurationEvent{fmt.Errorf("could not get camera reporting status: %w", err)})
		return false
	}
	return up
}

type hardwareOn struct{}

var _ = register(hardwareOn{})

func (s hardwareOn) Name() string { return "hardwareOn" }

// New implements registry.Newable for creating a fresh value of
// hardwareOn from an existing value.
func (s hardwareOn) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareOn() }, args...)
}

func newHardwareOn() *hardwareOn { return &hardwareOn{} }
func (s *hardwareOn) enter()     {}
func (s *hardwareOn) exit()      {}

type hardwareOff struct{}

var _ = register(hardwareOff{})

func (s hardwareOff) Name() string { return "hardwareOff" }

// New implements registry.Newable for creating a fresh value of
// hardwareOff from an existing value.
func (s hardwareOff) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareOff() }, args...)
}

func newHardwareOff() *hardwareOff { return &hardwareOff{} }
func (s *hardwareOff) enter()      {}
func (s *hardwareOff) exit()       {}

type hardwareFailure struct {
	*broadcastContext `json:"-"`
	err               error
}

var _ = register(hardwareFailure{})

func newHardwareFailure(ctx *broadcastContext, err error) *hardwareFailure {
	return &hardwareFailure{ctx, err}
}

func (s hardwareFailure) Name() string { return "hardwareFailure" }

func (s hardwareFailure) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{ Err string }{Err: s.err.Error()})
}

func (s *hardwareFailure) UnmarshalJSON(data []byte) error {
	aux := struct{ Err string }{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	s.err = errors.New(aux.Err)
	return nil
}

// New implements registry.Newable for creating a fresh value of
// hardwareFailure from an existing value.
func (s hardwareFailure) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareFailure(ctx, nil) }, args...)
}
func (s *hardwareFailure) enter() {
	notifyMsg := "entering hardware failure state"
	notifyKind := broadcastGeneric
	if s.err != nil {
		if errEvent, ok := s.err.(errorEvent); ok {
			notifyKind = errEvent.Kind()
		}
		notifyMsg = fmt.Sprintf("entering hardware failure state due to: %v", s.err)
	}
	s.logAndNotify(notifyKind, notifyMsg)
}
func (s *hardwareFailure) exit() {}

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
		sm.ctx.camera.publishEventIfStatus(sm.ctx, e, status, sm.ctx.cfg.CameraMac, sm.ctx.store, sm.log, sm.ctx.bus.publish)
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

		voltage, err := sm.ctx.camera.voltage(sm.ctx)
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
		sm.ctx.camera.publishEventIfStatus(sm.ctx, hardwareStartedEvent{}, true, sm.ctx.cfg.CameraMac, sm.ctx.store, sm.log, sm.ctx.bus.publish)
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
