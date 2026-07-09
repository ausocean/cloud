/*
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Russell Stanley <russell@ausocean.org>

LICENSE
  Copyright (C) 2025 the Australian Ocean Lab (AusOcean)

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

package main

import (
	"errors"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/event"
)

type directStarting struct {
	stateFields
	stateWithTimeoutFields
}

func newDirectStarting(ctx *broadcastContext) *directStarting {
	return &directStarting{stateWithTimeoutFields: newStateWithTimeoutFields(ctx)}
}

func (s *directStarting) enter() {
	s.LastEntered = time.Now()
	createBroadcastAndRequestHardware(s.broadcastContext, s.cfg, nil)
}

func (s *directStarting) handleEvent(sm *broadcastStateMachine, e event.Event) {
	switch e_ := e.(type) {
	case event.Time:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(e_.Time) {
			onFailureClosure(sm.ctx, sm.ctx.cfg, false)(errors.New("direct starting timed out"))
		}
	case event.HardwareStarted:
		startBroadcast(sm.ctx, sm.ctx.cfg)
	case event.Started:
		sm.transition(&directLive{})
	case event.LowVoltage:
		// If we're in the starting state we need to reset the timeout to allow for
		// hardware voltage recovery (remembering that this is not our primary timeout
		// mechanism, which is handled by the hardware SM but a rather a contingency that
		// we shouldn't hit with normal behaviour).
		const broadcastVoltageRecoveryOffset = 10 * time.Minute
		sm.currentState.(stateWithTimeout).reset(time.Duration(sanatisedVoltageRecoveryTimeout(sm.ctx))*time.Hour + broadcastVoltageRecoveryOffset)
	case event.VoltageRecovered:
		sm.currentState.(stateWithTimeout).reset(5 * time.Minute)
	case event.InvalidConfiguration:
		sm.transition(newDirectFailure(sm.ctx, e_))
	case event.StartFailed:
		sm.transition(newDirectIdle(sm.ctx))
	case event.CriticalFailure:
		sm.transition(newDirectFailure(sm.ctx, e_))
	case event.HardwareStartFailed:
		onFailureClosure(sm.ctx, sm.ctx.cfg, false)(e_)
	case event.ControllerFailure:
		sm.transition(newDirectFailure(sm.ctx, e_))
	case
		event.BadHealth,
		event.Finish,
		event.FixFailure,
		event.Start:
		sm.unexpectedEvent(e, s)
	}
}
