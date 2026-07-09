package main

import (
	"testing"
	"time"

	"context"

	"bou.ke/monkey"
	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/cmd/oceantv/notification"
	"github.com/ausocean/cloud/notify"
)

func TestHandleTimeEvent(t *testing.T) {
	// Mock eventBus to capture published events

	bCtx := standardMockBroadcastContext(t, false)

	now := time.Now()
	tests := []struct {
		desc           string
		initialState   state
		e              event.Event
		expectedEvents []event.Event
		expectedState  state
		cfg            *Cfg
	}{
		{
			desc:           "vidforwardPermanentLive with time after End",
			initialState:   newVidforwardPermanentLive(),
			e:              event.Time{now.Add(2 * time.Hour)}, // Assuming this is after cfg.End
			expectedEvents: []event.Event{event.Time{}, event.Finish{}, event.HardwareStopRequest{}},
			expectedState:  newVidforwardPermanentTransitionLiveToSlate(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardSecondaryLive with time after End",
			initialState:   newVidforwardSecondaryLive(bCtx),
			e:              event.Time{now.Add(2 * time.Hour)}, // Assuming this is after cfg.End
			expectedEvents: []event.Event{event.Time{}, event.Finish{}, event.Finished{}, event.HardwareStopRequest{}},
			expectedState:  newVidforwardSecondaryIdle(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "directLive with time after End",
			initialState:   newDirectLive(bCtx),
			e:              event.Time{now.Add(2 * time.Hour)}, // Assuming this is after cfg.End
			expectedEvents: []event.Event{event.Time{}, event.Finish{}, event.Finished{}, event.HardwareStopRequest{}},
			expectedState:  newDirectIdle(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentLiveUnhealthy with time after End",
			initialState:   newVidforwardPermanentLiveUnhealthy(bCtx),
			e:              event.Time{now.Add(2 * time.Hour)}, // Assuming this is after cfg.End
			expectedEvents: []event.Event{event.Time{}, event.Finish{}, event.HardwareStopRequest{}},
			expectedState:  newVidforwardPermanentTransitionLiveToSlate(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardSecondaryLiveUnhealthy with time after End",
			initialState:   newVidforwardSecondaryLiveUnhealthy(),
			e:              event.Time{now.Add(2 * time.Hour)}, // Assuming this is after cfg.End
			expectedEvents: []event.Event{event.Time{}, event.Finish{}, event.HardwareStopRequest{}},
			expectedState:  newVidforwardSecondaryIdle(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "directLiveUnhealthy with time after End",
			initialState:   newDirectLiveUnhealthy(bCtx),
			e:              event.Time{now.Add(2 * time.Hour)}, // Assuming this is after cfg.End
			expectedEvents: []event.Event{event.Time{}, event.Finish{}, event.Finished{}, event.HardwareStopRequest{}},
			expectedState:  newDirectIdle(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentIdle with time after end",
			initialState: newVidforwardPermanentIdle(bCtx),
			e:            event.Time{now.Add(70 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
			},
			expectedState: newVidforwardPermanentIdle(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardSecondaryIdle with time after end",
			initialState: newVidforwardSecondaryIdle(bCtx),
			e:            event.Time{now.Add(70 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
			},
			expectedState: newVidforwardSecondaryIdle(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentSlate with time after end",
			initialState: newVidforwardPermanentSlate(),
			e:            event.Time{now.Add(70 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
			},
			expectedState: newVidforwardPermanentSlate(),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentSlateUnhealthy with time after end",
			initialState: newVidforwardPermanentSlateUnhealthy(bCtx),
			e:            event.Time{now.Add(70 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
			},
			expectedState: newVidforwardPermanentSlateUnhealthy(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "directIdle with time after end",
			initialState: newDirectIdle(bCtx),
			e:            event.Time{now.Add(70 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
			},
			expectedState: newDirectIdle(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentStarting with time after end",
			initialState: &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(67*time.Minute))},
			e:            event.Time{now.Add(70 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
			},
			expectedState: &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(67*time.Minute))},
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardSecondaryStarting with time after end",
			initialState: &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(67*time.Minute))},
			e:            event.Time{now.Add(70 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
			},
			expectedState: &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(67*time.Minute))},
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "directStarting with time after end",
			initialState: &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(67*time.Minute))},
			e:            event.Time{now.Add(70 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
			},
			expectedState: &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(67*time.Minute))},
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentLive within broadcast period",
			initialState: newVidforwardPermanentLive(),
			e:            event.Time{now.Add(30 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
				event.StatusCheckDue{},
				event.ChatMessageDue{},
			},
			expectedState: newVidforwardPermanentLive(),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardSecondaryLive within broadcast period",
			initialState: newVidforwardSecondaryLive(bCtx),
			e:            event.Time{now.Add(30 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
				event.StatusCheckDue{},
				event.ChatMessageDue{},
			},
			expectedState: newVidforwardSecondaryLive(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "directLive within broadcast period",
			initialState: newDirectLive(bCtx),
			e:            event.Time{now.Add(30 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
				event.StatusCheckDue{},
				event.ChatMessageDue{},
			},
			expectedState: newDirectLive(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentLiveUnhealthy within broadcast period",
			initialState: newVidforwardPermanentLiveUnhealthy(bCtx),
			e:            event.Time{now.Add(30 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
				event.StatusCheckDue{},
				event.ChatMessageDue{},
				event.HardwareResetRequest{},
			},
			expectedState: newVidforwardPermanentLiveUnhealthy(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardSecondaryLiveUnhealthy within broadcast period",
			initialState: newVidforwardSecondaryLiveUnhealthy(),
			e:            event.Time{now.Add(30 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
				event.StatusCheckDue{},
				event.ChatMessageDue{},
			},
			expectedState: newVidforwardSecondaryLiveUnhealthy(),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "directLiveUnhealthy within broadcast period",
			initialState: newDirectLiveUnhealthy(bCtx),
			e:            event.Time{now.Add(30 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
				event.StatusCheckDue{},
				event.ChatMessageDue{},
				event.HardwareResetRequest{},
			},
			expectedState: newDirectLiveUnhealthy(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardSecondaryIdle within broadcast period",
			initialState: newVidforwardSecondaryIdle(bCtx),
			e:            event.Time{now.Add(30 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
				event.Start{},
				event.HardwareStartRequest{},
			},
			expectedState: newVidforwardSecondaryStarting(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentIdle within broadcast period",
			initialState: newVidforwardPermanentIdle(bCtx),
			e:            event.Time{now.Add(30 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
				event.Start{},
				event.HardwareStartRequest{},
			},
			expectedState: newVidforwardPermanentStarting(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentSlate within broadcast period",
			initialState: newVidforwardPermanentSlate(),
			e:            event.Time{now.Add(30 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
				event.Start{},
				event.HardwareStartRequest{},
			},
			expectedState: newVidforwardPermanentTransitionSlateToLive(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "directIdle in broadcastPeriod",
			initialState: newDirectIdle(bCtx),
			e:            event.Time{now.Add(30 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
				event.Start{},
				event.HardwareStartRequest{},
			},
			expectedState: newDirectStarting(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentStarting in broadcastPeriod",
			initialState:   &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(10*time.Minute))},
			e:              event.Time{now.Add(14 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}},
			expectedState:  &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(10*time.Minute))},
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardSecondaryStarting in broadcastPeriod",
			initialState:   &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(10*time.Minute))},
			e:              event.Time{now.Add(14 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}},
			expectedState:  &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(10*time.Minute))},
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "directStarting in broadcastPeriod",
			initialState:   &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(10*time.Minute))},
			e:              event.Time{now.Add(14 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}},
			expectedState:  &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(10*time.Minute))},
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentLive before start",
			initialState:   newVidforwardPermanentLive(),
			e:              event.Time{now.Add(5 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}, event.Finish{}, event.HardwareStopRequest{}},
			expectedState:  newVidforwardPermanentTransitionLiveToSlate(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardSecondaryLive before start",
			initialState:   newVidforwardSecondaryLive(bCtx),
			e:              event.Time{now.Add(5 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}, event.Finish{}, event.Finished{}, event.HardwareStopRequest{}},
			expectedState:  newVidforwardSecondaryIdle(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "directLive before start",
			initialState:   newDirectLive(bCtx),
			e:              event.Time{now.Add(5 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}, event.Finish{}, event.Finished{}, event.HardwareStopRequest{}},
			expectedState:  newDirectIdle(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentLiveUnhealthy before start",
			initialState:   newVidforwardPermanentLiveUnhealthy(bCtx),
			e:              event.Time{now.Add(5 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}, event.Finish{}, event.HardwareStopRequest{}},
			expectedState:  newVidforwardPermanentTransitionLiveToSlate(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardSecondaryLiveUnhealthy before start",
			initialState:   newVidforwardSecondaryLiveUnhealthy(),
			e:              event.Time{now.Add(5 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}, event.Finish{}, event.HardwareStopRequest{}},
			expectedState:  newVidforwardSecondaryIdle(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "directLiveUnhealthy before start",
			initialState:   newDirectLiveUnhealthy(bCtx),
			e:              event.Time{now.Add(5 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}, event.Finish{}, event.Finished{}, event.HardwareStopRequest{}},
			expectedState:  newDirectIdle(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardSecondaryIdle before start",
			initialState:   newVidforwardSecondaryIdle(bCtx),
			e:              event.Time{now.Add(5 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}},
			expectedState:  newVidforwardSecondaryIdle(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentIdle before start",
			initialState:   newVidforwardPermanentIdle(bCtx),
			e:              event.Time{now.Add(5 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}},
			expectedState:  newVidforwardPermanentIdle(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentSlate before start",
			initialState:   newVidforwardPermanentSlate(),
			e:              event.Time{now.Add(5 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}},
			expectedState:  newVidforwardPermanentSlate(),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentSlateUnhealthy before start",
			initialState:   newVidforwardPermanentSlateUnhealthy(bCtx),
			e:              event.Time{now.Add(5 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}},
			expectedState:  newVidforwardPermanentSlateUnhealthy(bCtx),
			cfg: &Cfg{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentStarting timed out",
			initialState:   &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			e:              event.Time{now.Add(6 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}, event.StartFailed{}, event.HardwareStopRequest{}},
			expectedState:  newVidforwardPermanentIdle(bCtx),
			cfg:            &Cfg{},
		},
		{
			desc:           "vidforwardPermanentStarting not timed out",
			initialState:   &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			e:              event.Time{now.Add(4 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}},
			expectedState:  &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			cfg:            &Cfg{},
		},
		{
			desc:           "vidforwardSecondaryStarting timed out",
			initialState:   &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			e:              event.Time{now.Add(6 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}, event.HardwareStopRequest{}},
			expectedState:  newVidforwardSecondaryIdle(bCtx),
			cfg:            &Cfg{},
		},
		{
			desc:           "vidforwardSecondaryStarting not timed out",
			initialState:   &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			e:              event.Time{now.Add(4 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}},
			expectedState:  &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			cfg:            &Cfg{},
		},
		{
			desc:           "directStarting timed out",
			initialState:   &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			e:              event.Time{now.Add(11 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}, event.StartFailed{}, event.Finished{}, event.HardwareStopRequest{}},
			expectedState:  newDirectIdle(bCtx),
			cfg:            &Cfg{},
		},
		{
			desc:           "directStarting not timed out",
			initialState:   &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			e:              event.Time{now.Add(4 * time.Minute)},
			expectedEvents: []event.Event{event.Time{}},
			expectedState:  &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			cfg:            &Cfg{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var publishedEvents []event.Event
			handler := func(e event.Event) error {
				publishedEvents = append(publishedEvents, e)
				return nil
			}
			ctx, _ := context.WithCancel(context.Background())
			bus := event.NewBasicEventBus(ctx, nil, func(string, ...interface{}) {})
			bus.Subscribe(handler)

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.Subscribe(sm.handleEvent)

			bus.Publish(tt.e)

			if len(publishedEvents) != len(tt.expectedEvents) {
				t.Fatalf(
					"expected %d events, got %d, expected: %v, got: %v",
					len(tt.expectedEvents),
					len(publishedEvents),
					eventsToStringSlice(tt.expectedEvents),
					eventsToStringSlice(publishedEvents),
				)
			}

			// Check each published event
			for i, e := range publishedEvents {
				// Assuming you have an eventToString function
				if e.String() != tt.expectedEvents[i].String() {
					t.Errorf(
						"expected event %v, got %v, expected events: %v, got events: %v",
						tt.expectedEvents[i].String(),
						e.String(),
						eventsToStringSlice(tt.expectedEvents),
						eventsToStringSlice(publishedEvents),
					)
				}
			}

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling time event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
			timeout := time.NewTimer(200 * time.Millisecond)
			select {
			case <-bCtx.man.(*dummyManager).startDone:
			case <-timeout.C:
			}
		})
	}
}

func TestHandleStartFailedEvent(t *testing.T) {
	// Mock eventBus to capture published events

	bCtx := standardMockBroadcastContext(t, false)

	now := fixedBroadcastTestTime(t)
	monkey.Patch(time.Now, func() time.Time { return now })
	defer monkey.Unpatch(time.Now)

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
		cfg           *Cfg
	}{
		{
			desc:          "vidforwardPermanentStarting",
			initialState:  newVidforwardPermanentStarting(bCtx),
			expectedState: newVidforwardPermanentIdle(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryStarting",
			initialState:  newVidforwardSecondaryStarting(bCtx),
			expectedState: newVidforwardSecondaryIdle(bCtx),
			cfg: &Cfg{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "directStarting",
			initialState:  newDirectStarting(bCtx),
			expectedState: newDirectIdle(bCtx),
			cfg: &Cfg{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := event.NewBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.Subscribe(sm.handleEvent)

			bus.Publish(event.StartFailed{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling start failed event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleBadHealthEvent(t *testing.T) {
	// Mock eventBus to capture published events

	bCtx := standardMockBroadcastContext(t, false)

	now := fixedBroadcastTestTime(t)
	monkey.Patch(time.Now, func() time.Time { return now })
	defer monkey.Unpatch(time.Now)

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
		cfg           *Cfg
	}{
		{
			desc:          "vidforwardPermanentLive",
			initialState:  newVidforwardPermanentLive(),
			expectedState: newVidforwardPermanentLiveUnhealthy(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentSlate",
			initialState:  newVidforwardPermanentSlate(),
			expectedState: newVidforwardPermanentSlateUnhealthy(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryLive",
			initialState:  newVidforwardSecondaryLive(bCtx),
			expectedState: newVidforwardSecondaryLiveUnhealthy(),
			cfg: &Cfg{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "directLive",
			initialState:  newDirectLive(bCtx),
			expectedState: newDirectLiveUnhealthy(bCtx),
			cfg: &Cfg{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentLiveUnhealthy (no change)",
			initialState:  newVidforwardPermanentLiveUnhealthy(bCtx),
			expectedState: newVidforwardPermanentLiveUnhealthy(bCtx), // No transition expected
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentSlateUnhealthy (no change)",
			initialState:  newVidforwardPermanentSlateUnhealthy(bCtx),
			expectedState: newVidforwardPermanentSlateUnhealthy(bCtx), // No transition expected
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryLiveUnhealthy (no change)",
			initialState:  newVidforwardSecondaryLiveUnhealthy(),
			expectedState: newVidforwardSecondaryLiveUnhealthy(), // No transition expected
			cfg: &Cfg{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "directLiveUnhealthy (no change)",
			initialState:  newDirectLiveUnhealthy(bCtx),
			expectedState: newDirectLiveUnhealthy(bCtx), // No transition expected
			cfg: &Cfg{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := event.NewBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.Subscribe(sm.handleEvent)

			bus.Publish(event.BadHealth{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling bad health event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleGoodHealthEvent(t *testing.T) {
	// Mock eventBus to capture published events

	bCtx := standardMockBroadcastContext(t, false)

	now := fixedBroadcastTestTime(t)
	monkey.Patch(time.Now, func() time.Time { return now })
	defer monkey.Unpatch(time.Now)

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
		cfg           *Cfg
	}{
		{
			desc:          "vidforwardPermanentLiveUnhealthy",
			initialState:  newVidforwardPermanentLiveUnhealthy(bCtx),
			expectedState: newVidforwardPermanentLive(),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentSlateUnhealthy",
			initialState:  newVidforwardPermanentSlateUnhealthy(bCtx),
			expectedState: newVidforwardPermanentSlate(),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryLiveUnhealthy",
			initialState:  newVidforwardSecondaryLiveUnhealthy(),
			expectedState: newVidforwardSecondaryLive(bCtx),
			cfg: &Cfg{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "directLiveUnhealthy",
			initialState:  newDirectLiveUnhealthy(bCtx),
			expectedState: newDirectLive(bCtx),
			cfg: &Cfg{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentLive (no change)",
			initialState:  newVidforwardPermanentLive(),
			expectedState: newVidforwardPermanentLive(), // No transition expected
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentSlate (no change)",
			initialState:  newVidforwardPermanentSlate(),
			expectedState: newVidforwardPermanentSlate(), // No transition expected
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryLive (no change)",
			initialState:  newVidforwardSecondaryLive(bCtx),
			expectedState: newVidforwardSecondaryLive(bCtx), // No transition expected
			cfg: &Cfg{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "directLive (no change)",
			initialState:  newDirectLive(bCtx),
			expectedState: newDirectLive(bCtx), // No transition expected
			cfg: &Cfg{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := event.NewBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.Subscribe(sm.handleEvent)

			bus.Publish(event.GoodHealth{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling good health event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleFinishEvent(t *testing.T) {
	bCtx := standardMockBroadcastContext(t, false)

	now := fixedBroadcastTestTime(t)
	monkey.Patch(time.Now, func() time.Time { return now })
	defer monkey.Unpatch(time.Now)
	tests := []struct {
		desc          string
		initialState  state
		expectedState state
		cfg           *Cfg
	}{
		{
			desc:          "vidforwardPermanentLive transitions to vidforwardPermanentTransitionLiveToSlate",
			initialState:  newVidforwardPermanentLive(),
			expectedState: newVidforwardPermanentTransitionLiveToSlate(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		// TODO: write test for LiveToSlate to Slate.
		{
			desc:          "vidforwardPermanentLiveUnhealthy transitions to vidforwardPermanentTransitionLiveToSlate",
			initialState:  newVidforwardPermanentLiveUnhealthy(bCtx),
			expectedState: newVidforwardPermanentTransitionLiveToSlate(bCtx),
			cfg: &Cfg{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		// TODO: test slate to slate to live.
		// TODO: test slate unhealthy to slate to live.
		{
			desc:          "vidforwardSecondaryLive transitions to vidforwardSecondaryIdle",
			initialState:  newVidforwardSecondaryLive(bCtx),
			expectedState: newVidforwardSecondaryIdle(bCtx),
			cfg: &Cfg{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryLiveUnhealthy transitions to vidforwardSecondaryIdle",
			initialState:  newVidforwardSecondaryLiveUnhealthy(),
			expectedState: newVidforwardSecondaryIdle(bCtx),
			cfg: &Cfg{
				Start: now.Add(3 * time.Hour),
				End:   now.Add(4 * time.Hour),
			},
		},
		{
			desc:          "directLive transitions to directIdle",
			initialState:  newDirectLive(bCtx),
			expectedState: newDirectIdle(bCtx),
			cfg: &Cfg{
				Start: now.Add(4 * time.Hour),
				End:   now.Add(5 * time.Hour),
			},
		},
		{
			desc:          "directLiveUnhealthy transitions to directIdle",
			initialState:  newDirectLiveUnhealthy(bCtx),
			expectedState: newDirectIdle(bCtx),
			cfg: &Cfg{
				Start: now.Add(5 * time.Hour),
				End:   now.Add(6 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := event.NewBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.Subscribe(sm.handleEvent)

			bus.Publish(event.Finish{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling finish event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleStartEvent(t *testing.T) {
	bCtx := standardMockBroadcastContext(t, false)

	now := fixedBroadcastTestTime(t)
	monkey.Patch(time.Now, func() time.Time { return now })
	defer monkey.Unpatch(time.Now)
	tests := []struct {
		desc          string
		initialState  state
		expectedState state
		cfg           *Cfg
	}{
		{
			desc:          "vidforwardPermanentIdle transitions to vidforwardPermanentStarting",
			initialState:  newVidforwardPermanentIdle(bCtx),
			expectedState: newVidforwardPermanentStarting(bCtx),
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentSlate transitions to vidforwardPermanentLive",
			initialState:  newVidforwardPermanentSlate(),
			expectedState: newVidforwardPermanentTransitionSlateToLive(bCtx),
			cfg: &Cfg{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryIdle transitions to vidforwardSecondaryStarting",
			initialState:  newVidforwardSecondaryIdle(bCtx),
			expectedState: newVidforwardSecondaryStarting(bCtx),
			cfg: &Cfg{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
		{
			desc:          "directIdle transitions to directStarting",
			initialState:  newDirectIdle(bCtx),
			expectedState: newDirectStarting(bCtx),
			cfg: &Cfg{
				Start: now.Add(3 * time.Hour),
				End:   now.Add(4 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryStarting remains vidforwardSecondaryStarting",
			initialState:  newVidforwardSecondaryStarting(bCtx),
			expectedState: newVidforwardSecondaryStarting(bCtx), // No change
			cfg: &Cfg{
				Start: now.Add(4 * time.Hour),
				End:   now.Add(5 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentStarting remains vidforwardPermanentStarting",
			initialState:  newVidforwardPermanentStarting(bCtx),
			expectedState: newVidforwardPermanentStarting(bCtx), // No change
			cfg: &Cfg{
				Start: now.Add(5 * time.Hour),
				End:   now.Add(6 * time.Hour),
			},
		},
		{
			desc:          "directStarting remains directStarting",
			initialState:  newDirectStarting(bCtx),
			expectedState: newDirectStarting(bCtx), // No change
			cfg: &Cfg{
				Start: now.Add(6 * time.Hour),
				End:   now.Add(7 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := event.NewBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.Subscribe(sm.handleEvent)

			bus.Publish(event.Start{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling start event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleStartedEvent(t *testing.T) {
	bCtx := standardMockBroadcastContext(t, false)

	now := fixedBroadcastTestTime(t)
	monkey.Patch(time.Now, func() time.Time { return now })
	defer monkey.Unpatch(time.Now)
	tests := []struct {
		desc          string
		initialState  state
		expectedState state
		cfg           *Cfg
	}{
		{
			desc:          "vidforwardPermanentStarting transitions to vidforwardPermanentLive",
			initialState:  &vidforwardPermanentStarting{},
			expectedState: &vidforwardPermanentLive{},
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryStarting transitions to vidforwardSecondaryLive",
			initialState:  &vidforwardSecondaryStarting{},
			expectedState: &vidforwardSecondaryLive{},
			cfg: &Cfg{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "directStarting transitions to directLive",
			initialState:  &directStarting{},
			expectedState: &directLive{},
			cfg: &Cfg{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := event.NewBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.Subscribe(sm.handleEvent)

			bus.Publish(event.Started{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling started event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestBroadcastStart(t *testing.T) {
	t.Skip("todo(#425): fix obsolete test system setup in this test")

	bCtx := &broadcastContext{
		store:     &dummyStore{},
		svc:       &dummyService{},
		logOutput: t.Log,
		notifier:  newMockNotifier(),
	}

	now := time.Now()
	tests := []struct {
		desc                     string
		cfg                      *Cfg
		initialState             state
		finalState               state
		hardwareMan              hardwareManager
		expectHardwareStartCall  bool
		expectBroadcastStartCall bool
		inputEvent               event.Event
		expectedEvents           []event.Event
	}{
		{
			desc: "direct broadcast successful start",
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
			initialState:             &directIdle{},
			finalState:               &directLive{},
			hardwareMan:              newDummyHardwareManager(),
			expectHardwareStartCall:  true,
			expectBroadcastStartCall: true,
			inputEvent:               event.Time{now.Add(1 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
				event.Start{},
				event.HardwareStartRequest{},
				event.Time{},
				event.HardwareStarted{},
				event.Started{},
				event.StatusCheckDue{},
				event.ChatMessageDue{},
			},
		},
		{
			desc: "direct broadcast failed hardware start",
			cfg: &Cfg{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
			initialState:             &directIdle{},
			finalState:               &directStarting{},
			hardwareMan:              newDummyHardwareManager(withHardwareFault()),
			expectHardwareStartCall:  true,
			expectBroadcastStartCall: false,
			inputEvent:               event.Time{now.Add(1 * time.Minute)},
			expectedEvents: []event.Event{
				event.Time{},
				event.Start{},
				event.HardwareStartRequest{},
				event.Time{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := event.NewBasicEventBus(ctx, nil, t.Logf)

			// Record all events published.
			var gotEvents []event.Event
			bus.Subscribe(
				func(e event.Event) error {
					gotEvents = append(gotEvents, e)
					return nil
				},
			)

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.hardware = tt.hardwareMan
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			hsm := newHardwareStateMachine(bCtx)
			bus.Subscribe(hsm.handleEvent)

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}
			bus.Subscribe(sm.handleEvent)

			sm.currentState = tt.initialState
			bus.Publish(tt.inputEvent)
			const timeEventInterval = 1 * time.Minute
			nextTimeEventTime := time.Now().Add(timeEventInterval)
			bus.Publish(event.Time{nextTimeEventTime})

			// Wait for broadcast start to complete, or timeout (if something failed).
			timeout := time.NewTimer(100 * time.Millisecond)
			select {
			case <-bCtx.man.(*dummyManager).startDone:
			case <-timeout.C:
				t.Log("timeout waiting for startDone")
			}

			// Check that the hardware manager start was called/not called as expected.
			startCalled := bCtx.hardware.(*dummyHardwareManager).startCalled
			if tt.expectHardwareStartCall != startCalled {
				t.Errorf("hardware manager start was/was not called as expected, expected: %v, got: %v",
					tt.expectHardwareStartCall, bCtx.hardware.(*dummyHardwareManager).startCalled)
			}

			// Check that the broadcast manager start was called/not called as expected.
			startCalled = bCtx.man.(*dummyManager).started
			if tt.expectBroadcastStartCall != startCalled {
				t.Errorf("broadcast manager start was/was not called as expected, expected: %v, got: %v",
					tt.expectBroadcastStartCall, startCalled)
			}

			// Basic check on length of expected and actual events
			if len(gotEvents) != len(tt.expectedEvents) {
				t.Fatalf(
					"expected %d events, got %d, expected: %v, got: %v",
					len(tt.expectedEvents),
					len(gotEvents),
					eventsToStringSlice(tt.expectedEvents),
					eventsToStringSlice(gotEvents),
				)
			}

			// Check each published event matches the events we expected to see.
			for i, e := range gotEvents {
				// Assuming you have an eventToString function
				if e.String() != tt.expectedEvents[i].String() {
					t.Errorf(
						"expected event %v, got %v, expected events: %v, got events: %v",
						tt.expectedEvents[i].String(),
						e.String(),
						eventsToStringSlice(tt.expectedEvents),
						eventsToStringSlice(gotEvents),
					)
				}
			}

			// Let's make sure we ended up in the expected final state.
			if stateToString(sm.currentState) != stateToString(tt.finalState) {
				t.Errorf("unexpected state after handling started event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.finalState))
			}
		})
	}
}

func eventsToStringSlice(events []event.Event) []string {
	var result []string
	for _, e := range events {
		result = append(result, e.String())
	}
	return result
}

func TestHandleCameraConfiguration(t *testing.T) {
	const testSiteKey = 7845764367

	tests := []struct {
		desc           string
		cfg            func(*Cfg)
		initialState   state
		finalState     state
		expectedEvents []event.Event
		expectedLogs   []string
		expectedNotify map[int64]map[notify.Kind][]string
	}{
		{
			desc: "unset camera config",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.CameraMac = 0
			},
			initialState: &directIdle{},
			finalState:   &directFailure{},
			expectedEvents: []event.Event{
				event.Time{},
				event.Start{},
				event.HardwareStartRequest{},
				event.InvalidConfiguration{},
				event.Finished{},
				event.HardwareStopRequest{},
			},
			expectedLogs: []string{
				"(invalidConfigurationEvent) camera mac is empty",
			},
			expectedNotify: map[int64]map[notify.Kind][]string{
				testSiteKey: {
					notification.KindConfiguration: []string{
						"error event: (invalidConfigurationEvent) camera mac is empty",
						"entering direct broadcast failure state due to: (invalidConfigurationEvent) camera mac is empty",
					},
				},
			},
		},
		{
			desc: "set camera config",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
			},
			initialState: &directIdle{},
			finalState:   &directLive{},
			expectedEvents: []event.Event{
				event.Time{},
				event.Start{},
				event.HardwareStartRequest{},
				event.Time{},
				event.Time{},
				event.HardwareStarted{},
				event.Started{},
			},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			logRecorder := newLogRecorder(t)

			ctx, _ := context.WithCancel(context.Background())

			// Use a monkey patch to replace time.Now() with a stable test time.
			// This will be updated before each tick to simulate time passing.
			testTime := fixedBroadcastTestTime(t)
			monkey.Patch(time.Now, func() time.Time { return testTime })
			defer monkey.Unpatch(time.Now)

			// Apply broadcast config modifications
			// and update the broadcast state based on the initial state.
			cfg := prepopulatedConfig()
			tt.cfg(cfg)
			updateBroadcastBasedOnState(tt.initialState, cfg)

			sys, err := newBroadcastSystem(
				ctx,
				newDummyStore(),
				cfg,
				logRecorder.log,
				withEventBus(newMockEventBus(func(msg string, args ...interface{}) { broadcast.LogForBroadcast(cfg, logRecorder.log, msg, args...) })),
				withBroadcastManager(newDummyManager(t, cfg)),
				withBroadcastService(newDummyService()),
				withForwardingService(newDummyForwardingService()),
				withHardwareManager(newDummyHardwareManager(withMACSanitisation())),
				withNotifier(newMockNotifier()),
			)
			if err != nil {
				t.Fatalf("failed to create broadcast system: %v", err)
			}

			// Tick until we reach the final state. It's expected this occurs within
			// reasonable time otherwise we have a problem.
			const maxTicks = 10
			for tick := 0; true; tick++ {
				if tick > maxTicks {
					t.Errorf(
						"failed to reach expected state after %d ticks, current state: %s, wanted state: %s",
						maxTicks,
						stateToString(sys.sm.currentState),
						stateToString(tt.finalState),
					)
					return
				}
				err = sys.tick()
				if err != nil {
					t.Errorf("failed to tick broadcast system: %v", err)
					return
				}

				// We've replaced time.Now() with the monkey patch, but it means we need to
				// manually advance time before ticking the broadcast system.
				testTime = testTime.Add(1 * time.Minute)

				if stateToString(sys.sm.currentState) == stateToString(tt.finalState) {
					break
				}
			}

			// Check the events that we got.
			err = sys.ctx.bus.(*mockEventBus).checkEvents(tt.expectedEvents)
			if err != nil {
				t.Errorf("unexpected events: %v", err)
			}

			// Check the logs that we got.
			err = logRecorder.checkLogs(tt.expectedLogs)
			if err != nil {
				t.Errorf("unexpected logs: %v", err)
			}

			// Check we got expected notifications.
			err = sys.ctx.notifier.(*mockNotifier).checkNotifications(tt.expectedNotify)
			if err != nil {
				t.Errorf("unexpected notifications: %v", err)
			}

			// Let's make sure we ended up in the expected final state.
			if stateToString(sys.sm.currentState) != stateToString(tt.finalState) {
				t.Errorf("unexpected state after handling started event: got %v, want %v",
					stateToString(sys.sm.currentState), stateToString(tt.finalState))
			}
		})
	}
}

func TestHardwareVoltageAndFaultHandling(t *testing.T) {
	const testSiteKey = 7845764367

	timeEvents := func(n int) []event.Event {
		var events []event.Event
		for i := 0; i < n; i++ {
			events = append(events, event.Time{})
		}
		return events
	}

	tests := []struct {
		desc                  string
		cfg                   func(*Cfg)
		initialBroadcastState state
		finalBroadcastState   state
		finalHardwareState    state
		hardwareMan           hardwareManager
		newBroadcastMan       func(*testing.T, *Cfg) BroadcastManager

		// Leave unset to use default max ticks.
		// Some tests may require more ticks to reach the final state.
		requiredTicks int

		expectedEvents []event.Event
		expectedLogs   []string
		expectedNotify map[int64]map[notify.Kind][]string
	}{
		// Tests that the logic around handling low voltage is correct and
		// that we correctly enter the recovery state.
		{
			desc: "direct broadcast; start with low voltage, then enter recovery",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOff"
				c.ControllerMAC = 1
			},
			initialBroadcastState: &directIdle{},
			finalBroadcastState:   &directStarting{},
			finalHardwareState:    &hardwareRecoveringVoltage{},
			hardwareMan:           newDummyHardwareManager(withLowVoltage()),
			newBroadcastMan: func(t *testing.T, c *Cfg) BroadcastManager {
				return newDummyManager(t, c)
			},
			expectedEvents: []event.Event{event.Time{}, event.Start{}, event.HardwareStartRequest{}, event.LowVoltage{}},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		// Tests that we can recover from the voltage recovery state.
		{
			desc: "direct broadcast; successful voltage recovery",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOff"
				c.ControllerMAC = 1
			},
			initialBroadcastState: &directIdle{},
			finalBroadcastState:   &directStarting{},
			finalHardwareState:    &hardwareStarting{},
			hardwareMan:           newDummyHardwareManager(withLowVoltage()),
			newBroadcastMan: func(t *testing.T, c *Cfg) BroadcastManager {
				return newDummyManager(t, c)
			},
			requiredTicks: 60,
			expectedEvents: append(
				append(
					[]event.Event{
						event.Time{},
						event.Start{},
						event.HardwareStartRequest{},
						event.LowVoltage{},
					}, timeEvents(48)...),
				[]event.Event{event.VoltageRecovered{}}...,
			),
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		// Tests that we identify charging fault errors.
		{
			desc: "direct broadcast; charging fault",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOff"
				c.ControllerMAC = 1
			},
			initialBroadcastState: &directIdle{},
			finalBroadcastState:   &directIdle{},
			finalHardwareState:    &hardwareOff{},
			hardwareMan:           newDummyHardwareManager(withLowVoltage(), withChargingFault()),
			newBroadcastMan: func(t *testing.T, c *Cfg) BroadcastManager {
				return newDummyManager(t, c)
			},
			requiredTicks: 260,
			expectedEvents: append(
				append(
					[]event.Event{
						event.Time{},
						event.Start{},
						event.HardwareStartRequest{},
						event.LowVoltage{},
					}, timeEvents(241)...), // Time events to account for charging time.
				[]event.Event{event.HardwareStartFailed{}, event.StartFailed{}, event.Finished{}, event.HardwareStopRequest{}}...,
			),
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		// Tests that we can identify a controller fault i.e. voltage
		// last reported is OK, but controller is not reporting.
		{
			desc: "direct broadcast; controller fault",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOff"
				c.ControllerMAC = 1
			},
			initialBroadcastState: &directIdle{},
			finalBroadcastState:   &directFailure{},
			finalHardwareState:    &hardwareFailure{},
			hardwareMan:           newDummyHardwareManager(withHardwareFault()),
			newBroadcastMan: func(t *testing.T, c *Cfg) BroadcastManager {
				return newDummyManager(t, c)
			},
			expectedEvents: []event.Event{
				event.Time{},
				event.Start{},
				event.HardwareStartRequest{},
				event.ControllerFailure{},
				event.Finished{},
				event.HardwareStopRequest{},
			},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		// Tests that we can start a permanent broadcast and deal with a voltage
		// recovery i.e. idle -> live
		{
			desc: "permanent broadcast; broadcast start, successful voltage recovery",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOff"
				c.ControllerMAC = 1
			},
			initialBroadcastState: &vidforwardPermanentIdle{},
			finalBroadcastState:   &vidforwardPermanentStarting{},
			finalHardwareState:    &hardwareStarting{},
			hardwareMan:           newDummyHardwareManager(withLowVoltage()),
			newBroadcastMan: func(t *testing.T, c *Cfg) BroadcastManager {
				return newDummyManager(t, c)
			},
			requiredTicks: 60,
			expectedEvents: append(
				append(
					[]event.Event{
						event.Time{},
						event.Start{},
						event.HardwareStartRequest{},
						event.LowVoltage{},
					}, timeEvents(48)...),
				[]event.Event{event.VoltageRecovered{}}...,
			),
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		// Tests that we transition to the permanent voltage recovery slate
		// state.
		{
			desc: "permanent broadcast; voltage recovery slate",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOff"
				c.ControllerMAC = 1
			},
			initialBroadcastState: &vidforwardPermanentSlate{},
			finalBroadcastState:   &vidforwardPermanentVoltageRecoverySlate{},
			finalHardwareState:    &hardwareRecoveringVoltage{},
			hardwareMan:           newDummyHardwareManager(withLowVoltage()),
			newBroadcastMan: func(t *testing.T, c *Cfg) BroadcastManager {
				return newDummyManager(t, c)
			},
			requiredTicks: 60,
			expectedEvents: []event.Event{
				event.Time{},
				event.Start{},
				event.HardwareStartRequest{},
				event.LowVoltage{},
			},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		// Tests that we can recover from the voltage recovery state for a
		// permanent broadcast.
		{
			desc: "permanent broadcast; successful voltage recovery",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOff"
				c.ControllerMAC = 1
				c.CheckingHealth = true
			},
			initialBroadcastState: &vidforwardPermanentSlate{},
			finalBroadcastState:   &vidforwardPermanentLive{},
			finalHardwareState:    &hardwareOn{},
			hardwareMan:           newDummyHardwareManager(withLowVoltage()),
			newBroadcastMan: func(t *testing.T, c *Cfg) BroadcastManager {
				return newDummyManager(t, c)
			},
			requiredTicks: 60,
			expectedEvents: append(
				append(
					[]event.Event{
						event.Time{},
						event.Start{},
						event.HardwareStartRequest{},
						event.LowVoltage{},
					}, timeEvents(48)...),
				[]event.Event{
					event.VoltageRecovered{},
					event.HardwareStartRequest{},
					event.Time{},
					event.HealthCheckDue{},
					event.GoodHealth{},
					event.Time{},
					event.HardwareStarted{},
					event.Time{},
					event.HealthCheckDue{},
					event.GoodHealth{},
					event.StatusCheckDue{},
					event.ChatMessageDue{},
				}...,
			),
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		{
			desc: "direct broadcast; start with low voltage alarm error, then enter recovery",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOff"
				c.ControllerMAC = 1
			},
			initialBroadcastState: &directIdle{},
			finalBroadcastState:   &directStarting{},
			finalHardwareState:    &hardwareRecoveringVoltage{},
			hardwareMan:           newDummyHardwareManager(withLowVoltage(), withHardwareError(LowVoltageAlarm)),
			newBroadcastMan: func(t *testing.T, c *Cfg) BroadcastManager {
				return newDummyManager(t, c)
			},
			expectedEvents: []event.Event{event.Time{}, event.Start{}, event.HardwareStartRequest{}, event.LowVoltage{}},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		{
			desc: "direct broadcast; low voltage alarm error, successful voltage recovery",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOff"
				c.ControllerMAC = 1
			},
			initialBroadcastState: &directIdle{},
			finalBroadcastState:   &directStarting{},
			finalHardwareState:    &hardwareStarting{},
			hardwareMan:           newDummyHardwareManager(withLowVoltage(), withHardwareError(LowVoltageAlarm)),
			newBroadcastMan: func(t *testing.T, c *Cfg) BroadcastManager {
				return newDummyManager(t, c)
			},
			requiredTicks: 60,
			expectedEvents: append(
				append(
					[]event.Event{
						event.Time{},
						event.Start{},
						event.HardwareStartRequest{},
						event.LowVoltage{},
					}, timeEvents(49)...),
				[]event.Event{event.VoltageRecovered{}}...,
			),
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			logRecorder := newLogRecorder(t)

			ctx, _ := context.WithCancel(context.Background())

			// Use a monkey patch to replace time.Now() with a stable test time.
			// This will be updated before each tick to simulate time passing.
			testTime := fixedBroadcastTestTime(t)
			monkey.Patch(time.Now, func() time.Time { return testTime })
			defer monkey.Unpatch(time.Now)

			// Apply broadcast config modifications
			// and update the broadcast state based on the initial state.
			cfg := prepopulatedConfig()
			tt.cfg(cfg)
			updateBroadcastBasedOnState(tt.initialBroadcastState, cfg)

			sys, err := newBroadcastSystem(
				ctx,
				newDummyStore(),
				cfg,
				logRecorder.log,
				withEventBus(newMockEventBus(func(msg string, args ...interface{}) { broadcast.LogForBroadcast(cfg, logRecorder.log, msg, args...) })),
				withBroadcastManager(tt.newBroadcastMan(t, cfg)),
				withBroadcastService(newDummyService()),
				withForwardingService(newDummyForwardingService()),
				withHardwareManager(tt.hardwareMan),
				withNotifier(newMockNotifier()),
			)
			if err != nil {
				t.Fatalf("failed to create broadcast system: %v", err)
			}

			// Tick until we reach the final state. It's expected this occurs within
			// reasonable time otherwise we have a problem.
			const defaultMaxTicks = 10
			for tick := 0; true; tick++ {
				// Test test case overwrite the default max ticks.
				maxTicks := defaultMaxTicks
				if tt.requiredTicks > 0 {
					maxTicks = tt.requiredTicks
				}

				if tick > maxTicks {
					t.Errorf(
						"failed to reach expected state after %d ticks, current broadcast state: %s, wanted broadcast state: %s, current hardware state: %s, wanted hardware state: %s",
						maxTicks,
						stateToString(sys.sm.currentState),
						stateToString(tt.finalBroadcastState),
						stateToString(sys.hsm.currentState),
						stateToString(tt.finalHardwareState),
					)
					return
				}

				// We've replaced time.Now() with the monkey patch, but it means we need to
				// manually advance time before ticking the broadcast system.
				testTime = testTime.Add(1 * time.Minute)

				err = sys.tick()
				if err != nil {
					t.Errorf("failed to tick broadcast system: %v", err)
					return
				}
				if stateToString(sys.sm.currentState) == stateToString(tt.finalBroadcastState) &&
					stateToString(sys.hsm.currentState) == stateToString(tt.finalHardwareState) {
					break
				}
			}

			// Check the events that we got.
			err = sys.ctx.bus.(*mockEventBus).checkEvents(tt.expectedEvents)
			if err != nil {
				t.Errorf("unexpected events: %v", err)
			}

			// Check the logs that we got.
			err = logRecorder.checkLogs(tt.expectedLogs)
			if err != nil {
				t.Errorf("unexpected logs: %v", err)
			}

			// Check we got expected notifications.
			err = sys.ctx.notifier.(*mockNotifier).checkNotifications(tt.expectedNotify)
			if err != nil {
				t.Errorf("unexpected notifications: %v", err)
			}

			// Let's make sure we ended up in the expected final broadcast machine state.
			if stateToString(sys.sm.currentState) != stateToString(tt.finalBroadcastState) {
				t.Errorf("unexpected state after handling started event: got %v, want %v",
					stateToString(sys.sm.currentState), stateToString(tt.finalBroadcastState))
			}

			// Also check if we ended up with the correct broadcast hardware machine state.
			if stateToString(sys.hsm.currentState) != stateToString(tt.finalHardwareState) {
				t.Errorf("unexpected state after handling started event: got %v, want %v",
					stateToString(sys.hsm.currentState), stateToString(tt.finalHardwareState))
			}
		})
	}
}

func prepopulatedConfig() *Cfg {
	return &Cfg{
		ShutdownActions: "shutdown",
		CameraMac:       2,
	}
}
