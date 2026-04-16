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
	"encoding/json"
	"errors"
	"fmt"
)

type directFailure struct {
	err error
	stateFields
	*broadcastContext `json: "-"`
}

func newDirectFailure(ctx *broadcastContext, err error) *directFailure {
	return &directFailure{broadcastContext: ctx, err: err}
}
func (s *directFailure) enter() {
	notifyMsg := "entering direct broadcast failure state"
	notifyKind := broadcastGeneric
	if s.err != nil {
		if errEvent, ok := s.err.(errorEvent); ok {
			notifyKind = errEvent.Kind()
		}
		notifyMsg = fmt.Sprintf("entering direct broadcast failure state due to: %v", s.err)
	}
	s.logAndNotify(notifyKind, notifyMsg)

	err := s.man.StopBroadcast(context.Background(), s.cfg, s.store, s.svc)
	if err != nil {
		s.log("could not stop broadcast on entry: %v", err)
	} else {
		s.bus.publish(finishedEvent{})
	}
	s.bus.publish(hardwareStopRequestEvent{})
}

func (s *directFailure) MarshalJSON() ([]byte, error) {
	if s.err != nil {
		return json.Marshal(struct{ Err string }{Err: s.err.Error()})
	}

	return json.Marshal(struct{ Err string }{Err: ""})
}

func (s *directFailure) UnmarshalJSON(data []byte) error {
	aux := struct{ Err string }{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	s.err = errors.New(aux.Err)
	return nil
}
