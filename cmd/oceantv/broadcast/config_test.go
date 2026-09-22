/*
AUTHORS
  Elliot Shine <elliot@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

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

package broadcast

import (
	"encoding/json"
	"testing"
)

func TestParseActionVars(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    ActionVars
		wantErr bool
	}{
		{
			name: "empty string",
			in:   "",
			want: nil,
		},
		{
			name: "skip",
			in:   "skip",
			want: ActionVars{{Name: "skip"}},
		},
		{
			name: "json array with comma-bearing value",
			in:   `[{"name":"Camera.mode","value":"a,b=c"}]`,
			want: ActionVars{{Name: "Camera.mode", Value: "a,b=c"}},
		},
		{
			name: "legacy csv",
			in:   "ESP.Power2=true,Camera.mode=Normal",
			want: ActionVars{{Name: "ESP.Power2", Value: "true"}, {Name: "Camera.mode", Value: "Normal"}},
		},
		{
			name: "legacy csv value containing equals",
			in:   "Camera.mode=a=b",
			want: ActionVars{{Name: "Camera.mode", Value: "a=b"}},
		},
		{
			name: "legacy csv with whitespace",
			in:   " ESP.Power2 = true , Camera.mode = Normal ",
			want: ActionVars{{Name: "ESP.Power2", Value: "true"}, {Name: "Camera.mode", Value: "Normal"}},
		},
		{
			name:    "malformed json",
			in:      `[{"name":`,
			wantErr: true,
		},
		{
			name:    "json object instead of array",
			in:      `{"name":"a","value":"b"}`,
			wantErr: true,
		},
		{
			name:    "legacy csv missing equals",
			in:      "ESP.Power2",
			wantErr: true,
		},
		{
			name:    "legacy csv empty name",
			in:      "=true",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseActionVars(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseActionVars() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			assertActionVars(t, got, tt.want)
		})
	}
}

func TestActionVarsUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    ActionVars
		wantErr bool
	}{
		{
			name: "legacy csv string",
			in:   `{"Actions":"ESP.Power2=true,Camera.mode=a=b"}`,
			want: ActionVars{{Name: "ESP.Power2", Value: "true"}, {Name: "Camera.mode", Value: "a=b"}},
		},
		{
			name: "legacy skip string",
			in:   `{"Actions":"skip"}`,
			want: ActionVars{{Name: "skip"}},
		},
		{
			name: "json array",
			in:   `{"Actions":[{"name":"Camera.mode","value":"a,b=c"}]}`,
			want: ActionVars{{Name: "Camera.mode", Value: "a,b=c"}},
		},
		{
			name: "null",
			in:   `{"Actions":null}`,
			want: nil,
		},
		{
			name:    "malformed legacy string",
			in:      `{"Actions":"ESP.Power2"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got struct {
				Actions ActionVars
			}
			err := json.Unmarshal([]byte(tt.in), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("json.Unmarshal() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			assertActionVars(t, got.Actions, tt.want)
		})
	}
}

// TestActionVarsRoundTripLegacy ensures a legacy CSV record is parsed and then
// re-marshalled as a JSON array without data loss.
func TestActionVarsRoundTripLegacy(t *testing.T) {
	type config struct {
		OnActions ActionVars
	}

	const legacy = `{"OnActions":"ESP.Power2=true,Camera.mode=Normal"}`
	want := ActionVars{{Name: "ESP.Power2", Value: "true"}, {Name: "Camera.mode", Value: "Normal"}}

	var first config
	err := json.Unmarshal([]byte(legacy), &first)
	if err != nil {
		t.Fatalf("could not unmarshal legacy config: %v", err)
	}
	assertActionVars(t, first.OnActions, want)

	data, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("could not marshal config: %v", err)
	}

	var second config
	err = json.Unmarshal(data, &second)
	if err != nil {
		t.Fatalf("could not unmarshal round-tripped config: %v", err)
	}
	assertActionVars(t, second.OnActions, want)
}

// TestActionVarsRoundTripComma ensures the JSON array format preserves values
// containing commas (the original bug).
func TestActionVarsRoundTripComma(t *testing.T) {
	type config struct {
		OnActions ActionVars
	}

	want := ActionVars{{Name: "Camera.StorageConfig", Value: `{"Bucket":"b","Prefix":"p"}`}}

	data, err := json.Marshal(config{OnActions: want})
	if err != nil {
		t.Fatalf("could not marshal config: %v", err)
	}

	var got config
	err = json.Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("could not unmarshal round-tripped config: %v", err)
	}
	assertActionVars(t, got.OnActions, want)
}

func assertActionVars(t *testing.T, got, want ActionVars) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d action vars, want %d: got %+v, want %+v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("action var[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
