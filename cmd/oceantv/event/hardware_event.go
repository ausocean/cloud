package event

import (
	"errors"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/notify"
)

type HardwareShutdownFailed struct{ error }

var _ = registerEvent(HardwareShutdownFailed{})

func (e HardwareShutdownFailed) String() string { return "hardwareShutdownFailedEvent" }
func (e HardwareShutdownFailed) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}
func (e HardwareShutdownFailed) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return HardwareShutdownFailed{err}, nil
}

// Kind implements the errorEvent interface.
func (e HardwareShutdownFailed) Kind() notify.Kind {
	if errEvent, ok := e.error.(Error); ok {
		return errEvent.Kind()
	}

	if unwrapped := UnwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcast.KindHardware
}

func (e HardwareShutdownFailed) Unwrap() error { return e.error }

func (e HardwareShutdownFailed) Is(target error) bool {
	if _, ok := target.(HardwareShutdownFailed); ok {
		return true
	}
	return errors.Is(e.error, target)
}

type HardwareShutdown struct{}

var _ = registerEvent(HardwareShutdown{})

func (e HardwareShutdown) String() string { return "hardwareShutdownEvent" }

type HardwarePowerOffFailed struct{ error }

var _ = registerEvent(HardwarePowerOffFailed{})

func (e HardwarePowerOffFailed) String() string { return "hardwarePowerOffFailedEvent" }
func (e HardwarePowerOffFailed) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}
func (e HardwarePowerOffFailed) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return HardwarePowerOffFailed{err}, nil
}

// Kind implements the errorEvent interface.
func (e HardwarePowerOffFailed) Kind() notify.Kind {
	if errEvent, ok := e.error.(Error); ok {
		return errEvent.Kind()
	}

	if unwrapped := UnwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcast.KindHardware
}

func (e HardwarePowerOffFailed) Unwrap() error { return e.error }

func (e HardwarePowerOffFailed) Is(target error) bool {
	if _, ok := target.(HardwarePowerOffFailed); ok {
		return true
	}
	return errors.Is(e.error, target)
}
