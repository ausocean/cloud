package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/cmd/oceantv/notifier"
	"github.com/ausocean/cloud/cmd/oceantv/registry"
	"github.com/ausocean/cloud/model"
)

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

func (s *hardwareRestarting) handleTimeEvent(t event.Time) {
	switch s.Substate.(type) {
	case *hardwareStopping:
		withTimeout := s.Substate.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			s.bus.Publish(event.HardwareStopFailed{errors.New("hardware stop timed out")})
			return
		}

		s.Substate.(*hardwareStopping).handleTimeEvent(t)
	case *hardwareStarting:
		withTimeout := s.Substate.(stateWithTimeout)
		if withTimeout.timedOut(t.Time) {
			s.bus.Publish(event.HardwareStartFailed{errors.New("exceeded starting timeout during hardware restart")})
			return
		}

		// If the camera is reporting then the start has completed.
		if s.cameraIsReporting() {
			s.bus.Publish(event.HardwareStarted{})
			return
		}
	default:
		// This is unexpected and probably means we haven't saved a substate properly.
		// So perform a notify log and default to a sensible state.
		s.logAndNotify(notifier.KindSoftware, "unexpected substate in hardwareRestarting: %v, re-entering state to initialise substate", s.Substate)
		s.enter()
	}
}

func (s *hardwareRestarting) handleHardwareStoppedEvent(e event.HardwareStopped) {
	s.log("handling hardware stopped event")
	switch s.Substate.(type) {
	case *hardwareStopping:
		s.transition()
	default:
		// For any other state ignore.
	}
}

func (s *hardwareRestarting) handleHardwareShutdownFailedEvent(e event.HardwareShutdownFailed) {
	switch s.Substate.(type) {
	case *hardwareStopping:
		s.Substate.(*hardwareStopping).handleHardwareShutdownFailedEvent(e)
	default:
		// Ignore.
	}
}

func (s *hardwareRestarting) cameraIsReporting() bool {
	up, err := s.hardware.IsUp(s.broadcastContext.newHWContext(), model.MacDecode(s.cfg.CameraMac))
	if err != nil {
		s.bus.Publish(event.InvalidConfiguration{fmt.Errorf("could not get camera reporting status: %w", err)})
		return false
	}
	return up
}
