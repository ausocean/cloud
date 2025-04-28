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

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
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
	model.RegisterEntities()
}

// TestPutScalars creates one day's worth of test data at one-minute intervals
// starting from datastore.EpochStart (i.e., timestamps 1483228800 to 1483315200).
func TestPutScalars(t *testing.T) {
	t.Log("TestPutScalars")
	if os.Getenv("VIDGRIND_CREDENTIALS") == "" {
		t.Skipf("VIDGRIND_CREDENTIALS missing")
	}

	ctx := context.Background()
	store, err := datastore.NewStore(ctx, "cloud", "vidgrind", "")
	if err != nil {
		t.Fatalf("NewStore failed with error: %v", err)
	}

	id := model.ToSID(mac, pin)

	// Clean up any existing scalars before starting.
	err = model.DeleteScalars(ctx, store, id)
	if err != nil {
		t.Fatalf("DeleteScalars before test failed with error: %v", err)
	}

	// Schedule a clean up after the test ends.
	t.Cleanup(func() {
		err := model.DeleteScalars(ctx, store, id)
		if err != nil {
			t.Errorf("cleanup: DeleteScalars failed with error: %v", err)
		}
	})

	ts := int64(datastore.EpochStart)
	samples := sampleSinusoid(amplitude, offset, minutesInDay)
	for i := 0; i < minutesInDay; i++ {
		err := model.PutScalar(ctx, store, &model.Scalar{
			ID:        id,
			Timestamp: ts,
			Value:     float64(samples[i]),
		})
		if err != nil {
			t.Errorf("PutScalar failed with error: %v", err)
		}
		ts += 60

		// Print progress every 10%.
		if i%(minutesInDay/10) == 0 {
			t.Logf("put %d/%d scalars", i, minutesInDay)
		}
	}
}

// TestGetScalars tests retrieving scalars directly from the datastore.
func TestGetScalars(t *testing.T) {
	t.Log("TestGetScalars")
	if os.Getenv("VIDGRIND_CREDENTIALS") == "" {
		t.Skipf("VIDGRIND_CREDENTIALS missing")
	}
	ctx := context.Background()
	store, err := datastore.NewStore(ctx, "cloud", "vidgrind", "")
	if err != nil {
		t.Fatalf("NewStore failed with error: %v", err)
	}

	id := model.ToSID(mac, pin)

	t.Log("deleting existing scalars before test")
	err = model.DeleteScalars(ctx, store, id)
	if err != nil {
		t.Fatalf("DeleteScalars before test failed with error: %v", err)
	}

	// Schedule a clean up after the test finishes.
	t.Cleanup(func() {
		t.Log("cleaning up scalars after test")
		err := model.DeleteScalars(ctx, store, id)
		if err != nil {
			t.Errorf("DeleteScalars after test failed with error: %v", err)
		}
	})

	t.Log("inserting fresh scalars for test")
	ts := int64(datastore.EpochStart)
	samples := sampleSinusoid(amplitude, offset, minutesInDay)
	for i := 0; i < minutesInDay; i++ {
		err := model.PutScalar(ctx, store, &model.Scalar{
			ID:        id,
			Timestamp: ts,
			Value:     float64(samples[i]),
		})
		if err != nil {
			t.Fatalf("PutScalar failed with error: %v", err)
		}
		ts += 60

		// Print progress every 10%.
		if i%(minutesInDay/10) == 0 {
			t.Logf("put %d/%d scalars", i, minutesInDay)
		}
	}

	t.Log("fetching scalars from datastore")
	scalars, err := model.GetScalars(ctx, store, id, []int64{datastore.EpochStart, datastore.EpochStart + minutesInDay*60})
	if err != nil {
		t.Fatalf("GetScalars failed with error: %v", err)
	}
	if len(scalars) != minutesInDay {
		t.Fatalf("Expected %d scalars, got %d", minutesInDay, len(scalars))
	}

	t.Log("verifying retrieved scalar values")
	ts = int64(datastore.EpochStart)
	for i := 0; i < minutesInDay; i++ {
		if int64(scalars[i].Value) != samples[i] {
			t.Errorf("#%d: expected %d, got %f", i, samples[i], scalars[i].Value)
		}
		ts += 60
	}
}

// TestFetchScalars tests fetching scalars via Ocean Bench's /data endpoint.
func TestFetchScalars(t *testing.T) {
	if os.Getenv("VIDGRIND_CREDENTIALS") == "" {
		t.Skipf("VIDGRIND_CREDENTIALS missing.")
	}

	ctx := context.Background()
	store, err := datastore.NewStore(ctx, "cloud", "vidgrind", "")
	if err != nil {
		t.Fatalf("NewStore failed with error: %v", err)
	}

	id := model.ToSID(mac, pin)

	t.Log("deleting existing scalars before test.")
	err = model.DeleteScalars(ctx, store, id)
	if err != nil {
		t.Fatalf("DeleteScalars before test failed with error: %v", err)
	}

	t.Cleanup(func() {
		t.Log("cleaning up scalars after test.")
		err := model.DeleteScalars(ctx, store, id)
		if err != nil {
			t.Errorf("DeleteScalars after test failed with error: %v", err)
		}
	})

	t.Log("inserting fresh scalars for test.")
	ts := int64(datastore.EpochStart)
	samples := sampleSinusoid(amplitude, offset, minutesInDay)
	for i := 0; i < minutesInDay; i++ {
		err := model.PutScalar(ctx, store, &model.Scalar{
			ID:        id,
			Timestamp: ts,
			Value:     float64(samples[i]),
		})
		if err != nil {
			t.Fatalf("PutScalar failed with error: %v", err)
		}
		ts += 60

		if i%(minutesInDay/10) == 0 {
			t.Logf("inserted %d/%d scalars", i, minutesInDay)
		}
	}

	t.Log("creating request to /data endpoint")
	q := fmt.Sprintf("/data/%d?ma=%s&pn=%s&ds=%d&df=%d&do=csv&tz=0", siteKey, mac, pin, datastore.EpochStart, datastore.EpochStart+minutesInDay*60)
	req, err := http.NewRequest("GET", q, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(dataHandler)

	t.Log("invoking handler")
	handler.ServeHTTP(rr, req)

	status := rr.Code
	if status != http.StatusOK {
		t.Errorf("dataHandler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	t.Log("parsing response CSV")
	reader := csv.NewReader(rr.Body)
	data, err := reader.ReadAll()
	if err != nil {
		t.Errorf("error parsing CSV: %v", err)
		return
	}

	if len(data) != minutesInDay {
		t.Errorf("expected %d CSV tuples, got %d", minutesInDay, len(data))
	}

	t.Log("verifying CSV content")
	ts = int64(datastore.EpochStart)
	for i := 0; i < minutesInDay; i++ {
		d, err := time.Parse(dateFmt, data[i][0])
		if err != nil {
			t.Errorf("#%d: error parsing date: %v", i, err)
		}
		if d.Unix() != ts {
			t.Errorf("#%d: expected timestamp of %d, got %d", i, ts, d.Unix())
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
