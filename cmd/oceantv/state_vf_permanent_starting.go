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
	"errors"
	"fmt"
	"time"
)

type vidforwardPermanentStarting struct {
	stateFields
	stateWithTimeoutFields
}

func newVidforwardPermanentStarting(ctx *broadcastContext) *vidforwardPermanentStarting {
	return &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFields(ctx)}
}

func (s *vidforwardPermanentStarting) enter() {
	s.LastEntered = time.Now()

	// Use a copy of the config so that we can adjust the end date to +1 year
	// without affecting the original config.
	cfg := *s.cfg
	cfg.End = cfg.End.AddDate(1, 0, 0)

	if !try(s.man.SetupSecondary(context.Background(), s.cfg, s.store), "could not setup secondary broadcast", s.log) {
		s.bus.publish(startFailedEvent{})
		return
	}

	// We pass this to createAndStart so that it's run after broadcast creation, therefore
	// vidforward gets up to date RTMP endpoint information.
	onBroadcastCreation := func() error {
		err := s.fwd.Stream(&cfg)
		if err != nil {
			return fmt.Errorf("could not set vidforward mode to stream: %w", err)
		}
		return nil
	}

	createBroadcastAndRequestHardware(
		s.broadcastContext,
		&cfg,
		onBroadcastCreation,
	)
}

func (s *vidforwardPermanentStarting) handleEvent(sm *broadcastStateMachine, event event) {
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
		sm.transition(newVidforwardPermanentIdle(sm.ctx))
	case startFailedEvent:
		sm.transition(newVidforwardPermanentIdle(sm.ctx))
	case criticalFailureEvent:
		sm.transition(newVidforwardPermanentFailure(sm.ctx))
	case hardwareStartFailedEvent:
		onFailureClosure(sm.ctx, sm.ctx.cfg, false)(e)
	case controllerFailureEvent:
		onFailureClosure(sm.ctx, sm.ctx.cfg, true)(e)
		sm.transition(newVidforwardPermanentIdle(sm.ctx))
	case timeEvent:
		withTimeout := sm.currentState.(stateWithTimeout)
		if withTimeout.timedOut(e.Time) {
			onFailureClosure(sm.ctx, sm.ctx.cfg, false)(errors.New("permanent starting timed out"))
		}
	case hardwareStartedEvent:
		startBroadcast(sm.ctx, sm.ctx.cfg)
	case startedEvent:
		sm.transition(newVidforwardPermanentLive())
	}
}
