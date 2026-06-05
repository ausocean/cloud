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
)

type vidforwardSecondaryStarting struct {
	stateFields
	stateWithTimeoutFields
}

func newVidforwardSecondaryStarting(ctx *broadcastContext) *vidforwardSecondaryStarting {
	return &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFields(ctx)}
}

func (s *vidforwardSecondaryStarting) enter() {
	s.LastEntered = time.Now()
	// We pass this to createBroadcastAndRequestHardware so that it's run after
	// broadcast creation, therefore vidforward gets up to date RTMP endpoint
	// information.
	onBroadcastCreation := func() error {
		err := s.fwd.Stream(s.cfg)
		if err != nil {
			return fmt.Errorf("could not set vidforward mode to stream: %w", err)
		}
		return nil
	}
	createBroadcastAndRequestHardware(
		s.broadcastContext,
		s.cfg,
		onBroadcastCreation,
	)
}

func (s *vidforwardSecondaryStarting) handleEvent(sm *broadcastStateMachine, event event) {
	switch e := event.(type) {
	case lowVoltageEvent:
		// If we're in the starting state we need to reset the timeout to allow for
		// hardware voltage recovery (remembering that this is not our primary timeout
		// mechanism, which is handled by the hardware SM but a rather a contingency that
		// we shouldn't hit with normal behaviour).
		const broadcastVoltageRecoveryOffset = 10 * time.Minute
		sm.currentState.(stateWithTimeout).reset(time.Duration(sanatisedVoltageRecoveryTimeout(sm.ctx))*time.Hour + broadcastVoltageRecoveryOffset)
	case voltageRecoveredEvent:
		sm.currentState.(stateWithTimeout).reset(5 * time.Minute)
	case invalidConfigurationEvent:
		// TODO: rather than disabling transition to a failure state.
		sm.logAndNotifyConfiguration("got invalid configuration event, disabling broadcast: %v", e.Error())
		try(
			sm.ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.Enabled = false }),
			"could not disable broadcast after invalid configuration",
			sm.logAndNotifySoftware,
		)
		sm.transition(newVidforwardSecondaryIdle(sm.ctx))
	case startFailedEvent:
		sm.transition(newVidforwardSecondaryIdle(sm.ctx))
	case criticalFailureEvent:
		// There might need to be a secondary failure state, but we're not sure
		// yet. For now, we'll just transition to idle. Most failures will occur
		// as a result of a hardware failure, for which the primary broadcast
		// will subsequently be in a failure state.
		sm.transition(newVidforwardSecondaryIdle(sm.ctx))
	case hardwareStartFailedEvent:
		onFailureClosure(sm.ctx, sm.ctx.cfg, false)(e)
	case controllerFailureEvent:
		onFailureClosure(sm.ctx, sm.ctx.cfg, true)(e)
		sm.transition(newVidforwardSecondaryIdle(sm.ctx))
	case timeEvent:
		sm.transitionIfTimedOut(sm.currentState, newVidforwardSecondaryIdle(sm.ctx), e)
	case hardwareStartedEvent:
		startBroadcast(sm.ctx, sm.ctx.cfg)
	case startedEvent:
		sm.transition(newVidforwardSecondaryLive(sm.ctx))
	}
}
