/*
DESCRIPTION
  Ocean Bench tests.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2019-2024 the Australian Ocean Lab (AusOcean).

  This file is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
  or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
  License for more details.

  You should have received a copy of the GNU General Public License in
  gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package main

import (
	"math"
	"testing"
)

const (
	testSkey    = 1
	testDevID   = "TestDevice"
	testDevMac  = "00:00:00:00:00:01"
	testDevMa   = 1
	testDevDkey = 10000001
	testLat     = "-34.91805"
	testLng     = "138.60475"
)

func TestLatLng(t *testing.T) {
	tests := []struct {
		sentence string
		lat, lng float64
		ok       bool
	}{
		{
			sentence: "$GPGGA,123456,3455.083,S,13836.285,E,1,2,3,4,M,5,M,,*00",
			lat:      -34.918050,
			lng:      138.604750,
			ok:       true,
		},
		{
			sentence: "$GPGGA,123519,4807.038,N,01131.000,W,1,2,3,4,M,5,M,,*00",
			lat:      48.1173,
			lng:      -11.5166667,
			ok:       true,
		},
		{
			sentence: "$GPGGA,junk",
			ok:       false,
		},
	}

	for i, test := range tests {
		lat, lng, ok := parseLatLng(test.sentence)
		if test.ok != ok {
			t.Errorf("parseLatLng #%d returned wrong result: got %t want %t", i, ok, test.ok)
		}
		if test.ok && !(feq(lat, test.lat) && feq(lng, test.lng)) {
			t.Errorf("parseLatLng #%d returned wrong lat,lng: got %f,%f want %f,%f", i, lat, lng, test.lat, test.lng)
		}
	}

}

func feq(a, b float64) bool {
	return math.Abs(a-b) < 0.00001
}

func TestParseLocation(t *testing.T) {
	// We use Victoria Square an an example location.
	tests := []struct {
		in   string
		want location
		ok   bool
	}{
		{
			in:   "-34.92857,138.60006,58.5", // Victoria Square with altitude
			want: location{Lat: -34.92857, Lng: 138.60006, Alt: 58.5},
			ok:   true,
		},
		{
			in:   "-34.92857,138.60006", // Victoria Square without altitude
			want: location{Lat: -34.92857, Lng: 138.60006},
			ok:   true,
		},
		{
			in: "",
			ok: false,
		},
		{
			in: "-34.92857,200",
			ok: false,
		},
		{
			in: "-100,138.60006",
			ok: false,
		},
		{
			in: "-34.92857",
			ok: false,
		},
		{
			in: "-34.92857,138.60006,58.5,0",
			ok: false,
		},
		{
			in: "a.b.c",
			ok: false,
		},
	}

	for _, test := range tests {
		loc, err := parseLocation(test.in)
		if err != nil {
			if test.ok {
				t.Errorf("parseLocation returned unexpected error: %v", err)
			}
			continue
		}
		if !test.ok {
			t.Errorf("parseLocation did not return an error")
		}
		if loc != test.want {
			t.Errorf("parseLocation returned unexpected wrong location: %v", loc)
		}
	}
}
