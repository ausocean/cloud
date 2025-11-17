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

type vidforwardPermanentVoltageRecoverySlate struct {
	stateFields
	stateWithTimeoutFields
}

func newVidforwardPermanentVoltageRecoverySlate(ctx *broadcastContext) *vidforwardPermanentVoltageRecoverySlate {
	return &vidforwardPermanentVoltageRecoverySlate{
		stateWithTimeoutFields: newStateWithTimeoutFieldsWithTimeout(
			ctx,
			time.Duration(sanatisedVoltageRecoveryTimeout(ctx))*time.Hour,
		),
	}
}
func (s *vidforwardPermanentVoltageRecoverySlate) enter() {
	s.LastEntered = time.Now()
	s.requestSlate()
}
func (s *vidforwardPermanentVoltageRecoverySlate) fix() { s.requestSlate() }
func (s *vidforwardPermanentVoltageRecoverySlate) requestSlate() {
	try(s.fwd.Slate(s.cfg, WithType(LowVoltage)), "could not set vidforward mode to low voltage slate", s.log)
}
