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
	"context"

	"github.com/ausocean/cloud/cmd/oceantv/event"
)

type directIdle struct {
	stateFields
	*broadcastContext `json: "-"`
}

func newDirectIdle(ctx *broadcastContext) *directIdle { return &directIdle{broadcastContext: ctx} }

func (s *directIdle) enter() {
	err := s.man.StopBroadcast(context.Background(), s.cfg, s.store, s.svc)
	if err != nil {
		s.log("could not stop broadcast on entry: %v", err)
	} else {
		s.bus.Publish(event.Finished{})
	}
	s.bus.Publish(event.HardwareStopRequest{})
}

func (s *directIdle) handleEvent(sm *broadcastStateMachine, e event.Event) {
	switch e_ := e.(type) {
	case event.Time:
		if sm.startIsDue(e_) {
			sm.ctx.bus.Publish(event.Start{})
			return
		} else {
			sm.log("start is not due, Start: %v, End: %v, time of event: %v", sm.ctx.cfg.Start.Format("15:04"), sm.ctx.cfg.End.Format("15:04"), e_.Time.Format("15:04"))
		}
	case event.Start:
		sm.transition(newDirectStarting(sm.ctx))
	case
		event.BadHealth,
		event.CriticalFailure,
		event.Finish,
		event.FixFailure,
		event.HardwareStartFailed,
		event.InvalidConfiguration,
		event.LowVoltage,
		event.StartFailed,
		event.Started,
		event.VoltageRecovered:
		sm.unexpectedEvent(e, s)
	}
}
