/*
DESCRIPTION
  Tests for the Ocean Bench's data handler.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2023-2024 the Australian Ocean Lab (AusOcean).

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
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"bitbucket.org/ausocean/iotsvc/iotds"
)

const (
	amplitude    = 256
	offset       = 512
	minutesInDay = 1440
	siteKey      = 1
	mac          = "00:00:00:00:00:01"
	pin          = "A0"
	dateFmt      = "2006-01-02 15:04"
)

func init() {
	iotds.RegisterEntities()
}

// TestPutScalars creates one day's worth of test data at one-minute intervals
// starting from iotds.EpochStart (i.e., timestamps 1483228800 to 1483315200).
func TestPutScalars(t *testing.T) {
	t.Log("TestPutScalars")
	if os.Getenv("VIDGRIND_CREDENTIALS") == "" {
		t.Skipf("VIDGRIND_CREDENTIALS missing")
	}

	ctx := context.Background()
	store, err := iotds.NewStore(ctx, "cloud", "vidgrind", "")
	if err != nil {
		t.Errorf("NewStore failed with error: %v", err)
	}

	// Skip if data is already present.
	id := iotds.ToSID(mac, pin)
	scalars, err := iotds.GetScalars(ctx, store, id, []int64{iotds.EpochStart, iotds.EpochStart + minutesInDay*60})
	if err != nil {
		t.Errorf("GetScalars failed with error: %v", err)
	}
	if len(scalars) == minutesInDay {
		t.Skip("Skipping TestPutScalars")
	}

	ts := int64(iotds.EpochStart)
	samples := sampleSinusoid(amplitude, offset, minutesInDay)
	for i := 0; i < minutesInDay; i++ {
		err := iotds.PutScalar(ctx, store, &iotds.Scalar{ID: id, Timestamp: ts, Value: float64(samples[i])})
		if err != nil {
			t.Errorf("PutScalar failed with error: %v", err)
		}
		ts += 60
	}
}

// TestGetScalars tests retrieving scalars directly from the datastore.
func TestGetScalars(t *testing.T) {
	t.Log("TestGetScalars")
	if os.Getenv("VIDGRIND_CREDENTIALS") == "" {
		t.Skipf("VIDGRIND_CREDENTIALS missing")
	}
	ctx := context.Background()
	store, err := iotds.NewStore(ctx, "cloud", "vidgrind", "")
	if err != nil {
		t.Errorf("NewStore failed with error: %v", err)
	}

	id := iotds.ToSID(mac, pin)
	scalars, err := iotds.GetScalars(ctx, store, id, []int64{iotds.EpochStart, iotds.EpochStart + minutesInDay*60})
	if err != nil {
		t.Errorf("GetScalars failed with error: %v", err)
	}
	if len(scalars) != minutesInDay {
		t.Errorf("Expected %d scalars, got %d", minutesInDay*60, len(scalars))
	}

	samples := sampleSinusoid(amplitude, offset, minutesInDay)
	ts := int64(iotds.EpochStart)
	for i := 0; i < minutesInDay; i++ {
		if int64(scalars[i].Value) != samples[i] {
			t.Errorf("#%d: expected %d, got %f", i, samples[i], scalars[i].Value)
		}
		ts += 60
	}
}

// TestFestScalars tests fetching scalars via Ocean Bench's /data endpoint.
func TestFetchScalars(t *testing.T) {
	t.Log("TestFetchScalars")
	if os.Getenv("VIDGRIND_CREDENTIALS") == "" {
		t.Skipf("VIDGRIND_CREDENTIALS missing")
	}

	// Create a request.
	q := fmt.Sprintf("/data/%d?ma=%s&pn=%s&ds=%d&df=%d&do=csv&tz=0", siteKey, mac, pin, iotds.EpochStart, iotds.EpochStart+minutesInDay*60)
	req, err := http.NewRequest("GET", q, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new Recorder to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(dataHandler)

	// Invoke the request.
	handler.ServeHTTP(rr, req)

	// Check the response status code is OK.
	status := rr.Code
	if status != http.StatusOK {
		t.Errorf("dataHandler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the response body is as expected.
	reader := csv.NewReader(rr.Body)
	data, err := reader.ReadAll()
	if err != nil {
		t.Errorf("Error parsing CSV: %v", err)
		return
	}
	if len(data) != minutesInDay {
		t.Errorf("Expected %d CSV tuples, got %d", minutesInDay*60, len(data))
	}

	samples := sampleSinusoid(amplitude, offset, minutesInDay)
	ts := int64(iotds.EpochStart)
	for i := 0; i < minutesInDay; i++ {
		d, err := time.Parse(dateFmt, data[i][0])
		if err != nil {
			t.Errorf("#%d: error parsing date: %v", i, err)
		}
		if d.Unix() != ts {
			t.Errorf("#%d: expected timestamp of %d, got %d", i, samples[i], d.Unix())
		}
		v, err := strconv.ParseInt(data[i][1], 10, 64)
		if err != nil {
			t.Errorf("#%d: error parsing value: %v", i, err)
		}
		if v != samples[i] {
			t.Errorf("#%d: expected value of %d, got %d", i, samples[i], v)
		}
		ts += 60
	}
}

// sampleSinusoid samples a sinusoid n times.
func sampleSinusoid(amplitude, offset float64, n int) []int64 {
	samples := make([]int64, n)
	for i := 0; i < n; i++ {
		angle := 2 * math.Pi * float64(i) / float64(n)
		samples[i] = int64(offset + amplitude*math.Sin(angle))
	}
	return samples
}
