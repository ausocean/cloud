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
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/cmd/oceantv/notification"
)

type vidforwardPermanentTransitionLiveToSlate struct {
	stateFields
	stateWithTimeoutFields
	stateWithHealthFields
	HardwareStopped bool
}

func newVidforwardPermanentTransitionLiveToSlate(ctx *broadcastContext) *vidforwardPermanentTransitionLiveToSlate {
	return &vidforwardPermanentTransitionLiveToSlate{stateWithTimeoutFields: newStateWithTimeoutFields(ctx)}
}

func (s *vidforwardPermanentTransitionLiveToSlate) enter() {
	s.LastEntered = time.Now()

	s.bus.Publish(event.HardwareStopRequest{})
	try(s.fwd.Slate(s.cfg), "could not set vidforward mode to slate", s.log)
}

func (s *vidforwardPermanentTransitionLiveToSlate) handleEvent(sm *broadcastStateMachine, e event.Event) {
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
	case event.GoodHealth:
		if sm.currentState.(*vidforwardPermanentTransitionLiveToSlate).isHardwareStopped() {
			sm.transition(newVidforwardPermanentSlate())
		}
	case event.Time:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(e_.Time) {
			sm.logAndNotify(notification.KindForwarder, "transition from live to slate timed out, staying in live state, check forwarding service")
			sm.transition(newVidforwardPermanentLive())
		}
		sm.publishHealthEvent(e_)
	case
		event.BadHealth,
		event.CriticalFailure,
		event.Finish,
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

func (s *vidforwardPermanentTransitionLiveToSlate) isHardwareStopped() bool {
	return s.cfg.HardwareState == hardwareStateToString(&hardwareOff{})
}
