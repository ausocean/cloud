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
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

const (
	mtsRequest  = "/mts"
	testSkey    = 1
	testDevID   = "TestDevice"
	testDevMac  = "00:00:00:00:00:01"
	testDevMa   = 1
	testDevDkey = 10000001
	testLat     = "-34.91805"
	testLng     = "138.60475"
)

// TestMtsHandler tests the /mts endpoint.
// NETRECEIVER_CREDENTIALS is required in order to access NetReceiver's datastore.
// VIDGRIND_CREDENTIALS is required in order to access Ocean Bench's datastore.
// Test MTS data all use pin V0, timestamp 1 and PID 0.
func TestMtsHandler(t *testing.T) {
	if os.Getenv("NETRECEIVER_CREDENTIALS") == "" {
		t.Skip("NETRECEIVER_CREDENTIALS required for TestMtsHandler")
	}
	if os.Getenv("VIDGRIND_CREDENTIALS") == "" {
		t.Skip("VIDGRIND_CREDENTIALS required for TestMtsHandler")
	}

	// Create/update the test device.
	ctx := context.Background()
	store, err := datastore.NewStore(ctx, "cloud", "netreceiver", "")
	if err != nil {
		t.Errorf("datastore.NewStore failed with error: %v", err)
	}
	lat, _ := strconv.ParseFloat(testLat, 64)
	lng, _ := strconv.ParseFloat(testLng, 64)
	err = model.PutDevice(ctx, store, &model.Device{
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

	testLoc := testLat + "," + testLng
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
		q := mtsRequest + "?ma=" + test.ma + "&dk=" + strconv.Itoa(test.dk) + "&V0=" + test.pn + "&ts=1&pi=0"

		// Create a POST request.
		r := bytes.NewReader(test.body)
		req, err := http.NewRequest("POST", q, r)
		if err != nil {
			t.Fatal(err)
		}

		// Create a new Recorder to record the response.
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(mtsHandler)

		// Invoke the request.
		handler.ServeHTTP(rr, req)

		// Check the response status code is OK.
		status := rr.Code
		if status != http.StatusOK {
			t.Errorf("mtsHandler #%d returned wrong status code: got %v want %v", i, status, http.StatusOK)
		}

		// Check the response body is as expected.
		if rr.Body.String() != test.want {
			t.Errorf("mtsHandler #%d returned unexpected body: got %v want %v", i, rr.Body.String(), test.want)
		}
	}
}
