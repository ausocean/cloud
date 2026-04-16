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
func (s *directLiveUnhealthy) fix() {
	const directLiveFixTimeout = 8 * time.Minute
	if time.Since(s.LastResetAttempt) <= directLiveFixTimeout {
		return
	}

	var e event
	const maxAttempts = 3
	if s.Attempts >= maxAttempts {
		e = fixFailureEvent{fmt.Errorf("failed to fix broadcast (attempts: %d, max attempts: %d)", s.Attempts, maxAttempts)}
	} else {
		s.logAndNotify(broadcastHardware, "attempting to fix broadcast by hardware restart request (attempts: %d, max attempts: %d)", s.Attempts, maxAttempts)
		s.Attempts++
		e = hardwareResetRequestEvent{}
	}

	s.LastResetAttempt = time.Now()
	s.bus.publish(e)
}
