/*
DESCRIPTION
  VidGrind Monitor testing.

AUTHORS
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2023 the Australian Ocean Lab (AusOcean)

  This file is part of VidGrind. VidGrind is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  VidGrind is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with NetReceiver in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"testing"

	"bitbucket.org/ausocean/iotsvc/iotds"
)

func TestSecondsToUptime(t *testing.T) {
	var tests = []struct {
		v       *iotds.Variable
		want    string
		wantErr error
	}{
		{
			v:       &iotds.Variable{Value: "90061"},
			want:    "1d 1h 1m 1s",
			wantErr: nil,
		},
		{
			v:       &iotds.Variable{Value: "0"},
			want:    "0d 0h 0m 0s",
			wantErr: nil,
		},
		{
			v:       &iotds.Variable{Value: ""},
			want:    "None",
			wantErr: nil,
		},
		{
			want:    "None",
			wantErr: nil,
		},
	}
	for i, test := range tests {
			got, gotErr := secondsToUptime(test.v)
			if got != test.want || gotErr != test.wantErr {
				t.Errorf("did not get expected result for test no. %d \ngot: %s \tgotErr: %v \nwant: %s \twantErr: %v", i, got, gotErr, test.want, test.wantErr)
			}
		}
	}
