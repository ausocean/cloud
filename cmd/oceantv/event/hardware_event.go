/*
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package event

import (
	"errors"

	"github.com/ausocean/cloud/cmd/oceantv/notifier"
	"github.com/ausocean/cloud/notify"
)

type HardwareShutdownFailed struct{ Err error }

var _ = registerEvent(HardwareShutdownFailed{})

func (e HardwareShutdownFailed) String() string { return "hardwareShutdownFailedEvent" }
func (e HardwareShutdownFailed) Error() string {
	if e.Err == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.Err.Error()
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
	if errEvent, ok := e.Err.(Error); ok {
		return errEvent.Kind()
	}

	if unwrapped := UnwrapErrEvent(e.Err, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return notifier.KindHardware
}

func (e HardwareShutdownFailed) Unwrap() error { return e.Err }

func (e HardwareShutdownFailed) Is(target error) bool {
	if _, ok := target.(HardwareShutdownFailed); ok {
		return true
	}
	return errors.Is(e.Err, target)
}

type HardwareShutdown struct{}

var _ = registerEvent(HardwareShutdown{})

func (e HardwareShutdown) String() string { return "hardwareShutdownEvent" }

type HardwarePowerOffFailed struct{ Err error }

var _ = registerEvent(HardwarePowerOffFailed{})

func (e HardwarePowerOffFailed) String() string { return "hardwarePowerOffFailedEvent" }
func (e HardwarePowerOffFailed) Error() string {
	if e.Err == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.Err.Error()
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
	if errEvent, ok := e.Err.(Error); ok {
		return errEvent.Kind()
	}

	if unwrapped := UnwrapErrEvent(e.Err, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return notifier.KindHardware
}

func (e HardwarePowerOffFailed) Unwrap() error { return e.Err }

func (e HardwarePowerOffFailed) Is(target error) bool {
	if _, ok := target.(HardwarePowerOffFailed); ok {
		return true
	}
	return errors.Is(e.Err, target)
}
