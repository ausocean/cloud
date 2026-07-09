package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/cmd/oceantv/notification"
	"github.com/ausocean/cloud/cmd/oceantv/registry"
	"github.com/ausocean/cloud/model"
)

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

func (s *hardwareStopping) handleTimeEvent(t event.Time) {
	switch s.Substate.(type) {
	case *hardwareShuttingDown:
		s.log("(hardwareStopping) handling timeEvent in hardwareStopping state: substate is hardwareShuttingDown")
		withTimeout := s.Substate.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			s.bus.Publish(event.HardwareShutdownFailed{errors.New("hardware shutdown timed out")})
			return
		}

		if !s.cameraIsReporting() {
			s.bus.Publish(event.HardwareShutdown{})
			s.transition()
			return
		}
		s.log("(hardwareStopping) camera is still reporting, waiting for shutdown to complete")

	case *hardwarePoweringOff:
		s.log("(hardwareStopping) handling timeEvent in hardwareStopping state: substate is hardwarePoweringOff")
		withTimeout := s.Substate.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			s.bus.Publish(event.HardwarePowerOffFailed{errors.New("hardware power off timed out")})
			return
		}

		if !s.cameraIsReporting() {
			s.bus.Publish(event.HardwareStopped{})
			return
		}
		s.log("(hardwareStopping) camera is still reporting, waiting for power off to complete")
	default:
		// This is unexpected and probably means we haven't saved a substate properly.
		// So perform a notify log and default to a sensible state.
		s.logAndNotify(notification.KindSoftware, "unexpected substate in hardwareStopping: %v, re-entering state to initialise substate", s.Substate)
		s.enter()
	}
}

func (s *hardwareStopping) handleHardwareShutdownFailedEvent(e event.HardwareShutdownFailed) {
	switch s.Substate.(type) {
	case *hardwareShuttingDown:
		// We want to get notified for failures and misconfigured configs, and log
		// when shutdown is skipped.
		if errors.Is(e, broadcast.WarnSkipShutdown) {
			s.log("skipping shutdown: %v:", e.Error)
		} else if errors.Is(e, errNoShutdownActions) {
			s.logAndNotify(notification.KindHardware, "shutdown skipped: %v", e.Error())
		}
		s.transition()
	default:
		// Ignore.
	}
}

func (s *hardwareStopping) handleHardwarePowerOffFailedEvent(e event.HardwarePowerOffFailed) {
	switch s.Substate.(type) {
	case *hardwarePoweringOff:
		s.bus.Publish(event.HardwareStopFailed{e})
	default:
		// Ignore.
	}
}

func (s *hardwareStopping) cameraIsReporting() bool {
	up, err := s.hardware.isUp(s.broadcastContext, model.MacDecode(s.cfg.CameraMac))
	if err != nil {
		s.bus.Publish(event.InvalidConfiguration{fmt.Errorf("could not get camera reporting status: %w", err)})
		return false
	}
	return up
}
