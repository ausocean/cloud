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

import "time"

type vidforwardPermanentTransitionSlateToLive struct {
	stateFields
	HardwareStarted bool
	stateWithTimeoutFields
	stateWithHealthFields
}

func newVidforwardPermanentTransitionSlateToLive(ctx *broadcastContext) *vidforwardPermanentTransitionSlateToLive {
	return &vidforwardPermanentTransitionSlateToLive{stateWithTimeoutFields: newStateWithTimeoutFields(ctx)}
}

func (s *vidforwardPermanentTransitionSlateToLive) enter() {
	s.LastEntered = time.Now()
	s.bus.publish(hardwareStartRequestEvent{})
	try(s.fwd.Stream(s.cfg), "could not set vidforward mode to stream", s.log)
}

func (s *vidforwardPermanentTransitionSlateToLive) handleEvent(sm *broadcastStateMachine, event event) {
	switch e := event.(type) {
	case lowVoltageEvent:
		sm.transition(newVidforwardPermanentVoltageRecoverySlate(sm.ctx))
	case invalidConfigurationEvent:
		// TODO: rather than disabling transition to a failure state.
		sm.logAndNotifyConfiguration("got invalid configuration event, disabling broadcast: %v", e.Error())
		try(
			sm.ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.Enabled = false }),
			"could not disable broadcast after invalid configuration",
			sm.logAndNotifySoftware,
		)
		sm.transition(newVidforwardPermanentIdle(sm.ctx))
	case hardwareStartFailedEvent:
		sm.logAndNotify(broadcastHardware, "hardware failure event in transition from slate to live, moving to failure slate state")
		sm.transition(newVidforwardPermanentFailure(sm.ctx))
	case goodHealthEvent:
		if s.isHardwareStarted() {
			sm.transition(newVidforwardPermanentLive())
		}
	case timeEvent:
		if s.timedOut(e.Time) {
			sm.ctx.logAndNotify(broadcastGeneric, "transition from slate to live timed out, transitioning to failure slate state")
			sm.transition(newVidforwardPermanentFailure(sm.ctx))
		}
		sm.publishHealthStatusOrChatEvents(e)
	}
}

func (s *vidforwardPermanentTransitionSlateToLive) isHardwareStarted() bool {
	return s.cfg.HardwareState == hardwareStateToString(&hardwareOn{})
}
