/*
AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  It is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package gauth

import (
	"encoding/hex"
	"reflect"
	"testing"
)

// TestJWT tests signing and unsigning of JWT claims.
func TestJWT(t *testing.T) {
	const hexSecret = "3af320667aba6a8b9ff9dc475adb382c"
	secret, err := hex.DecodeString(hexSecret)
	if err != nil {
		t.Fatalf("could not decode hexSecret: %v", err)
	}

	tests := []struct {
		claims map[string]interface{}
		want   string
	}{
		{
			claims: map[string]interface{}{},
			want:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.Rb6nbRORsCc2-_6aZjNE8YJ4dGCd_TQeslWYL3r0A38",
		},
		{
			claims: map[string]interface{}{"iss": "foo@bar.com"},
			want:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJmb29AYmFyLmNvbSJ9.AamfMMNGPn6OYkC5hFtQa5WZQ_waIgeU3-UHzOXdpdw",
		},
		{
			claims: map[string]interface{}{"iss": "foo@bar.com", "skey": 1.0},
			want:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJmb29AYmFyLmNvbSIsInNrZXkiOjF9.7TgjYQThG8Zmaa_Vi_VdX8858nh-9FkyI3ox5JvxbDo",
		},
	}

	for i, test := range tests {
		tokString, err := PutClaims(test.claims, secret)
		if err != nil {
			t.Errorf("PutClaims#%d failed with unexpected error: %v", i, err)
		}
		if tokString != test.want {
			t.Errorf("PutClaims#%d failed: expected %s, got %s", i, test.want, tokString)
		}
		claims, err := GetClaims(tokString, secret)
		if err != nil {
			t.Errorf("GetClaims#%d failed with unexpected error: %v", i, err)
		}
		if !reflect.DeepEqual(claims, test.claims) {
			t.Errorf("GetClaims#%d failed: expected %v, got %v", i, test.claims, claims)
		}
	}
}
