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

type vidforwardPermanentLiveUnhealthy struct {
	stateFields
	*broadcastContext `json: "-"`
	LastResetAttempt  time.Time
	Attempts          int
	liveStateFields
}

func newVidforwardPermanentLiveUnhealthy(ctx *broadcastContext) *vidforwardPermanentLiveUnhealthy {
	return &vidforwardPermanentLiveUnhealthy{broadcastContext: ctx}
}
func (s *vidforwardPermanentLiveUnhealthy) fix() {
	const resetInterval = 5 * time.Minute
	if time.Since(s.LastResetAttempt) <= resetInterval {
		return
	}

	s.Attempts++

	var (
		e   event
		msg string
	)

	const maxAttempts = 3
	if s.Attempts > maxAttempts {
		msg = "failed to fix permanent broadcast, transitioning to slate (attempts: %d, max attempts: %d)"
		e = fixFailureEvent{}
	} else {
		msg = "attempting to fix permanent broadcast by hardware restart and forward stream re-request (attempts: %d, max attempts: %d)"
		try(s.fwd.Stream(s.cfg), "could not set vidforward mode to stream", s.log)
		e = hardwareResetRequestEvent{}
	}

	s.logAndNotify(broadcastGeneric, msg, s.Attempts, maxAttempts)
	s.bus.publish(e)
	s.LastResetAttempt = time.Now()
}
