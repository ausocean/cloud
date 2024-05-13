/*
DESCRIPTION
  Ocean Bench tests.

AUTHORS
  Alan Noble <alan@ausocean.org>
  Trek Hopton <trek@ausocean.org>

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
	"bytes"
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"bitbucket.org/ausocean/iotsvc/iotds"
)

const (
	testSkey    = 1
	testDevID   = "TestDevice"
	testDevMac  = "00:00:00:00:00:01"
	testDevMa   = 1
	testDevDkey = 10000001
	testLat     = "-34.91805"
	testLng     = "138.60475"
	testAlt     = "0.0"
)

// TestRecvHandler tests  recvHandler
// NETRECEIVER_CREDENTIALS is required in order to access NetReceiver's datastore.
// VIDGRIND_CREDENTIALS is required in order to access Ocean Bench's datastore.
// Test MtsMedia data all use pin V0, timestamp 1 and PID 0.
func TestRecvHandler(t *testing.T) {
	if os.Getenv("NETRECEIVER_CREDENTIALS") == "" {
		t.Skip("NETRECEIVER_CREDENTIALS required for TestRecvHandler")
	}
	if os.Getenv("VIDGRIND_CREDENTIALS") == "" {
		t.Skip("VIDGRIND_CREDENTIALS required for TestRecvHandler")
	}

	// Re-create the test device.
	ctx := context.Background()
	store, err := iotds.NewStore(ctx, "cloud", "netreceiver", "")
	if err != nil {
		t.Errorf("iotds.NewStore failed with error: %v", err)
	}
	lat, _ := strconv.ParseFloat(testLat, 64)
	lng, _ := strconv.ParseFloat(testLng, 64)
	err = iotds.PutDevice(ctx, store, &iotds.Device{
		Skey:          testSkey,
		Dkey:          testDevDkey,
		Name:          testDevID,
		Mac:           testDevMa,
		Inputs:        "V0",
		MonitorPeriod: 60,
		Latitude:      lat,
		Longitude:     lng,
		Enabled:       true,
	})
	if err != nil {
		t.Errorf("PutDevice failed with error: %v", err)
	}

	testLoc := testLat + "," + testLng + "," + testAlt
	tests := []struct {
		ma   string
		dk   int
		pn   string
		body []byte
		want string
	}{
		// Zero-length body, with correct pin value. Should not get written to datastore.
		{
			ma:   testDevMac,
			dk:   testDevDkey,
			pn:   "0",
			want: `{"V0":0,"ll":"` + testLoc + `","ma":"` + testDevMac + `","ts":1}`,
		},
		// Invalid MAC address.
		{
			ma:   "00:00:00:00:00:00",
			dk:   testDevDkey,
			pn:   "0",
			body: []byte{},
			want: `{"er":"invalid MAC address"}`,
		},
		// Invalid device key.
		{
			ma:   testDevMac,
			dk:   0,
			pn:   "0",
			body: []byte{},
			want: `{"er":"invalid device key","rc":1}`,
		},
		// Invalid pin value.
		{
			ma:   testDevMac,
			dk:   testDevDkey,
			pn:   "foo",
			body: []byte{},
			want: `{"er":"invalid value","ll":"` + testLoc + `","ma":"` + testDevMac + `","ts":1}`,
		},
	}

	for i, test := range tests {
		// Form the query.
		q := "/recv?ma=" + test.ma + "&dk=" + strconv.Itoa(test.dk) + "&V0=" + test.pn + "&ts=1&pi=0"

		// Create a POST request.
		r := bytes.NewReader(test.body)
		req, err := http.NewRequest("POST", q, r)
		if err != nil {
			t.Fatal(err)
		}

		// Create a new Recorder to record the response.
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(recvHandler)

		// Invoke the request.
		handler.ServeHTTP(rr, req)

		// Check the response status code is OK.
		status := rr.Code
		if status != http.StatusOK {
			t.Errorf("recvHandler #%d returned wrong status code: got %v want %v", i, status, http.StatusOK)
		}

		// Check the response body is as expected.
		if rr.Body.String() != test.want {
			t.Errorf("recvHandler #%d returned unexpected body: got %v want %v", i, rr.Body.String(), test.want)
		}
	}
}

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

func TestConfigJSON(t *testing.T) {
	// Test case 1: device key is not provided.
	dev1 := &iotds.Device{
		Mac:           iotds.MacEncode("00:11:22:33:44:55"),
		Wifi:          "SSID,PASS",
		Inputs:        "S0,T0",
		Outputs:       "D1,D2",
		MonitorPeriod: 60,
		ActPeriod:     60,
		Version:       "1.0.0",
	}
	var vs1 int64 = 1
	dk1 := ""
	expectedJSON1 := `{"ma":"00:11:22:33:44:55","wi":"SSID,PASS","ip":"S0,T0","op":"D1,D2","mp":60,"ap":60,"cv":"1.0.0","vs":1}`

	result1, err := configJSON(dev1, vs1, dk1)
	if err != nil {
		t.Errorf("Test case 1 failed. Error: %v", err)
	} else if result1 != expectedJSON1 {
		t.Errorf("Test case 1 failed. Expected %s, but got %s", expectedJSON1, result1)
	}

	// Test case 2: device key is provided.
	dev2 := &iotds.Device{
		Mac:           iotds.MacEncode("00:11:22:33:44:55"),
		Wifi:          "SSID,PASS",
		Inputs:        "V0,T0",
		Outputs:       "X1,D2",
		MonitorPeriod: 120,
		ActPeriod:     120,
		Version:       "12.2.2",
	}
	var vs2 int64 = 2
	dk2 := "10"
	expectedJSON2 := `{"ma":"00:11:22:33:44:55","wi":"SSID,PASS","ip":"V0,T0","op":"X1,D2","mp":120,"ap":120,"cv":"12.2.2","vs":2,"dk":"10"}`

	result2, err := configJSON(dev2, vs2, dk2)
	if err != nil {
		t.Errorf("Test case 2 failed. Error: %v", err)
	} else if result2 != expectedJSON2 {
		t.Errorf("Test case 2 failed. Expected %s, but got %s", expectedJSON2, result2)
	}
}
