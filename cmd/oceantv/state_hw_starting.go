package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/model"
)

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
		s.hardware.start(s.broadcastContext)
		return
	}

	// The first check for any known hardware error states.
	hwErr, err := s.hardware.error(s.broadcastContext)
	if err != nil {
		errWrapped := fmt.Errorf("could not get hardware error state: %w", err)
		s.log(errWrapped.Error())
		// NOTE here we could do this, however it's not certain that all ESPs
		// will have the latest firmware that supports this, and so it's not
		// necessarily a showstopper.
		// s.bus.Publish(event.InvalidConfiguration{errWrapped})
		// return
	}

	switch {
	case errors.Is(hwErr, LowVoltageAlarm):
		s.log("controller voltage is low, waiting for recovery before starting")
		s.bus.Publish(event.LowVoltage{})
		return
	case errors.Is(hwErr, None):
		// Continue other checks, this is good.
	case hwErr != nil:
		errWrapped := fmt.Errorf("unhandled controller hardware error: %w", hwErr)
		s.log(errWrapped.Error())
		s.bus.Publish(event.InvalidConfiguration{errWrapped})
		return
	default:
		// This means we failed to get hwErr, which at this stage just means
		// we have a controller that doesn't have the latest firmware.
	}

	voltage, err := s.hardware.voltage(s.broadcastContext)
	if err != nil {
		errWrapped := fmt.Errorf("could not get hardware voltage: %w", err)
		s.log(errWrapped.Error())
		s.bus.Publish(event.InvalidConfiguration{errWrapped})
		return
	}

	alarmVoltage, err := s.hardware.alarmVoltage(s.broadcastContext)
	if err != nil {
		errWrapped := fmt.Errorf("could not get alarm voltage: %w", err)
		s.log(errWrapped.Error())
		s.bus.Publish(event.InvalidConfiguration{errWrapped})
		return
	}

	controllerIsOn, err := s.hardware.isUp(s.broadcastContext, model.MacDecode(s.cfg.ControllerMAC))
	if err != nil {
		errWrapped := fmt.Errorf("could not get controller status: %w", err)
		s.log(errWrapped.Error())
		s.bus.Publish(event.InvalidConfiguration{errWrapped})
		return
	}

	if voltage <= alarmVoltage {
		if controllerIsOn {
			s.log("voltage less than alarm voltage but controller is on, something is configured incorrectly")
			s.bus.Publish(event.InvalidConfiguration{errors.New("voltage less than alarm voltage but controller is on")})
			return
		}
		s.log("controller voltage is low, waiting for recovery before starting")
		s.bus.Publish(event.LowVoltage{})
		return
	}

	// Not below alarm voltage, but controller is not responding.
	// This is a critical failure.
	if !controllerIsOn {
		s.bus.Publish(event.ControllerFailure{errors.New("controller not responding above alarm voltage")})
		return
	}

	// Controller is reporting, but we're not above streaming voltage. Need
	// to wait for recovery.
	if voltage < s.cfg.RequiredStreamingVoltage {
		s.log("controller voltage is below required streaming voltage, waiting for recovery before starting")
		s.bus.Publish(event.LowVoltage{})
		return
	}

	// Controller is reporting and we're above streaming voltage, let's power
	// on the hardware.
	s.hardware.start(s.broadcastContext)
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
