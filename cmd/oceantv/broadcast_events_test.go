package main

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"context"
)

func TestBasicEventBus(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	storedEvents := []event{}

	storeMock := func(event event) {
		storedEvents = append(storedEvents, event)
	}

	log := func(string, ...interface{}) {}

	bus := newBasicEventBus(ctx, storeMock, log)

	t.Run("subscribe and publish", func(t *testing.T) {
		var mu sync.Mutex
		receivedEvents := []event{}

		handler := func(e event) error {
			mu.Lock()
			receivedEvents = append(receivedEvents, e)
			mu.Unlock()
			return nil
		}

		bus.subscribe(handler)
		bus.publish(timeEvent{})

		if len(receivedEvents) != 1 {
			t.Errorf("expected 1 event, got %d", len(receivedEvents))
		}

		// Check the type of event.
		if _, ok := receivedEvents[0].(timeEvent); !ok {
			t.Errorf("expected timeEvent, got %T", receivedEvents[0])
		}
	})

	t.Run("Multiple subscribers", func(t *testing.T) {
		var mu sync.Mutex
		receivedEvents := []event{}

		handler1 := func(e event) error {
			mu.Lock()
			receivedEvents = append(receivedEvents, e)
			mu.Unlock()
			return nil
		}

		handler2 := func(e event) error {
			mu.Lock()
			receivedEvents = append(receivedEvents, e)
			mu.Unlock()
			return nil
		}

		bus.subscribe(handler1)
		bus.subscribe(handler2)
		bus.publish(timeEvent{})

		if len(receivedEvents) != 2 {
			t.Errorf("expected 2 events, got %d", len(receivedEvents))
		}

		// Test type of events.
		if _, ok := receivedEvents[0].(timeEvent); !ok {
			t.Errorf("expected timeEvent, got %T", receivedEvents[0])
		}

		if _, ok := receivedEvents[1].(timeEvent); !ok {
			t.Errorf("expected timeEvent, got %T", receivedEvents[1])
		}
	})

	t.Run("Storing events after cancel", func(t *testing.T) {
		cancel() // cancel the context
		bus.publish(startEvent{})

		if len(storedEvents) != 1 {
			t.Errorf("expected 1 stored event, got %d", len(storedEvents))
		}

		// Test type of event.
		if _, ok := storedEvents[0].(startEvent); !ok {
			t.Errorf("expected startEvent, got %T", storedEvents[0])
		}
	})

	t.Run("Panic on non-cancellable context", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("The code did not panic")
			}
		}()
		busNonCancelable := newBasicEventBus(context.Background(), storeMock, log)
		busNonCancelable.publish(startEvent{})
	})
}

func TestStringToEvent(t *testing.T) {
	tests := []struct {
		name      string
		expected  event
		wantPanic bool
	}{
		{"timeEvent", timeEvent{}, false},
		{"finishEvent", finishEvent{}, false},
		{"startEvent", startEvent{}, false},
		{"startedEvent", startedEvent{}, false},
		{"startFailedEvent", startFailedEvent{}, false},
		{"healthCheckDueEvent", healthCheckDueEvent{}, false},
		{"statusCheckDueEvent", statusCheckDueEvent{}, false},
		{"chatMessageDueEvent", chatMessageDueEvent{}, false},
		{"badHealthEvent", badHealthEvent{}, false},
		{"goodHealthEvent", goodHealthEvent{}, false},
		{"hardwareResetRequestEvent", hardwareResetRequestEvent{}, false},
		{"hardwareStartFailedEvent", hardwareStartFailedEvent{}, false},
		{"hardwareStopFailedEvent", hardwareStopFailedEvent{}, false},
		{"hardwareStartedEvent", hardwareStartedEvent{}, false},
		{"hardwareStoppedEvent", hardwareStoppedEvent{}, false},
		{"slateResetRequested", slateResetRequested{}, false},
		{"NonExistentEvent", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil && tt.wantPanic {
					t.Errorf("expected panic, got none")
				}

				if r != nil && !tt.wantPanic {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			got := stringToEvent(tt.name)
			if !tt.wantPanic && !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("stringToEvent() got = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestErrorsIs(t *testing.T) {
	type testCase struct {
		name        string
		err         error
		target      error
		expectMatch bool
	}

	warn := warnSkipShutdown

	tests := []testCase{
		{
			name:        "ShutdownEvent wraps warnSkipShutdown directly",
			err:         hardwareShutdownFailedEvent{warn},
			target:      warn,
			expectMatch: true,
		},
		{
			name:        "ShutdownEvent wraps fmt.Errorf with warnSkipShutdown",
			err:         hardwareShutdownFailedEvent{fmt.Errorf("could not perform shutdown actions: %w", warn)},
			target:      warn,
			expectMatch: true,
		},
		{
			name:        "PowerOffEvent wraps empty ShutdownEvent directly",
			err:         hardwarePowerOffFailedEvent{hardwareShutdownFailedEvent{}},
			target:      hardwareShutdownFailedEvent{},
			expectMatch: true,
		},
		{
			name:        "PowerOffEvent wraps fmt.Errorf with empty ShutdownEvent",
			err:         hardwarePowerOffFailedEvent{fmt.Errorf("got error event: %w", hardwareShutdownFailedEvent{})},
			target:      hardwareShutdownFailedEvent{},
			expectMatch: true,
		},
		{
			name:        "PowerOffEvent wraps fmt.Errorf with ShutdownEvent{warn}",
			err:         hardwarePowerOffFailedEvent{fmt.Errorf("got error event: %w", hardwareShutdownFailedEvent{warn})},
			target:      hardwareShutdownFailedEvent{},
			expectMatch: true,
		},
		{
			name:        "PowerOffEvent wraps fmt.Errorf with ShutdownEvent{warn}, match on warn",
			err:         hardwarePowerOffFailedEvent{fmt.Errorf("got error event: %w", hardwareShutdownFailedEvent{warn})},
			target:      warn,
			expectMatch: true,
		},
		{
			name:        "PowerOffEvent wraps ShutdownEvent{warn} directly",
			err:         hardwarePowerOffFailedEvent{hardwareShutdownFailedEvent{warn}},
			target:      hardwareShutdownFailedEvent{},
			expectMatch: true,
		},
		{
			name:        "PowerOffEvent wraps ShutdownEvent{warn} directly, match on warn",
			err:         hardwarePowerOffFailedEvent{hardwareShutdownFailedEvent{warn}},
			target:      warn,
			expectMatch: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			match := errors.Is(tc.err, tc.target)
			if match != tc.expectMatch {
				t.Errorf("errors.Is(%v, %v) = %v; want %v", tc.err, tc.target, match, tc.expectMatch)
			}
		})
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		input       event
		target      error // Only relevant if input is errorEvent
		expectMatch bool  // Only applies for errorEvent + target
	}{
		{
			name:        "Shutdown wraps warn",
			input:       hardwareShutdownFailedEvent{warnSkipShutdown},
			target:      warnSkipShutdown,
			expectMatch: true,
		},
		{
			name:        "Shutdown wraps fmt.Errorf(warn)",
			input:       hardwareShutdownFailedEvent{fmt.Errorf("wrap: %w", warnSkipShutdown)},
			target:      warnSkipShutdown,
			expectMatch: true,
		},
		{
			name:        "PowerOff wraps Shutdown{warn}",
			input:       hardwarePowerOffFailedEvent{hardwareShutdownFailedEvent{warnSkipShutdown}},
			target:      hardwareShutdownFailedEvent{},
			expectMatch: true,
		},
		{
			name:        "PowerOff wraps fmt.Errorf(wrapped Shutdown{warn})",
			input:       hardwarePowerOffFailedEvent{fmt.Errorf("wrap: %w", hardwareShutdownFailedEvent{warnSkipShutdown})},
			target:      warnSkipShutdown,
			expectMatch: true,
		},
		{
			name:        "PowerOff wraps Shutdown with no cause",
			input:       hardwarePowerOffFailedEvent{hardwareShutdownFailedEvent{}},
			target:      hardwareShutdownFailedEvent{},
			expectMatch: true,
		},
		{
			name:        "PowerOff wraps Shutdown with no cause – doesn't match warn",
			input:       hardwarePowerOffFailedEvent{hardwareShutdownFailedEvent{}},
			target:      warnSkipShutdown,
			expectMatch: false,
		},
		{
			name:        "Non-error startEvent round-trip",
			input:       startEvent{},
			target:      nil,
			expectMatch: false, // unused
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := marshalEvent(tc.input)
			unmarshalled := unmarshalEvent(data)

			if _, ok := tc.input.(error); ok && tc.target != nil {
				// Check that Error() returns the same string.
				if fmt.Sprint(unmarshalled) != fmt.Sprint(tc.input) {
					t.Errorf("expected error event %v, got %v", tc.input, unmarshalled)
				}

				// Check that we can still perform matching.
				match := errors.Is(unmarshalled.(error), tc.target)
				if match != tc.expectMatch {
					t.Errorf("errors.Is(..., %v) = %v, want %v", tc.target, match, tc.expectMatch)
				}

			} else {
				// Non-error case: check structural identity.
				if fmt.Sprint(unmarshalled) != fmt.Sprint(tc.input) {
					t.Errorf("expected non-error event %v, got %v", tc.input, unmarshalled)
				}
			}
		})
	}
}
