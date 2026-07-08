package event

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"context"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
)

func TestBasicEventBus(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	storedEvents := []Event{}

	storeMock := func(event Event) {
		storedEvents = append(storedEvents, event)
	}

	log := func(string, ...interface{}) {}

	bus := NewBasicEventBus(ctx, storeMock, log)

	t.Run("subscribe and publish", func(t *testing.T) {
		var mu sync.Mutex
		receivedEvents := []Event{}

		handler := func(e Event) error {
			mu.Lock()
			receivedEvents = append(receivedEvents, e)
			mu.Unlock()
			return nil
		}

		bus.subscribe(handler)
		bus.publish(Time{})

		if len(receivedEvents) != 1 {
			t.Errorf("expected 1 event, got %d", len(receivedEvents))
		}

		// Check the type of event.
		if _, ok := receivedEvents[0].(Time); !ok {
			t.Errorf("expected Time event, got %T", receivedEvents[0])
		}
	})

	t.Run("Multiple subscribers", func(t *testing.T) {
		var mu sync.Mutex
		receivedEvents := []Event{}

		handler1 := func(e Event) error {
			mu.Lock()
			receivedEvents = append(receivedEvents, e)
			mu.Unlock()
			return nil
		}

		handler2 := func(e Event) error {
			mu.Lock()
			receivedEvents = append(receivedEvents, e)
			mu.Unlock()
			return nil
		}

		bus.subscribe(handler1)
		bus.subscribe(handler2)
		bus.publish(Time{})

		if len(receivedEvents) != 2 {
			t.Errorf("expected 2 events, got %d", len(receivedEvents))
		}

		// Test type of events.
		if _, ok := receivedEvents[0].(Time); !ok {
			t.Errorf("expected Time, got %T", receivedEvents[0])
		}

		if _, ok := receivedEvents[1].(Time); !ok {
			t.Errorf("expected Time, got %T", receivedEvents[1])
		}
	})

	t.Run("Storing events after cancel", func(t *testing.T) {
		cancel() // cancel the context
		bus.publish(Start{})

		if len(storedEvents) != 1 {
			t.Errorf("expected 1 stored event, got %d", len(storedEvents))
		}

		// Test type of event.
		if _, ok := storedEvents[0].(Start); !ok {
			t.Errorf("expected Start event, got %T", storedEvents[0])
		}
	})

	t.Run("Panic on non-cancellable context", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("The code did not panic")
			}
		}()
		busNonCancelable := NewBasicEventBus(context.Background(), storeMock, log)
		busNonCancelable.publish(Start{})
	})
}

func TestStringToEvent(t *testing.T) {
	tests := []struct {
		name      string
		expected  Event
		wantPanic bool
	}{
		{"TimeEvent", Time{}, false},
		{"finishEvent", Finish{}, false},
		{"StartEvent", Start{}, false},
		{"startedEvent", Started{}, false},
		{"startFailedEvent", StartFailed{}, false},
		{"healthCheckDueEvent", HealthCheckDue{}, false},
		{"statusCheckDueEvent", StatusCheckDue{}, false},
		{"chatMessageDueEvent", ChatMessageDue{}, false},
		{"badHealthEvent", BadHealth{}, false},
		{"goodHealthEvent", GoodHealth{}, false},
		{"hardwareResetRequestEvent", HardwareResetRequest{}, false},
		{"hardwareStartFailedEvent", HardwareStartFailed{}, false},
		{"hardwareStopFailedEvent", HardwareStopFailed{}, false},
		{"hardwareStartedEvent", HardwareStarted{}, false},
		{"hardwareStoppedEvent", HardwareStopped{}, false},
		{"slateResetRequested", SlateResetRequested{}, false},
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

			got := StringToEvent(tt.name)
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

	warn := broadcast.WarnSkipShutdown

	tests := []testCase{
		{
			name:        "ShutdownEvent wraps broadcast.WarnSkipShutdown directly",
			err:         HardwareShutdownFailed{warn},
			target:      warn,
			expectMatch: true,
		},
		{
			name:        "ShutdownEvent wraps fmt.Errorf with broadcast.WarnSkipShutdown",
			err:         HardwareShutdownFailed{fmt.Errorf("could not perform shutdown actions: %w", warn)},
			target:      warn,
			expectMatch: true,
		},
		{
			name:        "PowerOffEvent wraps empty ShutdownEvent directly",
			err:         HardwarePowerOffFailed{HardwareShutdownFailed{}},
			target:      HardwareShutdownFailed{},
			expectMatch: true,
		},
		{
			name:        "PowerOffEvent wraps fmt.Errorf with empty ShutdownEvent",
			err:         HardwarePowerOffFailed{fmt.Errorf("got error event: %w", HardwareShutdownFailed{})},
			target:      HardwareShutdownFailed{},
			expectMatch: true,
		},
		{
			name:        "PowerOffEvent wraps fmt.Errorf with Shutdown{warn}",
			err:         HardwarePowerOffFailed{fmt.Errorf("got error event: %w", HardwareShutdownFailed{warn})},
			target:      HardwareShutdownFailed{},
			expectMatch: true,
		},
		{
			name:        "PowerOffEvent wraps fmt.Errorf with Shutdown{warn}, match on warn",
			err:         HardwarePowerOffFailed{fmt.Errorf("got error event: %w", HardwareShutdownFailed{warn})},
			target:      warn,
			expectMatch: true,
		},
		{
			name:        "PowerOffEvent wraps Shutdown{warn} directly",
			err:         HardwarePowerOffFailed{HardwareShutdownFailed{warn}},
			target:      HardwareShutdownFailed{},
			expectMatch: true,
		},
		{
			name:        "PowerOffEvent wraps Shutdown{warn} directly, match on warn",
			err:         HardwarePowerOffFailed{HardwareShutdownFailed{warn}},
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
		input       Event
		target      error // Only relevant if input is errorEvent
		expectMatch bool  // Only applies for errorEvent + target
	}{
		{
			name:        "Shutdown wraps warn",
			input:       HardwareShutdownFailed{broadcast.WarnSkipShutdown},
			target:      broadcast.WarnSkipShutdown,
			expectMatch: true,
		},
		{
			name:        "Shutdown wraps fmt.Errorf(warn)",
			input:       HardwareShutdownFailed{fmt.Errorf("wrap: %w", broadcast.WarnSkipShutdown)},
			target:      broadcast.WarnSkipShutdown,
			expectMatch: true,
		},
		{
			name:        "PowerOff wraps Shutdown{warn}",
			input:       HardwarePowerOffFailed{HardwareShutdownFailed{broadcast.WarnSkipShutdown}},
			target:      HardwareShutdownFailed{},
			expectMatch: true,
		},
		{
			name:        "PowerOff wraps fmt.Errorf(wrapped Shutdown{warn})",
			input:       HardwarePowerOffFailed{fmt.Errorf("wrap: %w", HardwareShutdownFailed{broadcast.WarnSkipShutdown})},
			target:      broadcast.WarnSkipShutdown,
			expectMatch: true,
		},
		{
			name:        "PowerOff wraps Shutdown with no cause",
			input:       HardwarePowerOffFailed{HardwareShutdownFailed{}},
			target:      HardwareShutdownFailed{},
			expectMatch: true,
		},
		{
			name:        "PowerOff wraps Shutdown with no cause – doesn't match warn",
			input:       HardwarePowerOffFailed{HardwareShutdownFailed{}},
			target:      broadcast.WarnSkipShutdown,
			expectMatch: false,
		},
		{
			name:        "Non-error Start round-trip",
			input:       Start{},
			target:      nil,
			expectMatch: false, // unused
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := MarshalEvent(tc.input)
			unmarshalled := UnmarshalEvent(data)

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
