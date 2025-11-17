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

import "context"

type vidforwardSecondaryLive struct {
	stateFields
	*broadcastContext `json: "-"`
	liveStateFields
}

func newVidforwardSecondaryLive(ctx *broadcastContext) *vidforwardSecondaryLive {
	return &vidforwardSecondaryLive{broadcastContext: ctx}
}
func (s *vidforwardSecondaryLive) exit() {
	err := s.man.StopBroadcast(context.Background(), s.cfg, s.store, s.svc)
	if err != nil {
		s.log("could not stop broadcast on exit: %v", err)
		return
	}
	s.bus.publish(finishedEvent{})
}
