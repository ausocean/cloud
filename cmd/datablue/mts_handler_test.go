/*
LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

  This file is part of Data Blue. This is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Data Blue is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with Data Blue in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/stretchr/testify/assert"
)

const (
	testSiteKey         = 1
	testSiteName        = "Test Site"
	testDevMac1         = "00:00:00:00:00:01"
	testDevMa1          = 1
	testDevDkey1        = 10000001
	testDevDkeyStr1     = "10000001"
	testDevMac2         = "00:00:00:00:00:02"
	testDevMa2          = 2
	testDevDkey2        = 10000002
	testDevDkeyStr2     = "10000002"
	testUnknownMac      = "00:00:00:00:00:99"
	testInvalidDkey     = "99999999"
	testGeohash         = "r1f9652gs"
	testTimestamp       = int64(1600000000)
	testLat             = -34.91805
	testLng             = 138.60475
	testLLStr           = "-34.91805,138.60475"
	testBroadcastID     = "12345678-abcd-1234-abcd-1234567890ab"
	testVideoStorageURI = "gs://bucket/video.ts"
	testAudioStorageURI = "gs://bucket/audio.ts"
	testVideoType       = "video/mp2t"
	testAudioType       = "audio/aac"
	testDuration        = 3000
	testPTS             = 80000
)

func setupTestStore(t *testing.T) context.Context {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := datastore.NewStore(ctx, "file", "vidgrind", tempDir)
	assert.NoError(t, err)

	model.RegisterEntities()
	settingsStore = store
	mediaStore = store

	// Insert test site
	site := &model.Site{
		Skey:    testSiteKey,
		Name:    testSiteName,
		Enabled: true,
	}
	if err := model.PutSite(ctx, settingsStore, site); err != nil {
		t.Fatalf("failed to put test site: %v", err)
	}

	// Insert test device 1 (without location)
	dev1 := &model.Device{
		Skey:          testSiteKey,
		Mac:           testDevMa1,
		Dkey:          testDevDkey1,
		Name:          "Test Device 1",
		Inputs:        "V0,S0",
		MonitorPeriod: 60,
		Enabled:       true,
	}
	err = model.PutDevice(ctx, settingsStore, dev1)
	assert.NoError(t, err)

	// Insert test device 2 (with location)
	dev2 := &model.Device{
		Skey:          testSiteKey,
		Mac:           testDevMa2,
		Dkey:          testDevDkey2,
		Name:          "Test Device 2",
		Inputs:        "V0",
		Latitude:      testLat,
		Longitude:     testLng,
		MonitorPeriod: 60,
		Enabled:       true,
	}
	err = model.PutDevice(ctx, settingsStore, dev2)
	assert.NoError(t, err)

	return ctx
}

// TestMTSHandlerV2_Success tests that the mtsHandlerV2 function correctly
// handles a valid request with a single pin.
func TestMTSHandlerV2_Success(t *testing.T) {
	ctx := setupTestStore(t)

	meta := model.MtsMediaV2{
		Duration:      testDuration,
		StorageURI:    testVideoStorageURI,
		BroadcastID:   testBroadcastID,
		Discontinuity: true,
		PTS:           testPTS,
		Type:          testVideoType,
	}
	bodyBytes, err := json.Marshal(meta)
	assert.NoError(t, err)

	reqURL := fmt.Sprintf("/mtsv2?ma=%s&dk=%s&gh=%s&ts=%d&V0=%d", testDevMac1, testDevDkeyStr1, testGeohash, testTimestamp, len(bodyBytes))
	req := httptest.NewRequest(http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	mtsHandlerV2(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode, "expected status OK, got %v", res.Status)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	// Check the response has the correct values
	assert.Equal(t, testDevMac1, resp["ma"], "expected ma %s, got %v", testDevMac1, resp["ma"])
	assert.Equal(t, testTimestamp, int64(resp["ts"].(float64)), "expected ts %d, got %v", testTimestamp, resp["ts"])
	assert.Equal(t, len(bodyBytes), int(resp["V0"].(float64)), "expected V0 size %d, got %v", len(bodyBytes), resp["V0"])
	assert.Nil(t, resp["er"], "unexpected error in response: %v", resp["er"])

	mid := model.ToMID(testDevMac1, "V0")
	stored, err := model.GetMtsMediaV2(ctx, mediaStore, mid, testTimestamp)
	assert.NoError(t, err)

	// Check the stored entity has the correct values
	assert.Equal(t, mid, stored.MID, "expected MID %d, got %d", mid, stored.MID)
	assert.Equal(t, testGeohash, stored.Geohash, "expected Geohash %s, got %s", testGeohash, stored.Geohash)
	assert.Equal(t, testTimestamp, stored.Timestamp, "expected Timestamp %d, got %d", testTimestamp, stored.Timestamp)
	assert.Equal(t, meta.Duration, stored.Duration, "expected Duration %d, got %d", meta.Duration, stored.Duration)
	assert.Equal(t, meta.StorageURI, stored.StorageURI, "expected StorageURI %s, got %s", meta.StorageURI, stored.StorageURI)
	assert.Equal(t, meta.BroadcastID, stored.BroadcastID, "expected BroadcastID %s, got %s", meta.BroadcastID, stored.BroadcastID)
	assert.Equal(t, meta.Discontinuity, stored.Discontinuity, "expected Discontinuity %v, got %v", meta.Discontinuity, stored.Discontinuity)
	assert.Equal(t, meta.PTS, stored.PTS, "expected PTS %d, got %d", meta.PTS, stored.PTS)
	assert.Equal(t, meta.Type, stored.Type, "expected Type %s, got %s", meta.Type, stored.Type)
}

// TestMTSHandlerV2_DeviceLocation tests that the mtsHandlerV2 function
// includes the device location (ll) in the response when the device
// has a location set.
func TestMTSHandlerV2_DeviceLocation(t *testing.T) {
	_ = setupTestStore(t)

	meta := model.MtsMediaV2{
		Duration:   2000,
		StorageURI: "gs://bucket/loc.ts",
	}
	bodyBytes, err := json.Marshal(meta)
	assert.NoError(t, err)

	reqURL := fmt.Sprintf("/mtsv2?ma=%s&dk=%s&ts=%d&V0=%d", testDevMac2, testDevDkeyStr2, testTimestamp, len(bodyBytes))
	req := httptest.NewRequest(http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	mtsHandlerV2(w, req)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	// Check the response has the correct location
	assert.Equal(t, testLLStr, resp["ll"], "expected ll %s, got %v", testLLStr, resp["ll"])
}

// TestMTSHandlerV2_MultiplePins tests that the mtsHandlerV2 function
// correctly handles a valid request with multiple pins.
func TestMTSHandlerV2_MultiplePins(t *testing.T) {
	ctx := setupTestStore(t)

	metaV0 := model.MtsMediaV2{
		Duration:   testDuration,
		StorageURI: testVideoStorageURI,
		Type:       testVideoType,
	}
	metaS0 := model.MtsMediaV2{
		Duration:   testDuration,
		StorageURI: testAudioStorageURI,
		Type:       testAudioType,
	}

	bV0, _ := json.Marshal(metaV0)
	bS0, _ := json.Marshal(metaS0)

	combinedBody := append(bV0, bS0...)

	reqURL := fmt.Sprintf("/mtsv2?ma=%s&dk=%s&ts=%d&V0=%d&S0=%d", testDevMac1, testDevDkeyStr1, testTimestamp, len(bV0), len(bS0))
	req := httptest.NewRequest(http.MethodPost, reqURL, bytes.NewReader(combinedBody))
	w := httptest.NewRecorder()

	mtsHandlerV2(w, req)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	// Check that the response has the correct values
	assert.Nil(t, resp["er"])
	assert.Equal(t, testDevMac1, resp["ma"])
	assert.Equal(t, testTimestamp, int64(resp["ts"].(float64)))
	assert.Equal(t, len(bV0), int(resp["V0"].(float64)))
	assert.Equal(t, len(bS0), int(resp["S0"].(float64)))

	// Check that the V0 clip was stored correctly
	midV0 := model.ToMID(testDevMac1, "V0")
	storedV0, err := model.GetMtsMediaV2(ctx, mediaStore, midV0, testTimestamp)
	assert.NoError(t, err)
	assert.Equal(t, metaV0.StorageURI, storedV0.StorageURI)
	assert.Equal(t, metaV0.Type, storedV0.Type)
	assert.Equal(t, metaV0.Duration, storedV0.Duration)
	assert.Equal(t, metaV0.PTS, storedV0.PTS)
	assert.Equal(t, metaV0.Discontinuity, storedV0.Discontinuity)
	assert.Equal(t, metaV0.BroadcastID, storedV0.BroadcastID)

	// Check that the S0 clip was stored correctly
	midS0 := model.ToMID(testDevMac1, "S0")
	storedS0, err := model.GetMtsMediaV2(ctx, mediaStore, midS0, testTimestamp)
	assert.NoError(t, err)
	assert.Equal(t, metaS0.StorageURI, storedS0.StorageURI)
	assert.Equal(t, metaS0.Type, storedS0.Type)
	assert.Equal(t, metaS0.Duration, storedS0.Duration)
	assert.Equal(t, metaS0.PTS, storedS0.PTS)
	assert.Equal(t, metaS0.Discontinuity, storedS0.Discontinuity)
	assert.Equal(t, metaS0.BroadcastID, storedS0.BroadcastID)
}

// TestMTSHandlerV2_DefaultTimestamp tests that the mtsHandlerV2 function
// uses a default timestamp when no timestamp is provided.
func TestMTSHandlerV2_DefaultTimestamp(t *testing.T) {
	_ = setupTestStore(t)

	meta := model.MtsMediaV2{Duration: 1000}
	bodyBytes, _ := json.Marshal(meta)

	reqURL := fmt.Sprintf("/mtsv2?ma=%s&dk=%s&V0=%d", testDevMac1, testDevDkeyStr1, len(bodyBytes))
	req := httptest.NewRequest(http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	beforeTime := time.Now().Unix()
	mtsHandlerV2(w, req)
	afterTime := time.Now().Unix()

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	tsResp := int64(resp["ts"].(float64))
	// Check that a default timestamp was generated
	assert.True(t, tsResp >= beforeTime && tsResp <= afterTime, "expected default timestamp between %d and %d, got %d", beforeTime, afterTime, tsResp)
}

// TestMTSHandlerV2_InvalidDeviceKey tests that the mtsHandlerV2 function
// returns an error when an invalid device key is provided.
func TestMTSHandlerV2_InvalidDeviceKey(t *testing.T) {
	_ = setupTestStore(t)

	reqURL := fmt.Sprintf("/mtsv2?ma=%s&dk=%s&V0=10", testDevMac1, testInvalidDkey)
	req := httptest.NewRequest(http.MethodPost, reqURL, nil)
	w := httptest.NewRecorder()

	mtsHandlerV2(w, req)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	assert.Equal(t, resp["er"], model.ErrInvalidDeviceKey.Error())
	assert.Equal(t, int(resp["rc"].(float64)), model.DeviceStatusUpdate)
}

// TestMTSHandlerV2_UnknownDevice tests that the mtsHandlerV2 function
// returns an error when an unknown device is provided.
func TestMTSHandlerV2_UnknownDevice(t *testing.T) {
	_ = setupTestStore(t)

	reqURL := fmt.Sprintf("/mtsv2?ma=%s&dk=%s&V0=10", testUnknownMac, testDevDkeyStr1)
	req := httptest.NewRequest(http.MethodPost, reqURL, nil)
	w := httptest.NewRecorder()

	mtsHandlerV2(w, req)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	assert.Equal(t, resp["er"], datastore.ErrNoSuchEntity.Error())
}

// TestMTSHandlerV2_InvalidPinSize tests that the mtsHandlerV2 function
// returns an error when an invalid pin size is provided.
func TestMTSHandlerV2_InvalidPinSize(t *testing.T) {
	_ = setupTestStore(t)

	tests := []struct {
		name   string
		pinVal string
	}{
		{"negative size", "-5"},
		{"zero size", "0"},
		{"non-numeric size", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqURL := fmt.Sprintf("/mtsv2?ma=%s&dk=%s&V0=%s", testDevMac1, testDevDkeyStr1, tt.pinVal)
			req := httptest.NewRequest(http.MethodPost, reqURL, nil)
			w := httptest.NewRecorder()

			mtsHandlerV2(w, req)

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			assert.NoError(t, err)

			assert.Equal(t, resp["er"], errInvalidValue.Error())
		})
	}
}

// TestMTSHandlerV2_ShortBody tests that the mtsHandlerV2 function
// returns an error when a body smaller than the declared size is provided.
func TestMTSHandlerV2_ShortBody(t *testing.T) {
	ctx := setupTestStore(t)

	shortBody := []byte(`{"duration":1000}`)

	// Claiming V0 is 100 bytes, but body is smaller
	reqURL := fmt.Sprintf("/mtsv2?ma=%s&dk=%s&ts=%d&V0=100", testDevMac1, testDevDkeyStr1, testTimestamp)
	req := httptest.NewRequest(http.MethodPost, reqURL, bytes.NewReader(shortBody))
	w := httptest.NewRecorder()

	mtsHandlerV2(w, req)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	// Read body error breaks loop before clip unmarshal and entity creation
	mid := model.ToMID(testDevMac1, "V0")
	_, err = model.GetMtsMediaV2(ctx, mediaStore, mid, testTimestamp)
	assert.Error(t, err, "expected no MtsMediaV2 entity to be stored on short body read")
}

// TestMTSHandlerV2_InvalidJSONBody tests that the mtsHandlerV2 function
// returns an error when an invalid JSON body is provided.
func TestMTSHandlerV2_InvalidJSONBody(t *testing.T) {
	_ = setupTestStore(t)

	invalidJSON := []byte(`{not valid json!}`)

	reqURL := fmt.Sprintf("/mtsv2?ma=%s&dk=%s&V0=%d", testDevMac1, testDevDkeyStr1, len(invalidJSON))
	req := httptest.NewRequest(http.MethodPost, reqURL, bytes.NewReader(invalidJSON))
	w := httptest.NewRecorder()

	mtsHandlerV2(w, req)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	erStr, ok := resp["er"].(string)
	assert.True(t, ok, "expected string error message")
	assert.True(t, strings.HasPrefix(erStr, "could not unmarshal clip:"), "expected unmarshal error message")
}

// TestIsMtsPin tests the isMtsPin function which is used to determine
// if a pin is an MTS pin.
func TestIsMtsPin(t *testing.T) {
	tests := []struct {
		pin    string
		expect bool
	}{
		{"V0", true},
		{"V1", true},
		{"S0", true},
		{"S12", true},
		{"A0", false},
		{"", false},
		{"V", false},
		{"S", false},
		{"VABC", false},
	}

	for _, tt := range tests {
		t.Run(tt.pin, func(t *testing.T) {
			res := isMtsPin(tt.pin)
			assert.Equal(t, res, tt.expect, "isMtsPin(%q) = %v; want %v", tt.pin, res, tt.expect)
		})
	}
}
