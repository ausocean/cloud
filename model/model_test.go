/*
DESCRIPTION
  model tests.

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

package model

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ausocean/av/container/mts"
	"github.com/ausocean/av/container/mts/meta"
	"github.com/ausocean/av/container/mts/psi"
	"github.com/ausocean/openfish/datastore"
	"github.com/stretchr/testify/assert"
)

const (
	testSiteKey      = 1
	testSiteKey2     = 2
	testSiteName     = "OfficialTestSite"
	testSiteOrg      = "AusOcean"
	testSiteOps      = "ops@ausocean.org"
	testSiteLat      = -34.91805
	testSiteLng      = 138.60475
	testSiteTZ       = 9.5
	testSiteEnc      = `{"Skey":1,"Name":"OfficialTestSite","Description":"","OrgID":"AusOcean","OwnerEmail":"","OpsEmail":"ops@ausocean.org","YouTubeEmail":"","Latitude":-34.91805,"Longitude":138.60475,"Timezone":9.5,"NotifyPeriod":0,"Enabled":true,"Confirmed":false,"Premium":false,"Public":false,"Subscribed":"1970-01-01T00:00:00Z","Created":"1970-01-01T00:00:00Z"}`
	testDevMac       = "00:00:00:00:00:01"
	testDevMa        = 1
	testMID          = testDevMa << 4
	testDevDkey      = 10000001
	testDevID        = "TestDevice"
	testDevInputs    = "A0,V0"
	testDevEnc       = "1\t10000001\t1\tTestDevice\tX0,V0\t\t"
	testDevEnc2      = "1\t10000001\t1\tTestDevice\t\t\t"
	testDevEnc3      = "1\t10000001\t1\tTestDevice\tX0,V0\t\t\t0\t0\t0\ttag\tprotocol\t-34.918050\t138.604750\ttrue\t"
	testUserEmail    = "test@ausocean.org"
	testUserEmail2   = "test@testdomain.org"
	testUserPerm     = ReadPermission
	testUserPerm2    = ReadPermission | WritePermission
	testUserTime     = 1572157457
	testUserTime2    = 1572157475
	testUserEnc      = "1\ttest@ausocean.org\t\t1\t1572157457"
	testUserEnc2     = "2\ttest@ausocean.org\t\t3\t1572157475"
	testDevMac2      = "00:00:00:00:00:0F"
	testDevMac3      = "1A:2B:3C:4F:50:61"
	testDevMa2       = 15
	testMID2         = testDevMa2 << 4
	testMIDAll       = 0
	testDevPin       = "V0"
	testDevPin2      = "S1"
	testMetadata     = "loc:-34.91805,138.60475"
	testTimestamp    = datastore.EpochStart
	testGeohash      = "r1f9652gs"
	testTextMID      = (testDevMa << 4) | 0x08
	testDomain       = "@ausocean.org"
	testDomain2      = "@testdomain.org"
	testOtherUser    = "other@ausocean.org"
	testJunkUser     = "someone@junk.com"
	anyDomain        = "@"
	testCronEnc      = "1\tTest\t0\tSunrise\tfalse\t0\tset\tPower\toff\tfalse"
	testSubscriberID = 1234567890
	testFeedID       = 9876543210
)

var testTime = time.Unix(0, 0).UTC()

// TestEncoding tests various encoding and decoding functions.
func TestEncoding(t *testing.T) {
	// Test IsMacAddress
	if !IsMacAddress(testDevMac) {
		t.Errorf("IsMacAddress(%s) failed: expected true, got false", testDevMac)
	}
	if IsMacAddress("00:00:00:00:00:00") {
		t.Errorf("IsMacAddress(00:00:00:00:00:00) failed: expected false, got true")
	}
	if IsMacAddress("00:00:00:00:00:0G") {
		t.Errorf("IsMacAddress(00:00:00:00:0G) failed: expected false, got true")
	}
	if IsMacAddress("00:00:00:00:01") {
		t.Errorf("IsMacAddress(00:00:00:00:01) failed: expected false, got true")
	}
	if IsMacAddress("") {
		t.Errorf("IsMacAddress() failed: expected false, got true")
	}

	// MAC address encoding/decoding.
	ma := MacEncode(testDevMac)
	if ma != testDevMa {
		t.Errorf("MacEncode failed: expected %d, got %d", testDevMa, ma)
	}
	mac := MacDecode(testDevMa)
	if mac != testDevMac {
		t.Errorf("MacDecode failed: expected %s, got %s", testDevMac, mac)
	}
	ma = MacEncode(testDevMac3)
	mac3 := strings.ReplaceAll(testDevMac3, ":", "")
	ma2 := MacEncode(mac3)
	if ma != ma2 {
		t.Errorf("MacEncode failed: expected %d, got %d", ma, ma2)
	}
	mac = MacDecode(ma)
	if mac != testDevMac3 {
		t.Errorf("MacDecode failed: expected %s, got %s", testDevMac3, mac)
	}

	// MID encoding/ecoding
	mid := ToMID(testDevMac, testDevPin2)
	expect := testDevMa<<4 | int64(putMtsPin(testDevPin2))
	if mid != expect {
		t.Errorf("ToMID failed: expected %d, got %d", expect, mid)
	}
	mac, pin := FromMID(mid)
	if mac != testDevMac && pin != testDevPin2 {
		t.Errorf("FromMID failed: expected %s,%s, got %s,%s", testDevMac, testDevPin2, mac, pin)
	}

	// Device encoding/decoding.
	dev := Device{Skey: testSiteKey, Dkey: testDevDkey, Mac: 1, Name: "TestDevice", Inputs: "X0,V0", Enabled: true}
	enc := dev.Encode()
	if !strings.HasPrefix(string(enc), testDevEnc) {
		t.Errorf("Device.Encode failed: expected %s, got %s", testDevEnc, enc)
	}
	var dev2 Device
	err := dev2.Decode(enc)
	if err != nil {
		t.Errorf("Device.Decode failed with error: %s", err)
	}
	enc2 := dev2.Encode()
	if !strings.HasPrefix(string(enc2), testDevEnc) {
		t.Errorf("Device.Encode 2 failed: expected %s, got %s", testDevEnc, enc2)
	}

	dev2.Inputs = ""
	enc2 = dev2.Encode()
	if !strings.HasPrefix(string(enc2), testDevEnc2) {
		t.Errorf("Device.Decode failed: expected %s, got %s", testDevEnc2, enc2)
	}
	var dev3 Device
	err = dev3.Decode(enc2)
	if err != nil {
		t.Errorf("Device.Decode failed with error: %s", err)
	}
	if dev3.Inputs != "" {
		t.Errorf("Device.Decode returned non-empty Inputs")
	}

	// Site encoding/decoding.
	site := Site{Skey: testSiteKey, Name: testSiteName, OrgID: testSiteOrg, OpsEmail: testSiteOps, Latitude: testSiteLat, Longitude: testSiteLng, Timezone: testSiteTZ, Enabled: true, Subscribed: testTime, Created: testTime}
	enc = site.Encode()
	if string(enc) != testSiteEnc {
		t.Errorf("Site.Encode failed: expected %s, got %s", testSiteEnc, enc)
	}
	var site2 Site
	err = site2.Decode(enc)
	if err != nil {
		t.Errorf("Site.Decode failed with error: %s", err)
	}
	enc2 = site2.Encode()
	if string(enc2) != testSiteEnc {
		t.Errorf("Site.Encode 2 failed: expected %s, got %s", testSiteEnc, enc2)
	}

	// User encoding/decoding.
	user := User{Skey: testSiteKey, Email: testUserEmail, Perm: testUserPerm, Created: time.Unix(testUserTime, 0)}
	enc = user.Encode()
	if string(enc) != testUserEnc {
		t.Errorf("User.Encode failed: expected %s, got %s", testUserEnc, enc)
	}

	var user2 User
	err = user2.Decode(enc)
	if err != nil {
		t.Errorf("User.Decode failed with error: %s", err)
	}
	enc2 = user2.Encode()
	if string(enc2) != testUserEnc {
		t.Errorf("User.Encode 2 failed: expected %s, got %s", testUserEnc, enc2)
	}

	// MtsMedia encoding/decoding.
	m := MtsMedia{MID: testMID, Geohash: testGeohash, Timestamp: testTimestamp, PTS: 1, Duration: 2, Metadata: testMetadata, Clip: []byte{'A', 'B', 'C', 'D', 'E', 'F', 'G'}}
	enc = m.Encode()
	var m2 MtsMedia
	err = m2.Decode(enc)
	if err != nil {
		t.Errorf("MtsMedia.Decode failed with error: %s", err)
	}
	if m2.MID != testMID || m2.Geohash != testGeohash || m2.Timestamp != testTimestamp || m2.Metadata != testMetadata {
		t.Errorf("MtsMedia.Decode failed to decode correctly")
	}
	enc2 = m2.Encode()
	if string(enc) != string(enc2) {
		t.Errorf("MtsMedia.Encode 2 failed")
	}

	// Test MtsMedia:ID()
	tests := []struct {
		input MtsMedia
		count int64
		want  int64
	}{
		{
			input: MtsMedia{MID: 0, Timestamp: datastore.EpochStart},
			count: 0,
			want:  0,
		},
		{
			input: MtsMedia{MID: 0, Timestamp: datastore.EpochStart - 1},
			count: 0,
			want:  0,
		},
		{
			input: MtsMedia{MID: testMID, Timestamp: datastore.EpochStart + 1},
			count: 0,
			want:  testMID<<32 | 1<<3,
		},
		{
			input: MtsMedia{MID: testMID, Timestamp: datastore.EpochStart + 1},
			count: 1,
			want:  testMID<<32 | 1<<3 | 1,
		},
		{
			input: MtsMedia{MID: testMID + 1, Timestamp: datastore.EpochStart + 1},
			count: 0,
			want:  (testMID+1)<<32 | 1<<3,
		},
	}

	for i, test := range tests {
		id := datastore.IDKey(test.input.MID, test.input.Timestamp, test.count)
		if id != test.want {
			t.Errorf("IDKey %d failed: expected %d, got %d", i, test.want, id)
		}
	}
}

// TestFragmentMTSMedia tests the FragmentMTSMedia funtion.
func TestFragmentMTSMedia(t *testing.T) {
	const u = mts.PTSFrequency // MTS Frequency units.
	const ptsTolerance = 12000 // 133ms (this is the tolerance that vidgrind uses)

	type frag struct {
		len  int   // How long the MTSFragment's media slice should be.
		dur  int64 // Desired duration of fragment.
		cont bool  // Desired continues flag.
	}

	tests := []struct {
		input  [][2]int64
		period int64
		want   []frag
	}{
		// Whole second durations, 5s period.
		{
			input:  [][2]int64{{0, 1}, {1, 1}, {2, 1}, {3, 1}, {4, 1}, {5, 1}, {6, 1}, {7, 1}, {8, 1}, {9, 1}, {10, 1}, {11, 1}, {12, 1}, {13, 1}, {14, 1}},
			period: 5,
			want:   []frag{{5, 5 * u, true}, {5, 5 * u, true}, {5, 5 * u, true}},
		},
		{
			input:  [][2]int64{{1, 1}, {2, 1}, {3, 1}, {4, 1}, {7, 1}, {8, 1}, {9, 1}, {10, 1}, {11, 1}, {13, 1}, {14, 1}},
			period: 5,
			want:   []frag{{4, 4 * u, true}, {5, 5 * u, false}, {2, 2 * u, false}},
		},
		{
			input:  [][2]int64{{1, 1}, {2, 1}, {3, 1}, {13, 1}, {14, 1}},
			period: 5,
			want:   []frag{{3, 3 * u, true}, {2, 2 * u, false}},
		},
		// Ten-second durations, 30-second period.
		{
			input:  [][2]int64{{0, 10}, {10, 10}, {20, 10}, {30, 10}, {40, 10}, {50, 10}, {60, 10}, {70, 10}, {80, 10}, {90, 10}},
			period: 30,
			want:   []frag{{3, 30 * u, true}, {3, 30 * u, true}, {3, 30 * u, true}, {1, 10 * u, true}},
		},
		{
			input:  [][2]int64{{0, 10}, {10, 10}, {20, 10}, {130, 10}, {140, 10}},
			period: 30,
			want:   []frag{{3, 30 * u, true}, {2, 20 * u, false}},
		},
	}

	// Messy real-world data with missing timestamps (from Big_buck_bunny_01.ts).
	realInput := []MtsMedia{
		{Timestamp: 11, PTS: 0, Duration: 0, Continues: true},
		{Timestamp: 11, PTS: 0, Duration: 688501, Continues: true},
		{Timestamp: 16, PTS: 688501, Duration: 213000, Continues: true},
		{Timestamp: 21, PTS: 901501, Duration: 189001, Continues: true},
		{Timestamp: 22, PTS: 1090502, Duration: 213001, Continues: true},
		{Timestamp: 25, PTS: 1303503, Duration: 226500, Continues: true},
		{Timestamp: 26, PTS: 1530003, Duration: 225001, Continues: true},
		{Timestamp: 28, PTS: 1755004, Duration: 55500, Continues: true},
		{Timestamp: 30, PTS: 1810504, Duration: 454501, Continues: true},
		{Timestamp: 35, PTS: 2265005, Duration: 351000, Continues: true},
		{Timestamp: 30, PTS: 2616005, Duration: 94501, Continues: true},
		{Timestamp: 41, PTS: 2710506, Duration: 166501, Continues: true},
		{Timestamp: 41, PTS: 2877007, Duration: 193501, Continues: true},
		{Timestamp: 43, PTS: 3070508, Duration: 199501, Continues: true},
		{Timestamp: 47, PTS: 3270009, Duration: 211500, Continues: true},
		{Timestamp: 40, PTS: 3481509, Duration: 124500, Continues: true},
	}

	// These tests use the MTS media values extracted from Big_buck_bunny_01.ts.
	tests2 := []struct {
		input  []MtsMedia
		period int64
		want   []frag
	}{
		{
			input:  realInput,
			period: 10,
			want:   []frag{{3, 901501, true}, {5, 909003, true}, {3, 900002, true}, {5, 895503, true}},
		},
		{
			input:  realInput,
			period: 20,
			want:   []frag{{8, 1810504, true}, {8, 1795505, true}},
		},
		{
			input:  realInput,
			period: 40,
			want:   []frag{{16, 3606009, true}},
		},
	}

	for i, test := range tests {
		in := generateTestMtsMedia(test.input)
		out := FragmentMTSMedia(in, test.period, 0)
		if len(out) != len(test.want) {
			t.Fatalf("FragmentMTSMedia test %d failed, expected %d fragments, got %d", i, len(test.want), len(out))
		}

		for j, f := range out {
			if len(f.Medias) != test.want[j].len {
				t.Errorf("FragmentMTSMedia test %d failed; expected frag %v to contain %v MtsMedias, got %v", i, j, test.want[j].len, len(f.Medias))
			}
			if f.Duration != test.want[j].dur {
				t.Errorf("FragmentMTSMedia test %d failed; expected frag %v duration: %v, got %v", i, j, test.want[j].dur, f.Duration)
			}
			if f.Continues != test.want[j].cont {
				t.Errorf("FragmentMTSMedia test %d failed; expected continues flag for frag %v to be set to: %v, got %v", i, j, test.want[j].cont, f.Continues)
			}
		}
	}

	for i, test := range tests2 {
		out := FragmentMTSMedia(test.input, test.period, ptsTolerance)
		if len(out) != len(test.want) {
			t.Fatalf("FragmentMTSMedia test %d failed, expected %d fragments, got %d", i, len(test.want), len(out))
		}

		for j, f := range out {
			if len(f.Medias) != test.want[j].len {
				t.Errorf("FragmentMTSMedia test %d failed; expected frag %v to contain %v MtsMedias, got %v", i, j, test.want[j].len, len(f.Medias))
			}
			if f.Duration != test.want[j].dur {
				t.Errorf("FragmentMTSMedia test %d failed; expected frag %v duration: %v, got %v", i, j, test.want[j].dur, f.Duration)
			}
			if f.Continues != test.want[j].cont {
				t.Errorf("FragmentMTSMedia test %d failed; expected continues flag for frag %v to be set to: %v, got %v", i, j, test.want[j].cont, f.Continues)
			}
		}
	}

	// Empty case.
	out := FragmentMTSMedia([]MtsMedia{}, 10, 0)
	if len(out) != 0 {
		t.Errorf("FragmentMTSMedia emtpy case failed")
	}
}

// generateTestMtsMedia generates dummy []MtsMedia from timestamps and durations.
func generateTestMtsMedia(in [][2]int64) (out []MtsMedia) {

	for _, i := range in {
		out = append(out, MtsMedia{Timestamp: i[0], PTS: i[0] * mts.PTSFrequency, Duration: i[1] * mts.PTSFrequency, Continues: true})
	}
	return
}

// init registers our datastore entities.
func init() {
	RegisterEntities()
}

// TestNetreceiverAccess tests access to NetReceiver's datastore.
func TestNetreceiverFileAccess(t *testing.T) {
	testEntities(t, "file")
	testDevice(t, "file")
	testVariable(t, "file")
	testCron(t, "file")
	testSubscriber(t, "file")
	testSubscription(t, "file")
}

func TestNetreceiverCloudAccess(t *testing.T) {
	if os.Getenv("NETRECEIVER_CREDENTIALS") == "" {
		t.Skip("NETRECEIVER_CREDENTIALS required to access NetReceiver datastore")
	}
	testEntities(t, "cloud")
	testDevice(t, "cloud")
	testVariable(t, "cloud")
	testCron(t, "cloud")
	testSubscriber(t, "cloud")
	testSubscription(t, "cloud")
}

// testEntities tests access to various entities in NetReceiver's datastore.
func testEntities(t *testing.T, kind string) {
	ctx := context.Background()

	store, err := datastore.NewStore(ctx, kind, "netreceiver", "")
	if err != nil {
		t.Errorf("datastore.NewStore(%s, netreceiver) failed with error: %v", kind, err)
	}

	// If we're testing FileStore we first need to create some entities.
	var site Site
	if kind == "file" {
		// Create a site.
		site = Site{Skey: testSiteKey, Name: testSiteName, OrgID: testSiteOrg, OpsEmail: testSiteOps, Latitude: testSiteLat, Longitude: testSiteLng, Timezone: testSiteTZ, Enabled: true, Subscribed: testTime, Created: testTime}
		err = PutSite(ctx, store, &site)
		if err != nil {
			t.Errorf("store.Put(Site) failed with error: %v", err)
		}
		// Create a user of 2 sites.
		err = PutUser(ctx, store, &User{Skey: testSiteKey, Email: testUserEmail, Perm: testUserPerm, Created: time.Unix(testUserTime, 0)})
		if err != nil {
			t.Errorf("store.Put(User) #1 failed with error: %v", err)
		}
		err = PutUser(ctx, store, &User{Skey: testSiteKey2, Email: testUserEmail, Perm: testUserPerm2, Created: time.Unix(testUserTime2, 0)})
		if err != nil {
			t.Errorf("store.Put(User) #2 failed with error: %v", err)
		}
	}

	s, err := GetSite(ctx, store, testSiteKey)
	if err != nil {
		t.Errorf("GetSite failed with error: %v", err)
	}
	enc := string(s.Encode())
	if enc != testSiteEnc {
		t.Errorf("GetSite failed: expected %s, got %s", testSiteEnc, enc)
	}

	if kind == "file" {
		allSites, err := GetAllSites(ctx, store)
		if err != nil {
			t.Errorf("unexpected error getting all sites: %v", err)
		}
		if len(allSites) != 1 || allSites[0] != site {
			t.Errorf("unexpected result from GetAllSites:\ngot: %#v\nwant:%#v",
				allSites, site)
		}
	}

	user, err := GetUser(ctx, store, testSiteKey, testUserEmail)
	if err != nil {
		t.Errorf("GetUser failed with error: %v", err)
	}
	enc = string(user.Encode())
	if enc != testUserEnc {
		t.Errorf("GetUser failed: expected %s, got %s", testUserEnc, enc)
	}

	// Attempt to get a user that does not exist.
	_, err = GetUser(ctx, store, testSiteKey, testJunkUser)
	if err != datastore.ErrNoSuchEntity {
		t.Errorf("GetUser failed to return ErrNoSuchdatastore.Entity error")
	}
}

// testDevice tests Device methods.
func testDevice(t *testing.T, kind string) {
	ctx := context.Background()

	store, err := datastore.NewStore(ctx, kind, "netreceiver", "")
	if err != nil {
		t.Errorf("datastore.NewStore(%s, netreceiver) failed with error: %v", kind, err)
	}

	dev := &Device{Skey: testSiteKey, Dkey: testDevDkey, Mac: testDevMa, Name: testDevID, Inputs: testDevInputs, Enabled: true}
	err = PutDevice(ctx, store, dev)
	if err != nil {
		t.Errorf("PutDevice failed with error: %v", err)
	}
	dev, err = GetDevice(ctx, store, testDevMa)
	if err != nil {
		t.Errorf("GetDevice failed with error: %v", err)
	}
	if dev.Skey != testSiteKey || dev.Dkey != testDevDkey || dev.Inputs != testDevInputs || !dev.Enabled {
		t.Errorf("GetDevice returned wrong values; got %v", dev)
	}

	// Test checking
	_, err = CheckDevice(ctx, store, testDevMac, strconv.Itoa(testDevDkey))
	if err != nil {
		t.Errorf("checkDevice failed with error: %v", err)
	}

	// Test deletion.
	err = DeleteDevice(ctx, store, testDevMa)
	if err != nil {
		t.Errorf("DeleteDevice failed with error: %v", err)
	}
	dev, err = GetDevice(ctx, store, testDevMa)
	if err == nil {
		t.Errorf("GetDevice failed to fail")
	}
}

// testVariable tests Variable methods.
func testVariable(t *testing.T, kind string) {
	ctx := context.Background()

	store, err := datastore.NewStore(ctx, kind, "netreceiver", "")
	if err != nil {
		t.Errorf("datastore.NewStore(%s, netreceiver) failed with error: %v", kind, err)
	}

	tests := []struct {
		name     string
		value    string
		scope    string
		basename string
	}{
		{
			name:     "foo",
			value:    "bar",
			scope:    "",
			basename: "foo",
		},
		{
			name:     "_foo",
			value:    "bar",
			scope:    "",
			basename: "_foo",
		},
		{
			name:     "dev.foo",
			value:    "bar",
			scope:    "dev",
			basename: "foo",
		},
		{
			name:     "_sys.foo",
			value:    "bar",
			scope:    "_sys",
			basename: "foo",
		},
		{
			name:     "_sys.foo2",
			value:    "bar2",
			scope:    "_sys",
			basename: "foo2",
		},
		{
			name:     "01:23:45:67:89:AB.foo",
			value:    "bar",
			scope:    "0123456789AB",
			basename: "foo",
		},
	}

	for i, test := range tests {
		err = PutVariable(ctx, store, 0, test.name, test.value)
		if err != nil {
			t.Errorf("PutVariable %d failed with error: %v", i, err)
		}
		v, err := GetVariable(ctx, store, 0, test.name)
		if err != nil {
			t.Errorf("GetVariable %d failed with error: %v", i, err)
		}
		v, err = GetVariable(ctx, store, 0, strings.ReplaceAll(test.name, ":", ""))
		if err != nil {
			t.Errorf("GetVariable#2 %d failed with error: %v", i, err)
		}
		if v.Value != test.value {
			t.Errorf("GetVariable %d returned wrong value; expected %s, got %s", i, test.value, v.Value)
		}
		if v.Scope != test.scope {
			t.Errorf("GetVariable %d returned wrong scope; expected %s, got %s", i, test.scope, v.Scope)
		}
		bn := v.Basename()
		if bn != test.basename {
			t.Errorf("Basename returned wrong value; expected %s, got %s", test.basename, bn)
		}
	}

	vars, err := GetVariablesBySite(ctx, store, 0, "dev")
	if len(vars) != 1 {
		t.Errorf("GetVariablesBySite(dev) returned wrong number of variables; expected 1, got %d", len(vars))
	}
	vars, err = GetVariablesBySite(ctx, store, 0, "_sys")
	if len(vars) != 2 {
		t.Errorf("GetVariablesBySite(.sys) returned wrong number of variables; expected 2, got %d", len(vars))
	}
	vars, err = GetVariablesBySite(ctx, store, 0, "")
	if len(vars) < len(tests) {
		t.Errorf("GetVariablesBySite() returned wrong number of variables; expected at least %d, got %d", len(tests), len(vars))
	}

	for i, test := range tests[:2] {
		err = DeleteVariable(ctx, store, 0, test.name)
		if err != nil {
			t.Errorf("DeleteVariable %d failed with error: %v", i, err)
		}
	}
	err = DeleteVariables(ctx, store, 0, "_sys")
	if err != nil {
		t.Errorf("DeleteVariables failed with error: %v", err)
	}
}

// TestVidgrindAccess tests access to VidGrind's datastore.
// VIDGRIND_CREDENTIALS is required in order to access the datastore.
func TestVidgrindFileAccess(t *testing.T) {
	testMtsMedia(t, "file")
	testText(t, "file")
	testScalar(t, "file")
	testActuator(t, "file")
	testMtsDurations(t, "file")
	testSubscriber(t, "file")
}

func TestVidgrindCloudAccess(t *testing.T) {
	if os.Getenv("VIDGRIND_CREDENTIALS") == "" {
		t.Skip("VIDGRIND_CREDENTIALS required to test VidGrind datastore")
	}

	testMtsMedia(t, "cloud")
	testText(t, "cloud")
	testScalar(t, "cloud")
	testActuator(t, "cloud")
	testMtsDurations(t, "cloud")
	testSubscriber(t, "cloud")
}

// testMtsMedia tests MtsMedia methods.
func testMtsMedia(t *testing.T, kind string) {
	ctx := context.Background()

	store, err := datastore.NewStore(ctx, kind, "vidgrind", "")
	if err != nil {
		t.Errorf("datastore.NewStore(%s, vidgrind) failed with error: %v", kind, err)
	}

	// Write MtsMedia.
	p := mts.Packet{PID: mts.PIDVideo}
	pmtBytes := psi.NewPMTPSI().Bytes()
	mts.Meta = meta.New()
	mts.Meta.Add(mts.WriteRateKey, "25")
	pmtBytes, err = updateMeta(pmtBytes)
	if err != nil {
		t.Errorf("WriteMtsMedia #2 failed with unexpected error: %v", err)
	}
	patPkt := mts.Packet{
		PUSI: true,
		PID:  mts.PatPid,
	}
	pmtPkt := mts.Packet{
		PUSI:    true,
		PID:     mts.PmtPid,
		Payload: psi.AddPadding(pmtBytes),
	}
	pkt := append(patPkt.Bytes(nil), pmtPkt.Bytes(nil)...)
	pkt = append(pkt, p.Bytes(nil)...)
	l := len(pkt)
	err = WriteMtsMedia(ctx, store, &MtsMedia{MID: testMID2, Geohash: testGeohash, Timestamp: testTimestamp, Clip: pkt})
	if err != nil {
		t.Errorf("WriteMtsMedia failed with error: %v", err)
	}

	// Get MtsMedia.
	ts := []int64{testTimestamp}
	m, err := GetMtsMedia(ctx, store, testMID2, nil, ts)
	if err != nil {
		t.Errorf("GetMtsMedia failed with error: %v", err)
	}
	if len(m) == 0 {
		t.Errorf("GetMtsMedia failed to return anything")
	}
	if m[0].MID != testMID2 || m[0].Geohash != testGeohash || m[0].Timestamp != testTimestamp || len(m[0].Clip) != l {
		t.Errorf("GetMtsMedia returned incorrect data")
	}

	// Delete MtsMedia.
	err = DeleteMtsMedia(ctx, store, testMID2)
	if err != nil {
		t.Errorf("DeleteMtsMedia failed with error: %v", err)
	}

	// Write/delete large MtsMedia, just under the 1MB limit.
	// Insert valid PID into all packets.
	data := make([]byte, 996400)
	head := mtsHeadWithPID(mts.PIDVideo)
	for i := 0; i < len(data); i += mts.PacketSize {
		copy(head, data[i:i+3])
	}
	p2 := mts.Packet{
		PID:     mts.PIDVideo,
		Payload: data,
	}
	pkt2 := append(patPkt.Bytes(nil), pmtPkt.Bytes(nil)...)
	pkt2 = append(pkt2, p2.Bytes(nil)...)
	err = WriteMtsMedia(ctx, store, &MtsMedia{MID: testMID2, Geohash: testGeohash, Timestamp: testTimestamp, Clip: pkt2})
	if err != nil {
		t.Errorf("WriteMtsMedia #2 failed with error: %v", err)
	}
	err = DeleteMtsMedia(ctx, store, testMID2)
	if err != nil {
		t.Errorf("DeleteMtsMedia #2 failed with error: %v", err)
	}

	testBigBuckBunny(t, store)
	//	testGeohashes(t, store)
}

// updateMeta adds/updates a metaData descriptor in the given psi bytes using data
// contained in the global mts.Meta struct.
func updateMeta(b []byte) ([]byte, error) {
	p := psi.PSIBytes(b)
	err := p.AddDescriptor(psi.MetadataTag, mts.Meta.Encode())
	return []byte(p), err
}

// testBigBuckBunny write/gets/real large MTS data real(-ish) timestamps.
func testBigBuckBunny(t *testing.T, store datastore.Store) {
	ctx := context.Background()
	dir := os.Getenv("BIG_BUCK_BUNNY")
	if dir == "" {
		return
	}

	files := []string{"Big_buck_bunny_01.ts", "Big_buck_bunny_02.ts", "Big_buck_bunny_07.ts"}
	times := []int64{datastore.EpochStart + 10, datastore.EpochStart + 20, datastore.EpochStart + 70}
	for i, file := range files {
		data, err := ioutil.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Errorf("ReadFile #3 failed with error: %v", err)
		}
		err = WriteMtsMedia(ctx, store, &MtsMedia{MID: testMID2, Geohash: testGeohash, Timestamp: times[i], Clip: data})
		if err != nil {
			t.Errorf("WriteMtsMedia #3 failed with error: %v", err)
		}
	}

	ts := []int64{datastore.EpochStart, datastore.EpochStart + 100}
	m, err := GetMtsMedia(ctx, store, testMID2, nil, ts)
	if err != nil {
		t.Errorf("GetMtsMedia #3 failed with error: %v", err)
	}
	if len(m) < 9 {
		t.Errorf("GetMtsMedia #3 returned wrong number of results; expected 9 got %d", len(m))
	}
	// Test the key IDs and durations are as expected.
	// See datastore.IDKey for an explanation of how key IDs are formed.
	ids := []int64{
		testMID2<<32 | (10 << 3),
		testMID2<<32 | (10<<3 + 1),
		testMID2<<32 | (20 << 3),
		testMID2<<32 | (20<<3 + 1),
		testMID2<<32 | (20<<3 + 2),
		testMID2<<32 | (20<<3 + 3),
		testMID2<<32 | (20<<3 + 4),
		testMID2<<32 | (70 << 3),
		testMID2<<32 | (70<<3 + 1),
	}
	durations := []int64{690000, 208500, 192000, 214500, 213000, 228000, 53999, 667500, 235500}
	for i := range ids {
		if m[i].Key.ID != ids[i] {
			t.Errorf("GetMtsMedia #4.%d expected ID of %d, got %d", i, ids[i], m[i].Key.ID)
		}
		if m[i].Duration != durations[i] {
			t.Errorf("GetMtsMedia #4.%d expected duration of %d, got %d", i, durations[i], m[i].Duration)
		}
	}

	// Attempt invalid query with both a geo range and a time range.
	_, err = GetMtsMedia(ctx, store, testMID2, []string{"r1g", "r1h"}, ts)
	if err == nil {
		t.Errorf("GetMtsMedia #5 expected error, got nil")
	}

	// Key only queries:
	// Get keys without a timestamp.
	keys, err := GetMtsMediaKeys(ctx, store, testMID2, nil, nil)
	if err != nil {
		t.Errorf("GetMtsMediaKeys #1 failed with error: %v", err)
	}
	if len(keys) != 9 {
		t.Errorf("GetMtsMediaKeys #1 returned wrong number of results; expected 9, got %d", len(keys))
	}

	// Get keys with a matching timestamp.
	keys, err = GetMtsMediaKeys(ctx, store, testMID2, nil, ts)
	if err != nil {
		t.Errorf("GetMtsMediaKeys #2 failed with error: %v", err)
	}
	if len(keys) != 9 {
		t.Errorf("GetMtsMediaKeys #2 returned wrong number of results; expected 9, got %d", len(keys))
	}

	for i, k := range keys {
		m, err := GetMtsMediaByKey(ctx, store, uint64(k.ID))
		if err != nil {
			t.Errorf("GetMtsMediaByKey %d failed with error: %v", i, err)
		}
		mid, ts, _ := datastore.SplitIDKey(k.ID)
		if m.MID&0xffffff != mid {
			t.Errorf("GetMtsMediaByKey %d expected MID of %d, got %d", i, m.MID, mid)
		}
		if m.Timestamp != ts {
			t.Errorf("GetMtsMediaByKey %d expected Timestamp of %d, got %d", i, m.Timestamp, ts)
		}
	}

	// Get keys without a matching timestamp.
	ts = []int64{datastore.EpochStart + 100}
	keys, err = GetMtsMediaKeys(ctx, store, testMID2, nil, ts)
	if err != nil {
		t.Errorf("GetMtsMediaKeys #3 failed with error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("GetMtsMediaKeys #3 returned wrong number of results; expected 0, got %d", len(keys))
	}

	// Get media by a range of keys.
	m, err = GetMtsMediaByKeys(ctx, store, []uint64{uint64(ids[0]), uint64(ids[7])})
	if err != nil {
		t.Errorf("GetMtsMediaKeys #1 failed with error: %v", err)
	}
	if len(m) != 2 {
		t.Errorf("GetMtsMediaByKeys #1 returned wrong number of results; expected 2, got %d", len(m))
	}
	m, err = GetMtsMediaByKeys(ctx, store, []uint64{uint64(ids[0])})
	if err != nil {
		t.Errorf("GetMtsMediaKeys #2 failed with error: %v", err)
	}
	if len(m) != 1 {
		t.Errorf("GetMtsMediaByKeys #2 returned wrong number of results; expected 1, got %d", len(m))
	}

	DeleteMtsMedia(ctx, store, testMID2)
}

// testGeohashes tests MtsMedia with geohashes
func testGeohashes(t *testing.T, store datastore.Store) {
	ctx := context.Background()

	_, filestore := store.(*datastore.FileStore)
	if filestore {
		// FileStore not implement geohash queries.
		return
	}

	// First, create some records with geohashes.
	hashes := []string{
		"r1f9652gs", // -34.91805,138.60475 = Benham Lab, University of Adelaide
		"r1f965203", // -34.91864,138.60358 = Union House, University of Adelaide
		"r1f93fzsn", // -34.92069,138.60311 = The South Australian Museum
		"r1f93fexs", // -34.92150,138.59754 = Adelaide Railway Station
		"r1f93cmqr", // -34.92857,138.60006 = Victoria Square
	}

	pkt := make([]byte, mts.PacketSize)
	for i, gh := range hashes {
		err := WriteMtsMedia(ctx, store, &MtsMedia{MID: testMID2, Geohash: gh, Timestamp: testTimestamp + int64(i), Clip: pkt})
		if err != nil {
			t.Errorf("WriteMtsMedia #4 failed with error: %v", err)
		}
	}

	// Second, perform some searches.
	tests := []struct {
		gh   []string
		want []string
	}{
		// Zero match.
		{
			gh:   []string{"r1g", "r1h"},
			want: []string{},
		},
		// Exact match.
		{
			gh:   []string{"r1f9652gs"},
			want: []string{"r1f9652gs"},
		},
		// 2-location match.
		{
			gh:   []string{"r1f9652", "r1f9653"},
			want: []string{"r1f965203", "r1f9652gs"},
		},
		// 3-location match.
		{
			gh:   []string{"r1f93", "r1f94"},
			want: []string{"r1f93cmqr", "r1f93fexs", "r1f93fzsn"},
		},
		// 5-location match.
		{
			gh:   []string{"r1f9", "r1fb"},
			want: []string{"r1f93cmqr", "r1f93fexs", "r1f93fzsn", "r1f965203", "r1f9652gs"},
		},
	}

	for i, test := range tests {
		m, err := GetMtsMedia(ctx, store, testMID2, test.gh, []int64{})
		if err != nil {
			t.Errorf("GetMtsMedia #%d failed with error: %v", 5+i, err)
		}
		if len(test.want) != len(m) {
			t.Errorf("GetMtsMedia #%d expected %d results, got %d results", 5+i, len(test.want), len(m))
			continue
		}
		for j := range test.want {
			if test.want[j] != m[j].Geohash {
				t.Errorf("GetMtsMedia #%d.%d expected %s, got %s", 5+i, j, test.want[j], m[j].Geohash)
			}
		}
	}

	// Tidy up.
	DeleteMtsMedia(ctx, store, testMID2)
}

// testText tests Text
func testText(t *testing.T, kind string) {
	ctx := context.Background()

	store, err := datastore.NewStore(ctx, kind, "vidgrind", "")
	if err != nil {
		t.Errorf("datastore.NewStore(%s, vidgrind) failed with error: %v", kind, err)
	}

	texts := []Text{
		Text{Timestamp: datastore.EpochStart, Type: "text/plain", Data: "Hello world"},
		Text{Timestamp: datastore.EpochStart + 1, Type: "text/plain", Data: "Hallo Welt"},
		Text{Timestamp: datastore.EpochStart + 2, Type: "text/plain", Data: "Hola Mundo"},
	}
	// First, write some text
	for i, text := range texts {
		err := WriteText(ctx, store, &Text{MID: testTextMID, Timestamp: text.Timestamp, Type: text.Type, Data: text.Data})
		if err != nil {
			t.Errorf("WriteText #%d failed with error: %v", i, err)
		}
	}

	tests := []struct {
		ts   []int64
		want []string
	}{
		{
			ts:   []int64{datastore.EpochStart},
			want: []string{"Hello world"},
		},
		{
			ts:   []int64{datastore.EpochStart, datastore.EpochStart + 4},
			want: []string{"Hello world", "Hallo Welt", "Hola Mundo"},
		},
		{
			ts:   []int64{datastore.EpochStart + 4},
			want: []string{},
		},
	}

	for i, test := range tests {
		texts, err := GetText(ctx, store, testTextMID, test.ts)
		if err != nil {
			t.Errorf("GetText #%d failed with error: %v", i, err)
		}
		for j := range test.want {
			if texts[j].Data != test.want[j] {
				t.Errorf("GetText #%d returned wrong data: expected %s, got %s", i, test.want[j], texts[j].Data)
			}
		}
	}

	// Tidy up.
	DeleteText(ctx, store, testTextMID)
}

// testActuator checks that we successfully add actuators to the datastore and then get them.
func testActuator(t *testing.T, kind string) {
	ctx := context.Background()
	store, err := datastore.NewStore(ctx, kind, "vidgrind", "")
	if err != nil {
		t.Errorf("datastore.NewStore(%s, vidgrind) failed with error: %v", kind, err)
	}

	tests := []struct {
		want Actuator
	}{
		{want: Actuator{AID: "testActuator1", Var: "testActuatorVar1", Pin: "testDevice1" + ".D1"}},
		{want: Actuator{AID: "testActuator2", Var: "testActuatorVar2", Pin: "testDevice2" + ".D2"}},
		{want: Actuator{AID: "testActuator3", Var: "testActuatorVar3", Pin: "testDevice3" + ".D3"}},
		{want: Actuator{AID: "testActuator4", Var: "testActuatorVar4", Pin: "testDevice4" + ".D4"}},
	}

	// Write actuators to datastore.
	for i := range tests {
		err := PutActuator(ctx, store, &tests[i].want)
		if err != nil {
			t.Errorf("WriteActuator #%d failed with error: %v", i, err)
		}
	}

	// Retrieve by pin.
	for i := range tests {
		act, err := GetActuatorByPin(ctx, store, 0, "testDevice"+strconv.Itoa(i+1), "D"+strconv.Itoa(i+1))
		if err != nil {
			t.Errorf("GetActuatorByPin #%d failed with error: %v", i, err)
		}

		if !reflect.DeepEqual(act[0], tests[i].want) {
			t.Errorf("Did not get expected result.\nGot: %v\nWant: %v", act, tests[i].want)
		}
	}

	// Clean up.
	for i := range tests {
		DeleteActuator(ctx, store, tests[i].want.AID)
	}
}

// testScalar tests scalars.
func testScalar(t *testing.T, kind string) {
	ctx := context.Background()
	store, err := datastore.NewStore(ctx, kind, "vidgrind", "")
	if err != nil {
		t.Errorf("datastore.NewStore(%s, vidgrind) failed with error: %v", kind, err)
	}

	tests := []struct {
		scalar Scalar
		want   Scalar
	}{
		{
			scalar: Scalar{ID: ToSID(testDevMac, "A0"), Timestamp: datastore.EpochStart, Value: 1},
			want:   Scalar{ID: (testDevMa << 8) | 100, Timestamp: datastore.EpochStart, Value: 1},
		},
		{
			scalar: Scalar{ID: ToSID(testDevMac, "D12"), Timestamp: datastore.EpochStart, Value: 2},
			want:   Scalar{ID: (testDevMa << 8) | 13, Timestamp: datastore.EpochStart, Value: 2},
		},
		{
			scalar: Scalar{ID: ToSID(testDevMac, "X22"), Timestamp: datastore.EpochStart, Value: 3},
			want:   Scalar{ID: (testDevMa << 8) | (128 + 22), Timestamp: datastore.EpochStart, Value: 3},
		},
		{
			scalar: Scalar{ID: ToSID(testDevMac, "X22"), Timestamp: datastore.EpochStart + 1, Value: 4},
			want:   Scalar{ID: (testDevMa << 8) | (128 + 22), Timestamp: datastore.EpochStart + 1, Value: 4},
		},
		{
			scalar: Scalar{ID: ToSID(testDevMac, "X22"), Timestamp: datastore.EpochStart + 2, Value: 4.12345},
			want:   Scalar{ID: (testDevMa << 8) | (128 + 22), Timestamp: datastore.EpochStart + 2, Value: 4.123},
		},
		{
			scalar: Scalar{ID: ToSID(testDevMac, ""), Timestamp: datastore.EpochStart + 3, Value: 5},
			want:   Scalar{ID: testDevMa << 8, Timestamp: datastore.EpochStart + 3, Value: 5},
		},
	}

	// First, write some scalars
	for i := range tests {
		err := PutScalar(ctx, store, &tests[i].scalar)
		if err != nil {
			t.Errorf("WriteScalar #%d failed with error: %v", i, err)
		}
	}

	// Second, retrieve with a single timestamp.
	for i := range tests {
		s, err := GetScalar(ctx, store, tests[i].scalar.ID, tests[i].scalar.Timestamp)
		if err != nil {
			t.Errorf("GetScalars #%d failed with error: %v", i, err)
		}
		if tests[i].want.ID != s.ID {
			t.Errorf("GetScalars #%d expected ID %d, got %d", i, tests[i].want.ID, s.ID)
		}
		if tests[i].want.Timestamp != s.Timestamp {
			t.Errorf("GetScalars #%d expected Timestamp %d, got %d", i, tests[i].want.Timestamp, s.Timestamp)
		}
		if tests[i].want.FormatValue(3) != s.FormatValue(3) {
			t.Errorf("GetScalars #%d expected Value %f, got %f", i, tests[i].want.Value, s.Value)
		}
	}

	// Third, retrieve using a timestamp range.
	s, err := GetScalars(ctx, store, ToSID(testDevMac, "X22"), []int64{datastore.EpochStart, datastore.EpochStart + 2})
	if err != nil {
		t.Errorf("GetScalars failed with error: %v", err)
	}
	if len(s) != 2 {
		t.Errorf("GetScalars expected 2 results, got %d results", len(s))
	}

	// Tidy up.
	for i := range tests {
		DeleteScalars(ctx, store, tests[i].scalar.ID)
	}
}

func testMtsDurations(t *testing.T, kind string) {
	ctx := context.Background()

	testDataPath := "../../test/test-data/av/input/audio"
	_, err := os.Stat(testDataPath)
	if err != nil {
		t.Skipf("skipping testMtsDurations")
		return
	}

	store, err := datastore.NewStore(ctx, kind, "vidgrind", "")
	if err != nil {
		t.Fatalf("could not create new store: %v", err)
	}

	tests := []struct {
		file string
		want int64
	}{
		{"test_audio_mts1.ts", 3 * mts.PTSFrequency},
		{"test_audio_mts2.ts", 3 * mts.PTSFrequency},
		{"test_audio_mts3.ts", 2 * mts.PTSFrequency},
		{"test_audio_mts4.ts", 3 * mts.PTSFrequency},
	}

	const tolerance = 3
	for i, dt := range tests {
		// Read input MTS audio.
		data, err := ioutil.ReadFile(filepath.Join(testDataPath, dt.file))
		if err != nil {
			t.Errorf("Unable to read MTS test file %v for test %v: %v", dt.file, i, err)
			continue
		}

		ts := time.Now().Unix()
		m := &MtsMedia{
			MID:       testMID,
			Timestamp: ts,
			Clip:      data,
			FramePTS:  mts.PTSFrequency,
		}
		err = WriteMtsMedia(ctx, store, m)
		if err != nil {
			t.Errorf("write failed for test %v: %v", i, err)
			continue
		}

		media, err := GetMtsMedia(ctx, store, m.MID, nil, []int64{ts})
		if err != nil {
			t.Errorf("could not get media for test %v: %v", i, err)
			continue
		}
		if len(media) == 0 {
			t.Errorf("no media in store for test %v", i)
			continue
		}
		if media[0].Duration < dt.want-tolerance || media[0].Duration > dt.want+tolerance {
			t.Errorf("failed to calculate correct duration for test %v, expected %v, got %v", i, dt.want, media[0].Duration)
		}
	}
}

// TestFirstMediaPID tests the functionality of firstMediaPID.
func TestFirstMediaPID(t *testing.T) {
	// MTS packet types.
	const (
		pat = iota
		pmt
		vid
		aud
	)

	tests := []struct {
		wantPID  uint16 // The PID that is expected to be found.
		pass     bool   // True if the test should pass without error.
		order    []int  // Odd index ints represent a packet type, even indices represent the number of that packet type to append to the test data.
		nPackets int    // The number of packets in the test data.
	}{
		{
			wantPID:  0,
			pass:     false,
			order:    []int{pat, 1},
			nPackets: 1,
		},
		{
			wantPID:  0,
			pass:     false,
			order:    []int{pmt, 1},
			nPackets: 1,
		},
		{
			wantPID:  mts.PIDAudio,
			pass:     true,
			order:    []int{aud, 1},
			nPackets: 1,
		},
		{
			wantPID:  mts.PIDAudio,
			pass:     true,
			order:    []int{pat, 1, pmt, 1, aud, 10},
			nPackets: 12,
		},
		{
			wantPID:  mts.PIDVideo,
			pass:     true,
			order:    []int{vid, 6},
			nPackets: 6,
		},
		{
			wantPID:  mts.PIDVideo,
			pass:     true,
			order:    []int{pat, 1, pmt, 1, vid, 1},
			nPackets: 3,
		},
		{
			wantPID:  mts.PIDVideo,
			pass:     true,
			order:    []int{pat, 1, pmt, 1, vid, 30, pat, 1},
			nPackets: 33,
		},
	}
	for i, test := range tests {
		packets := make([]byte, test.nPackets*mts.PacketSize)
		for i := 0; i < len(test.order)/2; i++ {
			switch test.order[i*2] {
			case pat:
				for j := 0; j < test.order[i*2+1]; j++ {
					p := mts.Packet{PID: mts.PatPid}
					packets = append(packets, p.Bytes(nil)...)
				}
			case pmt:
				for j := 0; j < test.order[i*2+1]; j++ {
					p := mts.Packet{PID: mts.PmtPid}
					packets = append(packets, p.Bytes(nil)...)
				}
			case vid:
				for j := 0; j < test.order[i*2+1]; j++ {
					p := mts.Packet{PID: mts.PIDVideo}
					packets = append(packets, p.Bytes(nil)...)
				}
			case aud:
				for j := 0; j < test.order[i*2+1]; j++ {
					p := mts.Packet{PID: mts.PIDAudio}
					packets = append(packets, p.Bytes(nil)...)
				}
			}
		}
		pid, err := firstMediaPID(packets)
		if err != nil {
			if test.pass {
				t.Errorf("test %d failed: %v", i, err)
			}
		}
		if pid != test.wantPID {
			t.Errorf("test %d failed, got PID: %d, want PID: %d", i, pid, test.wantPID)
		}
	}
}

// mtsHeadWithPID returns first 3 bytes of a valid mts packet header containing given PID for test purposes.
func mtsHeadWithPID(pid uint16) []byte {
	return []byte{0x47, byte((pid & 0xFF00) >> 8), byte(pid & 0x00FF)}
}

func TestGotsPacket(t *testing.T) {
	b := make([]byte, mts.PacketSize)
	p := gotsPacket(b)
	p.SetPID(256)
	if p.PID() != 256 {
		t.Error("failed to set PID in gots Packet")
	}
}

// testCron tests Cron.
func testCron(t *testing.T, kind string) {
	ctx := context.Background()

	store, err := datastore.NewStore(ctx, kind, "vidgrind", "")
	if err != nil {
		t.Fatalf("could not create new store: %v", err)
	}

	c1 := Cron{Skey: 1, ID: "Test", Time: time.Unix(0, 0), TOD: "Sunrise", Action: "set", Var: "Power", Data: "off"}
	err = PutCron(ctx, store, &c1)
	enc := string(c1.Encode())
	if enc != testCronEnc {
		t.Errorf("Cron.Encode(1) failed: expected %s, got %s", testCronEnc, enc)
	}
	if err != nil {
		t.Errorf("PutCron failed with error %v", err)
	}
	c2, err := GetCron(ctx, store, 1, "Test")
	if err != nil {
		t.Errorf("GetCron failed with error %v", err)
	}
	enc = string(c2.Encode())
	if enc != testCronEnc {
		t.Errorf("Cron.Encode(2) failed: expected %s, got %s", testCronEnc, enc)
	}
	err = DeleteCron(ctx, store, 1, "Test")
	if err != nil {
		t.Errorf("DeleteCron failed with error %v", err)
	}
}

// testSubscriber tests Subscriber methods.
func testSubscriber(t *testing.T, kind string) {
	ctx := context.Background()
	store, err := datastore.NewStore(ctx, kind, "vidgrind", "")
	if err != nil {
		t.Fatalf("could not create new store: %v", err)
	}

	// Since we will create a new subscriber, we need to make sure to delete the existing one if it exists
	store.Delete(ctx, store.IDKey(typeSubscriber, testSubscriberID))

	// Remove the monotonic time element from the Created field.
	s1 := &Subscriber{testSubscriberID, "", testUserEmail, "first", "last", nil, "", "", time.Now().Round(time.Second).UTC()}

	err = CreateSubscriber(ctx, store, s1)
	if err != nil {
		t.Errorf("CreateSubscriber failed with error: %v", err)
	}

	s2, err := GetSubscriber(ctx, store, s1.ID)
	if err != nil {
		t.Errorf("GetSubscriber failed with error: %v", err)
	}

	if !reflect.DeepEqual(s1, s2) {
		t.Errorf("Got different subscriber than created (by ID), got: \n%+v, wanted \n%+v", s2, s1)
	}

	s2, err = GetSubscriberByEmail(ctx, store, testUserEmail)
	if err != nil {
		t.Errorf("GetSubscriberByEmail failed with error: %v", err)
	}

	if !reflect.DeepEqual(s1, s2) {
		t.Errorf("Got different subscriber than created (by Email), got: \n%+v, wanted \n%+v", s2, s1)
	}

	s1.FamilyName = "New-Name"
	err = UpdateSubscriber(ctx, store, s1)
	if err != nil {
		t.Errorf("UpdateSubscriber failed with error: %v", err)
	}

	s2, err = GetSubscriber(ctx, store, testSubscriberID)
	if err != nil {
		t.Errorf("GetSubscriberByEmail failed with error: %v", err)
	}

	if !reflect.DeepEqual(s1, s2) {
		t.Errorf("Got different subscriber than updated (by ID), got: \n%+v, wanted \n%+v", s2, s1)
	}
}

// testSubscriber tests Subscription methods.
func testSubscription(t *testing.T, kind string) {
	ctx := context.Background()
	store, err := datastore.NewStore(ctx, kind, "vidgrind", "")
	if err != nil {
		t.Fatalf("could not create new store: %v", err)
	}

	// Since we will create a new subscription, we need to make sure to delete the existing one if it exists
	store.Delete(ctx, store.NameKey(typeSubscription, fmt.Sprintf("%d.%d", testSubscriberID, testFeedID)))

	start := time.Now().Truncate(24 * time.Hour).UTC()
	finish := start.AddDate(0, 0, 1)
	s1 := &Subscription{SubscriberID: testSubscriberID, FeedID: testFeedID, Class: SubscriptionDay, Prefs: "", Start: start, Finish: finish, Renew: true}

	err = CreateSubscription(ctx, store, testSubscriberID, testFeedID, "", true, WithSubscriptionClass(SubscriptionDay))
	if err != nil {
		t.Errorf("CreateSubscription failed with error: %v", err)
	}

	s2, err := GetSubscription(ctx, store, testSubscriberID, testFeedID)
	if err != nil {
		t.Errorf("GetSubscription failed with error: %v", err)
	}

	if !reflect.DeepEqual(s1, s2) {
		t.Errorf("Got different subscription than created (by IDs), got: \n%+v, wanted \n%+v", s2, s1)
	}

	subs, err := GetSubscriptions(ctx, store, testSubscriberID)
	if err != nil {
		t.Errorf("GetSubscriptions failed with error: %v", err)
	}

	if len(subs) != 1 {
		t.Errorf("got incorrect number of subscriptions, got %d, wanted 1", len(subs))
	}

	if !reflect.DeepEqual(s1, &subs[0]) {
		t.Errorf("Got different subscription than created, got: \n%+v, wanted \n%+v", &subs[0], s1)
	}

	s1.Renew = false
	err = UpdateSubscription(ctx, store, s1)
	if err != nil {
		t.Errorf("UpdateSubscriber failed with error: %v", err)
	}

	s2, err = GetSubscription(ctx, store, testSubscriberID, testFeedID)
	if err != nil {
		t.Errorf("GetSubscription failed with error: %v", err)
	}

	if !reflect.DeepEqual(s1, s2) {
		t.Errorf("Got different subscription than updated (by IDs), got: \n%+v, wanted \n%+v", s2, s1)
	}

}

func testFeed(t *testing.T, kind string) {
	ctx := context.Background()
	store, err := datastore.NewStore(ctx, kind, "vidgrind", "")
	if err != nil {
		t.Fatalf("could not create new store: %v", err)
	}

	// Since we will create a new Feed, we need to make sure to delete the existing one if it exists
	store.Delete(ctx, store.IDKey(typeFeed, testFeedID))
}

// Benchmarks follow.
// These are executed by running "go test -bench=."

// BenchmarkSiteWithCaching benchmarks site retrieval with caching.
func BenchmarkSiteWithCaching(b *testing.B) {
	benchmarkSite(b)
}

// BenchmarkSiteWithoutCaching benchmarks site retrieval without caching.
func BenchmarkSiteWithoutCaching(b *testing.B) {
	siteCache = nil // Disable caching.
	benchmarkSite(b)
}

func benchmarkSite(b *testing.B) {
	ctx := context.Background()
	store, err := datastore.NewStore(ctx, "cloud", "vidgrind", "")
	if err != nil {
		b.Errorf("could not get store: %v", err)
	}
	site := Site{Skey: testSiteKey, Name: testSiteName, Latitude: testSiteLat, Longitude: testSiteLng, Timezone: testSiteTZ, Enabled: true}
	err = PutSite(ctx, store, &site)
	if err != nil {
		b.Errorf("could not put site: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetSite(ctx, store, testSiteKey)
		if err != nil {
			b.Fatalf("could not get site: %v", err)
		}
	}
}

func TestFeed(t *testing.T) {
	const (
		testFeedName   = "Test Feed"
		testFeedArea   = "Fleurieu Peninsula"
		testFeedClass  = "Video"
		testFeedSource = "https://youtube.com/watch?v=1234567890"
	)

	ctx := context.Background()
	store, err := datastore.NewStore(ctx, "file", "vidgrind", "")
	if err != nil {
		t.Fatalf("could not get store: %v", err)
	}

	// Clear any existing feeds.
	store.Delete(ctx, store.IDKey(typeFeed, testFeedID))
	store.Delete(ctx, store.IDKey(typeFeed, testFeedID+1))

	feed := &Feed{ID: testFeedID, Name: testFeedName, Area: testFeedArea, Class: testFeedClass, Source: testFeedSource, Created: time.Now().UTC().Truncate(0)}
	err = CreateFeed(ctx, store, feed)
	if err != nil {
		t.Errorf("could not create feed: %v", err)
	}

	feed2, err := GetFeed(ctx, store, testFeedID)
	if err != nil {
		t.Errorf("could not get feed: %v", err)
	}

	assert.Equal(t, feed, feed2, "Got different feed than put, got: \n%+v, wanted \n%+v", feed2, feed)

	feed.Name = "Updated Feed Name"
	feed, err = UpdateFeed(ctx, store, feed)
	if err != nil {
		t.Errorf("could not update feed: %v", err)
	}

	feed3, err := GetFeed(ctx, store, testFeedID)
	if err != nil {
		t.Errorf("could not get feed: %v", err)
	}

	assert.Equal(t, feed, feed3, "Got different feed than put, got: \n%+v, wanted \n%+v", feed3, feed)

	newFeed := &Feed{ID: testFeedID + 1, Name: "New Feed", Created: time.Now().UTC().Truncate(0)}
	err = CreateFeed(ctx, store, newFeed)
	if err != nil {
		t.Errorf("could not create new feed: %v", err)
	}

	feeds, err := GetAllFeeds(ctx, store)
	if err != nil {
		t.Errorf("could not get all feeds: %v", err)
	}

	assert.Equal(t, []Feed{*feed, *newFeed}, feeds, "Got different feeds than put, got: \n%+v, wanted \n%+v", feeds, []Feed{*feed, *newFeed})

	err = DeleteFeed(ctx, store, testFeedID)
	if err != nil {
		t.Errorf("could not delete feed: %v", err)
	}

	feed4, err := GetFeed(ctx, store, testFeedID)
	if !errors.Is(err, datastore.ErrNoSuchEntity) {
		t.Errorf("expected ErrNoSuchEntity, got %v", err)
	}

	if feed4 != nil {
		t.Errorf("expected nil, got %v", feed4)
	}
}
