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

import "github.com/ausocean/cloud/cmd/oceantv/event"

type directLive struct {
	*broadcastContext `json: "-"`
	stateFields
	liveStateFields
}

func newDirectLive(ctx *broadcastContext) *directLive {
	return &directLive{broadcastContext: ctx}
}

func (s *directLive) handleEvent(sm *broadcastStateMachine, e event.Event) {
	switch e_ := e.(type) {
	case event.InvalidConfiguration:
		sm.transition(newDirectFailure(sm.ctx, e_))
	case event.BadHealth:
		sm.transition(newDirectLiveUnhealthy(sm.ctx))
	case event.Time:
		if sm.finishIsDue(e_) {
			sm.ctx.bus.Publish(event.Finish{})
			return
		}
		sm.publishHealthStatusOrChatEvents(e_)
	case event.Finish:
		sm.transition(newDirectIdle(sm.ctx))
	case
		event.CriticalFailure,
		event.FixFailure,
		event.HardwareStartFailed,
		event.LowVoltage,
		event.Start,
		event.StartFailed,
		event.Started,
		event.VoltageRecovered:
		sm.unexpectedEvent(e, s)
	}
}
