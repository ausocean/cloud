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
	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/cmd/oceantv/notification"
)

type vidforwardPermanentLive struct {
	stateFields
	liveStateFields
}

func newVidforwardPermanentLive() *vidforwardPermanentLive { return &vidforwardPermanentLive{} }

func (s *vidforwardPermanentLive) handleEvent(sm *broadcastStateMachine, e event.Event) {
	switch e_ := e.(type) {
	case event.InvalidConfiguration:
		// TODO: rather than disabling transition to a failure state.
		sm.logAndNotifyConfiguration("got invalid configuration event, disabling broadcast: %v", e_.Error())
		try(
			sm.ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.Enabled = false }),
			"could not disable broadcast after invalid configuration",
			sm.logAndNotifySoftware,
		)
		sm.transition(newVidforwardPermanentIdle(sm.ctx))
	case event.HardwareStartFailed:
		sm.logAndNotify(notification.KindHardware, "hardware failure event in permanent live state, moving to failure slate state")
		sm.transition(newVidforwardPermanentFailure(sm.ctx))
	case event.BadHealth:
		sm.transition(newVidforwardPermanentLiveUnhealthy(sm.ctx))
	case event.Time:
		if sm.finishIsDue(e_) {
			sm.ctx.bus.Publish(event.Finish{})
			return
		}
		sm.publishHealthStatusOrChatEvents(e_)
	case event.Finish:
		sm.transition(newVidforwardPermanentTransitionLiveToSlate(sm.ctx))
	case
		event.CriticalFailure,
		event.FixFailure,
		event.LowVoltage,
		event.Start,
		event.StartFailed,
		event.Started,
		event.VoltageRecovered:
		sm.unexpectedEvent(e, s)
	}
}
