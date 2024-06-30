/*
DESCRIPTION
  file_test.go provides testing for functionality contained in file.go.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ausocean/av/revid"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/cloud/cmd/vidforward/global"
	"github.com/ausocean/utils/logging"
)

const (
	testURL = "rtmp://some-random-url.abcdef-12345"
	testMAC = "78:90:AE:7B:2C:76"
)

func init() {
	inTest = true
}

func TestBroadcastMarshal(t *testing.T) {
	logger := (*logging.TestLogger)(t)

	// Marshalling functionality uses this.
	global.SetLogger(logger)

	tests := []struct {
		in     Broadcast
		expect []byte
	}{
		{
			in: Broadcast{
				mac:    testMAC,
				urls:   []string{testURL},
				status: statusActive,
				rv:     newRevidForTest((*logging.TestLogger)(t), testURL, t),
			},
			expect: []byte("{\"MAC\":\"" + testMAC + "\",\"URLs\":[\"" + testURL + "\"],\"Status\":\"" + statusActive + "\"}"),
		},
	}

	for i, test := range tests {
		got, err := test.in.MarshalJSON()
		if err != nil {
			t.Errorf("could not marshal json for test no. %d: %v", i, err)
			continue
		}
		if !bytes.Equal(got, test.expect) {
			t.Errorf("did not get expected result.\nGot: %v\nWnt: %v\n", string(got), string(test.expect))
		}
	}
}

func TestBroadcastUnmarshal(t *testing.T) {
	logger := (*logging.TestLogger)(t)

	// Marshalling functionality uses this.
	global.SetLogger(logger)

	tests := []struct {
		in     []byte
		expect *Broadcast
	}{
		{
			expect: &Broadcast{
				mac:    testMAC,
				urls:   []string{testURL},
				status: statusActive,
			},
			in: []byte("{\"MAC\":\"" + testMAC + "\",\"URLs\":[\"" + testURL + "\"],\"Status\":\"" + statusActive + "\"}"),
		},
	}

	for i, test := range tests {
		var got Broadcast
		err := got.UnmarshalJSON(test.in)
		if err != nil {
			t.Errorf("could not marshal json for test no. %d: %v", i, err)
			continue
		}
		if !broadcastsEqual(&got, test.expect) {
			t.Errorf("did not get expected result.\nGot: %v\nWnt: %v\n", got, test.expect)
		}
	}
}

func TestBroadcastManagerMarshal(t *testing.T) {
	logger := (*logging.TestLogger)(t)

	// Marshalling functionality uses this.
	global.SetLogger(logger)

	tests := []struct {
		in     broadcastManager
		expect []byte
	}{
		{
			in: broadcastManager{
				broadcasts: map[MAC]*Broadcast{
					testMAC: &Broadcast{
						testMAC,
						[]string{testURL},
						statusSlate,
						newRevidForTest((*logging.TestLogger)(t), testURL, t),
					},
				},
				log:         logger,
				dogNotifier: newWatchdogNotifierForTest(t, logger),
			},
			expect: []byte("{\"Broadcasts\":{\"" + testMAC + "\":{\"MAC\":\"" + testMAC + "\",\"URLs\":[\"" + testURL + "\"],\"Status\":\"" + statusSlate + "\"}}}"),
		},
	}

	for i, test := range tests {
		got, err := test.in.MarshalJSON()
		if err != nil {
			t.Errorf("could not marshal json for test no. %d: %v", i, err)
			continue
		}
		if !bytes.Equal(got, test.expect) {
			t.Errorf("did not get expected result.\nGot: %v\nWnt: %v\n", string(got), string(test.expect))
		}
	}
}

func TestBroadcastManagerUnmarshal(t *testing.T) {
	logger := (*logging.TestLogger)(t)

	// Marshalling functionality uses this.
	global.SetLogger(logger)

	tests := []struct {
		in     []byte
		expect broadcastManager
	}{
		{
			in: []byte("{\"Broadcasts\":{\"" + testMAC + "\":{\"MAC\":\"" + testMAC + "\",\"URLs\":[\"" + testURL + "\"],\"Status\":\"" + statusSlate + "\"}},\"SlateExitSignals\":[\"" + testMAC + "\"]}"),
			expect: broadcastManager{
				broadcasts: map[MAC]*Broadcast{
					testMAC: &Broadcast{
						testMAC,
						[]string{testURL},
						statusSlate,
						newRevidForTest((*logging.TestLogger)(t), testURL, t),
					},
				},
				slateExitSignals: newExitSignalsForTest(t, testMAC),
				log:              logger,
				dogNotifier:      newWatchdogNotifierForTest(t, logger),
			},
		},
	}

	for i, test := range tests {
		var got broadcastManager
		if err := json.Unmarshal(test.in, &got); err != nil {
			t.Errorf("could not unmarshal json for test no. %d: %v", i, err)
			continue
		}
		if !broadcastManagersEqual(got, test.expect) {
			t.Errorf("did not get expected result.\nGot: %+v\nWnt: %+v\n", got, test.expect)
		}
	}
}

func broadcastManagersEqual(m1, m2 broadcastManager) bool {
	if !broadcastMapsEqual(m1.broadcasts, m2.broadcasts) ||
		!slateExitSignalMapsEqual(m1.slateExitSignals, m2.slateExitSignals) ||
		!watchdogNotifiersEqual(*m1.dogNotifier, *m2.dogNotifier) {
		return false
	}
	return true
}

func broadcastMapsEqual(m1, m2 map[MAC]*Broadcast) bool {
	return mapsEqual(m1, m2, broadcastsEqual)
}

func slateExitSignalMapsEqual(m1, m2 map[MAC]chan struct{}) bool {
	return mapsEqual(m1, m2, func(v1, v2 chan struct{}) bool {
		return ((v1 == nil || v2 == nil) && v1 == v2) || (v1 != nil && v2 != nil)
	})
}

func activeHandlersMapEqual(m1, m2 map[int]handlerInfo) bool {
	return mapsEqual(m1, m2, func(v1, v2 handlerInfo) bool { return v1.name == v2.name })
}

// mapsEqual is a generic function to check that any two maps are equal based on
// the provided value compare function cmp.
func mapsEqual[K comparable, V any](m1, m2 map[K]V, cmp func(v1, v2 V) bool) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		v2, ok := m2[k]
		if !ok || !cmp(v1, v2) {
			return false
		}
	}
	return true
}

func watchdogNotifiersEqual(w1, w2 watchdogNotifier) bool {
	if w1.watchdogInterval != w2.watchdogInterval ||
		!activeHandlersMapEqual(w1.activeHandlers, w2.activeHandlers) {
		return false
	}
	return true
}

func broadcastsEqual(b1, b2 *Broadcast) bool {
	if b1 == nil {
		panic("b1 is nil")
	}
	if b2 == nil {
		panic("b2 is nil")
	}
	if b1.mac != b2.mac || !reflect.DeepEqual(b1.urls, b2.urls) || b1.status != b2.status ||
		((b1.rv == nil || b2.rv == nil) && b1.rv != b2.rv) {
		return false
	}
	if b1.rv != nil && !configsEqual(b1.rv.Config(), b2.rv.Config()) {
		return false
	}
	return true
}

// configsEqual returns true if the provided config.Config values are equal. The
// comparison is shallow given that only fields of basic types are compared, not
// structs or interfaces.
func configsEqual(cfg1, cfg2 config.Config) bool {
	cfg1ValOf := reflect.ValueOf(cfg1)
	cfg2ValOf := reflect.ValueOf(cfg2)
	for i := 0; i < cfg1ValOf.NumField(); i++ {
		if cfg1ValOf.Field(i).Kind() == reflect.Struct || cfg1ValOf.Field(i).Kind() == reflect.Interface {
			continue
		}
		if !reflect.DeepEqual(cfg1ValOf.Field(i).Interface(), cfg2ValOf.Field(i).Interface()) {
			return false
		}
	}
	return true
}

// newRevidForTest allows us to create revid in table driven test entry.
func newRevidForTest(log logging.Logger, url string, t *testing.T) *revid.Revid {
	r, err := newRevid(log, []string{url})
	if err != nil {
		t.Fatalf("could not create revid pipeline: %v", err)
		return nil
	}
	return r
}

// newExitSignalsForTest creates a map of chan struct{} for the provided MACs.
// This is used to populate the slateExitSignals field in the broadcastManager.
func newExitSignalsForTest(t *testing.T, macs ...MAC) map[MAC]chan struct{} {
	sigMap := make(map[MAC]chan struct{})
	for _, m := range macs {
		sigMap[m] = make(chan struct{})
	}
	return sigMap
}

// newWatchdogNotifierForTest allows us to create watchdog notifier in test table.
func newWatchdogNotifierForTest(t *testing.T, l logging.Logger) *watchdogNotifier {
	n, err := newWatchdogNotifier(l, func() {})
	if err != nil {
		t.Fatalf("could not create new watchdog notifier: %v", err)
		return nil
	}
	return n
}
