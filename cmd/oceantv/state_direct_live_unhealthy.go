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
	"fmt"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/cmd/oceantv/notifier"
)

type directLiveUnhealthy struct {
	*broadcastContext `json: "-"`
	LastResetAttempt  time.Time
	Attempts          int
	stateFields
	liveStateFields
}

func newDirectLiveUnhealthy(ctx *broadcastContext) *directLiveUnhealthy {
	return &directLiveUnhealthy{broadcastContext: ctx}
}

func (s *directLiveUnhealthy) handleEvent(sm *broadcastStateMachine, e event.Event) {
	switch e_ := e.(type) {
	case event.Time:
		if sm.finishIsDue(e_) {
			sm.ctx.bus.Publish(event.Finish{})
			return
		}
		sm.publishHealthStatusOrChatEvents(e_)
		sm.tryToFixCurrentState()
	case event.Finish:
		sm.transition(newDirectIdle(sm.ctx))
	case event.InvalidConfiguration:
		sm.transition(newDirectFailure(sm.ctx, e_))
	case event.GoodHealth:
		sm.transition(newDirectLive(sm.ctx))
	case event.FixFailure:
		sm.transition(newDirectFailure(sm.ctx, e_))
	case event.HardwareStartFailed:
		// This causes the hardware to go into failure mode, so we should go into failure mode for the broadcast state too.
		sm.transition(newDirectFailure(sm.ctx, e_.Err))
	case
		event.CriticalFailure,
		event.LowVoltage,
		event.Start,
		event.StartFailed,
		event.Started,
		event.VoltageRecovered:
		sm.unexpectedEvent(e, s)
	}
}

func (s *directLiveUnhealthy) fix() {
	const directLiveFixTimeout = 8 * time.Minute
	if time.Since(s.LastResetAttempt) <= directLiveFixTimeout {
		return
	}

	var e event.Event
	const maxAttempts = 3
	if s.Attempts >= maxAttempts {
		e = event.FixFailure{fmt.Errorf("failed to fix broadcast (attempts: %d, max attempts: %d)", s.Attempts, maxAttempts)}
	} else {
		s.logAndNotify(notifier.KindHardware, "attempting to fix broadcast by hardware restart request (attempts: %d, max attempts: %d)", s.Attempts, maxAttempts)
		s.Attempts++
		e = event.HardwareResetRequest{}
	}

	s.LastResetAttempt = time.Now()
	s.bus.Publish(e)
}
