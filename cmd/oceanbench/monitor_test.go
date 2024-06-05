/*
DESCRIPTION
  Ocean Bench Monitor testing.

AUTHORS
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2023-2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Bench. Ocean Bench is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean Bench is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"testing"

	"github.com/ausocean/cloud/model"
)

func TestSecondsToUptime(t *testing.T) {
	var tests = []struct {
		v       *model.Variable
		want    string
		wantErr error
	}{
		{
			v:       &model.Variable{Value: "90061"},
			want:    "1d 1h 1m 1s",
			wantErr: nil,
		},
		{
			v:       &model.Variable{Value: "0"},
			want:    "0d 0h 0m 0s",
			wantErr: nil,
		},
		{
			v:       &model.Variable{Value: ""},
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
