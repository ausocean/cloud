package main

import (
	"testing"
	"time"

	"context"

	"bou.ke/monkey"
	"github.com/ausocean/cloud/notify"
)

func TestHandleTimeEvent(t *testing.T) {
	// Mock eventBus to capture published events

	bCtx := standardMockBroadcastContext(t, false)

	now := time.Now()
	tests := []struct {
		desc           string
		initialState   state
		event          event
		expectedEvents []event
		expectedState  state
		cfg            *BroadcastConfig
	}{
		{
			desc:           "vidforwardPermanentLive with time after End",
			initialState:   newVidforwardPermanentLive(),
			event:          timeEvent{now.Add(2 * time.Hour)}, // Assuming this is after cfg.End
			expectedEvents: []event{timeEvent{}, finishEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newVidforwardPermanentTransitionLiveToSlate(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardSecondaryLive with time after End",
			initialState:   newVidforwardSecondaryLive(bCtx),
			event:          timeEvent{now.Add(2 * time.Hour)}, // Assuming this is after cfg.End
			expectedEvents: []event{timeEvent{}, finishEvent{}, finishedEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newVidforwardSecondaryIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "directLive with time after End",
			initialState:   newDirectLive(bCtx),
			event:          timeEvent{now.Add(2 * time.Hour)}, // Assuming this is after cfg.End
			expectedEvents: []event{timeEvent{}, finishEvent{}, finishedEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newDirectIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentLiveUnhealthy with time after End",
			initialState:   newVidforwardPermanentLiveUnhealthy(bCtx),
			event:          timeEvent{now.Add(2 * time.Hour)}, // Assuming this is after cfg.End
			expectedEvents: []event{timeEvent{}, finishEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newVidforwardPermanentTransitionLiveToSlate(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardSecondaryLiveUnhealthy with time after End",
			initialState:   newVidforwardSecondaryLiveUnhealthy(),
			event:          timeEvent{now.Add(2 * time.Hour)}, // Assuming this is after cfg.End
			expectedEvents: []event{timeEvent{}, finishEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newVidforwardSecondaryIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "directLiveUnhealthy with time after End",
			initialState:   newDirectLiveUnhealthy(bCtx),
			event:          timeEvent{now.Add(2 * time.Hour)}, // Assuming this is after cfg.End
			expectedEvents: []event{timeEvent{}, finishEvent{}, finishedEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newDirectIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentIdle with time after end",
			initialState: newVidforwardPermanentIdle(bCtx),
			event:        timeEvent{now.Add(70 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
			},
			expectedState: newVidforwardPermanentIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardSecondaryIdle with time after end",
			initialState: newVidforwardSecondaryIdle(bCtx),
			event:        timeEvent{now.Add(70 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
			},
			expectedState: newVidforwardSecondaryIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentSlate with time after end",
			initialState: newVidforwardPermanentSlate(),
			event:        timeEvent{now.Add(70 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
			},
			expectedState: newVidforwardPermanentSlate(),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentSlateUnhealthy with time after end",
			initialState: newVidforwardPermanentSlateUnhealthy(bCtx),
			event:        timeEvent{now.Add(70 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
			},
			expectedState: newVidforwardPermanentSlateUnhealthy(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "directIdle with time after end",
			initialState: newDirectIdle(bCtx),
			event:        timeEvent{now.Add(70 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
			},
			expectedState: newDirectIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentStarting with time after end",
			initialState: &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(67*time.Minute))},
			event:        timeEvent{now.Add(70 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
			},
			expectedState: &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(67*time.Minute))},
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardSecondaryStarting with time after end",
			initialState: &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(67*time.Minute))},
			event:        timeEvent{now.Add(70 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
			},
			expectedState: &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(67*time.Minute))},
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "directStarting with time after end",
			initialState: &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(67*time.Minute))},
			event:        timeEvent{now.Add(70 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
			},
			expectedState: &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(67*time.Minute))},
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentLive within broadcast period",
			initialState: newVidforwardPermanentLive(),
			event:        timeEvent{now.Add(30 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
				statusCheckDueEvent{},
				chatMessageDueEvent{},
			},
			expectedState: newVidforwardPermanentLive(),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardSecondaryLive within broadcast period",
			initialState: newVidforwardSecondaryLive(bCtx),
			event:        timeEvent{now.Add(30 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
				statusCheckDueEvent{},
				chatMessageDueEvent{},
			},
			expectedState: newVidforwardSecondaryLive(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "directLive within broadcast period",
			initialState: newDirectLive(bCtx),
			event:        timeEvent{now.Add(30 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
				statusCheckDueEvent{},
				chatMessageDueEvent{},
			},
			expectedState: newDirectLive(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentLiveUnhealthy within broadcast period",
			initialState: newVidforwardPermanentLiveUnhealthy(bCtx),
			event:        timeEvent{now.Add(30 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
				statusCheckDueEvent{},
				chatMessageDueEvent{},
				hardwareResetRequestEvent{},
			},
			expectedState: newVidforwardPermanentLiveUnhealthy(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardSecondaryLiveUnhealthy within broadcast period",
			initialState: newVidforwardSecondaryLiveUnhealthy(),
			event:        timeEvent{now.Add(30 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
				statusCheckDueEvent{},
				chatMessageDueEvent{},
			},
			expectedState: newVidforwardSecondaryLiveUnhealthy(),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "directLiveUnhealthy within broadcast period",
			initialState: newDirectLiveUnhealthy(bCtx),
			event:        timeEvent{now.Add(30 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
				statusCheckDueEvent{},
				chatMessageDueEvent{},
				hardwareResetRequestEvent{},
			},
			expectedState: newDirectLiveUnhealthy(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardSecondaryIdle within broadcast period",
			initialState: newVidforwardSecondaryIdle(bCtx),
			event:        timeEvent{now.Add(30 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
				startEvent{},
				hardwareStartRequestEvent{},
			},
			expectedState: newVidforwardSecondaryStarting(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentIdle within broadcast period",
			initialState: newVidforwardPermanentIdle(bCtx),
			event:        timeEvent{now.Add(30 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
				startEvent{},
				hardwareStartRequestEvent{},
			},
			expectedState: newVidforwardPermanentStarting(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "vidforwardPermanentSlate within broadcast period",
			initialState: newVidforwardPermanentSlate(),
			event:        timeEvent{now.Add(30 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
				startEvent{},
				hardwareStartRequestEvent{},
			},
			expectedState: newVidforwardPermanentTransitionSlateToLive(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:         "directIdle in broadcastPeriod",
			initialState: newDirectIdle(bCtx),
			event:        timeEvent{now.Add(30 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
				startEvent{},
				hardwareStartRequestEvent{},
			},
			expectedState: newDirectStarting(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentStarting in broadcastPeriod",
			initialState:   &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(10*time.Minute))},
			event:          timeEvent{now.Add(14 * time.Minute)},
			expectedEvents: []event{timeEvent{}},
			expectedState:  &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(10*time.Minute))},
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardSecondaryStarting in broadcastPeriod",
			initialState:   &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(10*time.Minute))},
			event:          timeEvent{now.Add(14 * time.Minute)},
			expectedEvents: []event{timeEvent{}},
			expectedState:  &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(10*time.Minute))},
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "directStarting in broadcastPeriod",
			initialState:   &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(10*time.Minute))},
			event:          timeEvent{now.Add(14 * time.Minute)},
			expectedEvents: []event{timeEvent{}},
			expectedState:  &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now.Add(10*time.Minute))},
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentLive before start",
			initialState:   newVidforwardPermanentLive(),
			event:          timeEvent{now.Add(5 * time.Minute)},
			expectedEvents: []event{timeEvent{}, finishEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newVidforwardPermanentTransitionLiveToSlate(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardSecondaryLive before start",
			initialState:   newVidforwardSecondaryLive(bCtx),
			event:          timeEvent{now.Add(5 * time.Minute)},
			expectedEvents: []event{timeEvent{}, finishEvent{}, finishedEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newVidforwardSecondaryIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "directLive before start",
			initialState:   newDirectLive(bCtx),
			event:          timeEvent{now.Add(5 * time.Minute)},
			expectedEvents: []event{timeEvent{}, finishEvent{}, finishedEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newDirectIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentLiveUnhealthy before start",
			initialState:   newVidforwardPermanentLiveUnhealthy(bCtx),
			event:          timeEvent{now.Add(5 * time.Minute)},
			expectedEvents: []event{timeEvent{}, finishEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newVidforwardPermanentTransitionLiveToSlate(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardSecondaryLiveUnhealthy before start",
			initialState:   newVidforwardSecondaryLiveUnhealthy(),
			event:          timeEvent{now.Add(5 * time.Minute)},
			expectedEvents: []event{timeEvent{}, finishEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newVidforwardSecondaryIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "directLiveUnhealthy before start",
			initialState:   newDirectLiveUnhealthy(bCtx),
			event:          timeEvent{now.Add(5 * time.Minute)},
			expectedEvents: []event{timeEvent{}, finishEvent{}, finishedEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newDirectIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardSecondaryIdle before start",
			initialState:   newVidforwardSecondaryIdle(bCtx),
			event:          timeEvent{now.Add(5 * time.Minute)},
			expectedEvents: []event{timeEvent{}},
			expectedState:  newVidforwardSecondaryIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentIdle before start",
			initialState:   newVidforwardPermanentIdle(bCtx),
			event:          timeEvent{now.Add(5 * time.Minute)},
			expectedEvents: []event{timeEvent{}},
			expectedState:  newVidforwardPermanentIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentSlate before start",
			initialState:   newVidforwardPermanentSlate(),
			event:          timeEvent{now.Add(5 * time.Minute)},
			expectedEvents: []event{timeEvent{}},
			expectedState:  newVidforwardPermanentSlate(),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentSlateUnhealthy before start",
			initialState:   newVidforwardPermanentSlateUnhealthy(bCtx),
			event:          timeEvent{now.Add(5 * time.Minute)},
			expectedEvents: []event{timeEvent{}},
			expectedState:  newVidforwardPermanentSlateUnhealthy(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(10 * time.Minute),
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:           "vidforwardPermanentStarting timed out",
			initialState:   &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			event:          timeEvent{now.Add(6 * time.Minute)},
			expectedEvents: []event{timeEvent{}, startFailedEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newVidforwardPermanentIdle(bCtx),
			cfg:            &BroadcastConfig{},
		},
		{
			desc:           "vidforwardPermanentStarting not timed out",
			initialState:   &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			event:          timeEvent{now.Add(4 * time.Minute)},
			expectedEvents: []event{timeEvent{}},
			expectedState:  &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			cfg:            &BroadcastConfig{},
		},
		{
			desc:           "vidforwardSecondaryStarting timed out",
			initialState:   &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			event:          timeEvent{now.Add(6 * time.Minute)},
			expectedEvents: []event{timeEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newVidforwardSecondaryIdle(bCtx),
			cfg:            &BroadcastConfig{},
		},
		{
			desc:           "vidforwardSecondaryStarting not timed out",
			initialState:   &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			event:          timeEvent{now.Add(4 * time.Minute)},
			expectedEvents: []event{timeEvent{}},
			expectedState:  &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			cfg:            &BroadcastConfig{},
		},
		{
			desc:           "directStarting timed out",
			initialState:   &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			event:          timeEvent{now.Add(11 * time.Minute)},
			expectedEvents: []event{timeEvent{}, startFailedEvent{}, finishedEvent{}, hardwareStopRequestEvent{}},
			expectedState:  newDirectIdle(bCtx),
			cfg:            &BroadcastConfig{},
		},
		{
			desc:           "directStarting not timed out",
			initialState:   &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			event:          timeEvent{now.Add(4 * time.Minute)},
			expectedEvents: []event{timeEvent{}},
			expectedState:  &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(bCtx, now)},
			cfg:            &BroadcastConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var publishedEvents []event
			handler := func(e event) error {
				publishedEvents = append(publishedEvents, e)
				return nil
			}
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(string, ...interface{}) {})
			bus.subscribe(handler)

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)

			bus.publish(tt.event)

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

	now := time.Now()

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
		cfg           *BroadcastConfig
	}{
		{
			desc:          "vidforwardPermanentStarting",
			initialState:  newVidforwardPermanentStarting(bCtx),
			expectedState: newVidforwardPermanentIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryStarting",
			initialState:  newVidforwardSecondaryStarting(bCtx),
			expectedState: newVidforwardSecondaryIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "directStarting",
			initialState:  newDirectStarting(bCtx),
			expectedState: newDirectIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)

			bus.publish(startFailedEvent{})

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

	now := time.Now()

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
		cfg           *BroadcastConfig
	}{
		{
			desc:          "vidforwardPermanentLive",
			initialState:  newVidforwardPermanentLive(),
			expectedState: newVidforwardPermanentLiveUnhealthy(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentSlate",
			initialState:  newVidforwardPermanentSlate(),
			expectedState: newVidforwardPermanentSlateUnhealthy(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryLive",
			initialState:  newVidforwardSecondaryLive(bCtx),
			expectedState: newVidforwardSecondaryLiveUnhealthy(),
			cfg: &BroadcastConfig{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "directLive",
			initialState:  newDirectLive(bCtx),
			expectedState: newDirectLiveUnhealthy(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentLiveUnhealthy (no change)",
			initialState:  newVidforwardPermanentLiveUnhealthy(bCtx),
			expectedState: newVidforwardPermanentLiveUnhealthy(bCtx), // No transition expected
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentSlateUnhealthy (no change)",
			initialState:  newVidforwardPermanentSlateUnhealthy(bCtx),
			expectedState: newVidforwardPermanentSlateUnhealthy(bCtx), // No transition expected
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryLiveUnhealthy (no change)",
			initialState:  newVidforwardSecondaryLiveUnhealthy(),
			expectedState: newVidforwardSecondaryLiveUnhealthy(), // No transition expected
			cfg: &BroadcastConfig{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "directLiveUnhealthy (no change)",
			initialState:  newDirectLiveUnhealthy(bCtx),
			expectedState: newDirectLiveUnhealthy(bCtx), // No transition expected
			cfg: &BroadcastConfig{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)

			bus.publish(badHealthEvent{})

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

	now := time.Now()

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
		cfg           *BroadcastConfig
	}{
		{
			desc:          "vidforwardPermanentLiveUnhealthy",
			initialState:  newVidforwardPermanentLiveUnhealthy(bCtx),
			expectedState: newVidforwardPermanentLive(),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentSlateUnhealthy",
			initialState:  newVidforwardPermanentSlateUnhealthy(bCtx),
			expectedState: newVidforwardPermanentSlate(),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryLiveUnhealthy",
			initialState:  newVidforwardSecondaryLiveUnhealthy(),
			expectedState: newVidforwardSecondaryLive(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "directLiveUnhealthy",
			initialState:  newDirectLiveUnhealthy(bCtx),
			expectedState: newDirectLive(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentLive (no change)",
			initialState:  newVidforwardPermanentLive(),
			expectedState: newVidforwardPermanentLive(), // No transition expected
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentSlate (no change)",
			initialState:  newVidforwardPermanentSlate(),
			expectedState: newVidforwardPermanentSlate(), // No transition expected
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryLive (no change)",
			initialState:  newVidforwardSecondaryLive(bCtx),
			expectedState: newVidforwardSecondaryLive(bCtx), // No transition expected
			cfg: &BroadcastConfig{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "directLive (no change)",
			initialState:  newDirectLive(bCtx),
			expectedState: newDirectLive(bCtx), // No transition expected
			cfg: &BroadcastConfig{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)

			bus.publish(goodHealthEvent{})

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

	now := time.Now()
	tests := []struct {
		desc          string
		initialState  state
		expectedState state
		cfg           *BroadcastConfig
	}{
		{
			desc:          "vidforwardPermanentLive transitions to vidforwardPermanentTransitionLiveToSlate",
			initialState:  newVidforwardPermanentLive(),
			expectedState: newVidforwardPermanentTransitionLiveToSlate(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		// TODO: write test for LiveToSlate to Slate.
		{
			desc:          "vidforwardPermanentLiveUnhealthy transitions to vidforwardPermanentTransitionLiveToSlate",
			initialState:  newVidforwardPermanentLiveUnhealthy(bCtx),
			expectedState: newVidforwardPermanentTransitionLiveToSlate(bCtx),
			cfg: &BroadcastConfig{
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
			cfg: &BroadcastConfig{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryLiveUnhealthy transitions to vidforwardSecondaryIdle",
			initialState:  newVidforwardSecondaryLiveUnhealthy(),
			expectedState: newVidforwardSecondaryIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(3 * time.Hour),
				End:   now.Add(4 * time.Hour),
			},
		},
		{
			desc:          "directLive transitions to directIdle",
			initialState:  newDirectLive(bCtx),
			expectedState: newDirectIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(4 * time.Hour),
				End:   now.Add(5 * time.Hour),
			},
		},
		{
			desc:          "directLiveUnhealthy transitions to directIdle",
			initialState:  newDirectLiveUnhealthy(bCtx),
			expectedState: newDirectIdle(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(5 * time.Hour),
				End:   now.Add(6 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)

			bus.publish(finishEvent{})

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

	now := time.Now()
	tests := []struct {
		desc          string
		initialState  state
		expectedState state
		cfg           *BroadcastConfig
	}{
		{
			desc:          "vidforwardPermanentIdle transitions to vidforwardPermanentStarting",
			initialState:  newVidforwardPermanentIdle(bCtx),
			expectedState: newVidforwardPermanentStarting(bCtx),
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentSlate transitions to vidforwardPermanentLive",
			initialState:  newVidforwardPermanentSlate(),
			expectedState: newVidforwardPermanentTransitionSlateToLive(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryIdle transitions to vidforwardSecondaryStarting",
			initialState:  newVidforwardSecondaryIdle(bCtx),
			expectedState: newVidforwardSecondaryStarting(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
		{
			desc:          "directIdle transitions to directStarting",
			initialState:  newDirectIdle(bCtx),
			expectedState: newDirectStarting(bCtx),
			cfg: &BroadcastConfig{
				Start: now.Add(3 * time.Hour),
				End:   now.Add(4 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryStarting remains vidforwardSecondaryStarting",
			initialState:  newVidforwardSecondaryStarting(bCtx),
			expectedState: newVidforwardSecondaryStarting(bCtx), // No change
			cfg: &BroadcastConfig{
				Start: now.Add(4 * time.Hour),
				End:   now.Add(5 * time.Hour),
			},
		},
		{
			desc:          "vidforwardPermanentStarting remains vidforwardPermanentStarting",
			initialState:  newVidforwardPermanentStarting(bCtx),
			expectedState: newVidforwardPermanentStarting(bCtx), // No change
			cfg: &BroadcastConfig{
				Start: now.Add(5 * time.Hour),
				End:   now.Add(6 * time.Hour),
			},
		},
		{
			desc:          "directStarting remains directStarting",
			initialState:  newDirectStarting(bCtx),
			expectedState: newDirectStarting(bCtx), // No change
			cfg: &BroadcastConfig{
				Start: now.Add(6 * time.Hour),
				End:   now.Add(7 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)

			bus.publish(startEvent{})

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

	now := time.Now()
	tests := []struct {
		desc          string
		initialState  state
		expectedState state
		cfg           *BroadcastConfig
	}{
		{
			desc:          "vidforwardPermanentStarting transitions to vidforwardPermanentLive",
			initialState:  &vidforwardPermanentStarting{},
			expectedState: &vidforwardPermanentLive{},
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
		},
		{
			desc:          "vidforwardSecondaryStarting transitions to vidforwardSecondaryLive",
			initialState:  &vidforwardSecondaryStarting{},
			expectedState: &vidforwardSecondaryLive{},
			cfg: &BroadcastConfig{
				Start: now.Add(1 * time.Hour),
				End:   now.Add(2 * time.Hour),
			},
		},
		{
			desc:          "directStarting transitions to directLive",
			initialState:  &directStarting{},
			expectedState: &directLive{},
			cfg: &BroadcastConfig{
				Start: now.Add(2 * time.Hour),
				End:   now.Add(3 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}

			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)

			bus.publish(startedEvent{})

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
		cfg                      *BroadcastConfig
		initialState             state
		finalState               state
		hardwareMan              hardwareManager
		expectHardwareStartCall  bool
		expectBroadcastStartCall bool
		inputEvent               event
		expectedEvents           []event
	}{
		{
			desc: "direct broadcast successful start",
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
			initialState:             &directIdle{},
			finalState:               &directLive{},
			hardwareMan:              newDummyHardwareManager(),
			expectHardwareStartCall:  true,
			expectBroadcastStartCall: true,
			inputEvent:               timeEvent{now.Add(1 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
				startEvent{},
				hardwareStartRequestEvent{},
				timeEvent{},
				hardwareStartedEvent{},
				startedEvent{},
				statusCheckDueEvent{},
				chatMessageDueEvent{},
			},
		},
		{
			desc: "direct broadcast failed hardware start",
			cfg: &BroadcastConfig{
				Start: now,
				End:   now.Add(1 * time.Hour),
			},
			initialState:             &directIdle{},
			finalState:               &directStarting{},
			hardwareMan:              newDummyHardwareManager(withHardwareFault()),
			expectHardwareStartCall:  true,
			expectBroadcastStartCall: false,
			inputEvent:               timeEvent{now.Add(1 * time.Minute)},
			expectedEvents: []event{
				timeEvent{},
				startEvent{},
				hardwareStartRequestEvent{},
				timeEvent{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, t.Logf)

			// Record all events published.
			var gotEvents []event
			bus.subscribe(
				func(e event) error {
					gotEvents = append(gotEvents, e)
					return nil
				},
			)

			bCtx.man = newDummyManager(t, tt.cfg)
			bCtx.camera = tt.hardwareMan
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = tt.cfg
			bCtx.bus = bus

			hsm := newHardwareStateMachine(bCtx)
			bus.subscribe(hsm.handleEvent)

			sm, err := getBroadcastStateMachine(bCtx)
			if err != nil {
				t.Fatalf("failed to create state machine: %v", err)
			}
			bus.subscribe(sm.handleEvent)

			sm.currentState = tt.initialState
			bus.publish(tt.inputEvent)
			const timeEventInterval = 1 * time.Minute
			nextTimeEventTime := time.Now().Add(timeEventInterval)
			bus.publish(timeEvent{nextTimeEventTime})

			// Wait for broadcast start to complete, or timeout (if something failed).
			timeout := time.NewTimer(100 * time.Millisecond)
			select {
			case <-bCtx.man.(*dummyManager).startDone:
			case <-timeout.C:
				t.Log("timeout waiting for startDone")
			}

			// Check that the hardware manager start was called/not called as expected.
			startCalled := bCtx.camera.(*dummyHardwareManager).startCalled
			if tt.expectHardwareStartCall != startCalled {
				t.Errorf("hardware manager start was/was not called as expected, expected: %v, got: %v",
					tt.expectHardwareStartCall, bCtx.camera.(*dummyHardwareManager).startCalled)
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

func eventsToStringSlice(events []event) []string {
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
		cfg            func(*BroadcastConfig)
		initialState   state
		finalState     state
		expectedEvents []event
		expectedLogs   []string
		expectedNotify map[int64]map[notify.Kind][]string
	}{
		{
			desc: "unset camera config",
			cfg: func(c *BroadcastConfig) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.CameraMac = 0
			},
			initialState: &directIdle{},
			finalState:   &directFailure{},
			expectedEvents: []event{
				timeEvent{},
				startEvent{},
				hardwareStartRequestEvent{},
				invalidConfigurationEvent{},
				finishedEvent{},
				hardwareStopRequestEvent{},
			},
			expectedLogs: []string{
				"(invalidConfigurationEvent) camera mac is empty",
			},
			expectedNotify: map[int64]map[notify.Kind][]string{
				testSiteKey: {
					broadcastConfiguration: []string{
						"(name: , id: ) error event: (invalidConfigurationEvent) camera mac is empty",
						"(name: , id: ) entering direct broadcast failure state due to: (invalidConfigurationEvent) camera mac is empty",
					},
				},
			},
		},
		{
			desc: "set camera config",
			cfg: func(c *BroadcastConfig) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
			},
			initialState: &directIdle{},
			finalState:   &directLive{},
			expectedEvents: []event{
				timeEvent{},
				startEvent{},
				hardwareStartRequestEvent{},
				timeEvent{},
				timeEvent{},
				hardwareStartedEvent{},
				startedEvent{},
			},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			logRecorder := newLogRecorder(t)

			ctx, _ := context.WithCancel(context.Background())

			// Apply broadcast config modifications
			// and update the broadcast state based on the initial state.
			cfg := prepopulatedConfig()
			tt.cfg(cfg)
			updateBroadcastBasedOnState(tt.initialState, cfg)

			// Use a monkey patch to replace time.Now() with our own time.
			// This will be updated before each tick to simulate time passing.
			testTime := time.Now()
			monkey.Patch(time.Now, func() time.Time { return testTime })
			defer monkey.Unpatch(time.Now)

			sys, err := newBroadcastSystem(
				ctx,
				newDummyStore(),
				cfg,
				logRecorder.log,
				withEventBus(newMockEventBus(func(msg string, args ...interface{}) { logForBroadcast(cfg, logRecorder.log, msg, args...) })),
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

	timeEvents := func(n int) []event {
		var events []event
		for i := 0; i < n; i++ {
			events = append(events, timeEvent{})
		}
		return events
	}

	tests := []struct {
		desc                  string
		cfg                   func(*BroadcastConfig)
		initialBroadcastState state
		finalBroadcastState   state
		finalHardwareState    state
		hardwareMan           hardwareManager
		newBroadcastMan       func(*testing.T, *BroadcastConfig) BroadcastManager

		// Leave unset to use default max ticks.
		// Some tests may require more ticks to reach the final state.
		requiredTicks int

		expectedEvents []event
		expectedLogs   []string
		expectedNotify map[int64]map[notify.Kind][]string
	}{
		// Tests that the logic around handling low voltage is correct and
		// that we correctly enter the recovery state.
		{
			desc: "direct broadcast; start with low voltage, then enter recovery",
			cfg: func(c *BroadcastConfig) {
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
			newBroadcastMan: func(t *testing.T, c *BroadcastConfig) BroadcastManager {
				return newDummyManager(t, c)
			},
			expectedEvents: []event{timeEvent{}, startEvent{}, hardwareStartRequestEvent{}, lowVoltageEvent{}},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		// Tests that we can recover from the voltage recovery state.
		{
			desc: "direct broadcast; successful voltage recovery",
			cfg: func(c *BroadcastConfig) {
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
			newBroadcastMan: func(t *testing.T, c *BroadcastConfig) BroadcastManager {
				return newDummyManager(t, c)
			},
			requiredTicks: 60,
			expectedEvents: append(
				append(
					[]event{
						timeEvent{},
						startEvent{},
						hardwareStartRequestEvent{},
						lowVoltageEvent{},
					}, timeEvents(48)...),
				[]event{voltageRecoveredEvent{}}...,
			),
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		// Tests that we identify charging fault errors.
		{
			desc: "direct broadcast; charging fault",
			cfg: func(c *BroadcastConfig) {
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
			newBroadcastMan: func(t *testing.T, c *BroadcastConfig) BroadcastManager {
				return newDummyManager(t, c)
			},
			requiredTicks: 260,
			expectedEvents: append(
				append(
					[]event{
						timeEvent{},
						startEvent{},
						hardwareStartRequestEvent{},
						lowVoltageEvent{},
					}, timeEvents(241)...), // Time events to account for charging time.
				[]event{hardwareStartFailedEvent{}, startFailedEvent{}, finishedEvent{}, hardwareStopRequestEvent{}}...,
			),
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		// Tests that we can identify a controller fault i.e. voltage
		// last reported is OK, but controller is not reporting.
		{
			desc: "direct broadcast; controller fault",
			cfg: func(c *BroadcastConfig) {
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
			newBroadcastMan: func(t *testing.T, c *BroadcastConfig) BroadcastManager {
				return newDummyManager(t, c)
			},
			expectedEvents: []event{
				timeEvent{},
				startEvent{},
				hardwareStartRequestEvent{},
				controllerFailureEvent{},
				finishedEvent{},
				hardwareStopRequestEvent{},
			},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		// Tests that we can start a permanent broadcast and deal with a voltage
		// recovery i.e. idle -> live
		{
			desc: "permanent broadcast; broadcast start, successful voltage recovery",
			cfg: func(c *BroadcastConfig) {
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
			newBroadcastMan: func(t *testing.T, c *BroadcastConfig) BroadcastManager {
				return newDummyManager(t, c)
			},
			requiredTicks: 60,
			expectedEvents: append(
				append(
					[]event{
						timeEvent{},
						startEvent{},
						hardwareStartRequestEvent{},
						lowVoltageEvent{},
					}, timeEvents(48)...),
				[]event{voltageRecoveredEvent{}}...,
			),
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		// Tests that we transition to the permanent voltage recovery slate
		// state.
		{
			desc: "permanent broadcast; voltage recovery slate",
			cfg: func(c *BroadcastConfig) {
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
			newBroadcastMan: func(t *testing.T, c *BroadcastConfig) BroadcastManager {
				return newDummyManager(t, c)
			},
			requiredTicks: 60,
			expectedEvents: []event{
				timeEvent{},
				startEvent{},
				hardwareStartRequestEvent{},
				lowVoltageEvent{},
			},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},

		// Tests that we can recover from the voltage recovery state for a
		// permanent broadcast.
		{
			desc: "permanent broadcast; successful voltage recovery",
			cfg: func(c *BroadcastConfig) {
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
			newBroadcastMan: func(t *testing.T, c *BroadcastConfig) BroadcastManager {
				return newDummyManager(t, c)
			},
			requiredTicks: 60,
			expectedEvents: append(
				append(
					[]event{
						timeEvent{},
						startEvent{},
						hardwareStartRequestEvent{},
						lowVoltageEvent{},
					}, timeEvents(48)...),
				[]event{
					voltageRecoveredEvent{},
					hardwareStartRequestEvent{},
					timeEvent{},
					healthCheckDueEvent{},
					goodHealthEvent{},
					timeEvent{},
					hardwareStartedEvent{},
					timeEvent{},
					healthCheckDueEvent{},
					goodHealthEvent{},
					statusCheckDueEvent{},
					chatMessageDueEvent{},
				}...,
			),
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			logRecorder := newLogRecorder(t)

			ctx, _ := context.WithCancel(context.Background())

			// Apply broadcast config modifications
			// and update the broadcast state based on the initial state.
			cfg := prepopulatedConfig()
			tt.cfg(cfg)
			updateBroadcastBasedOnState(tt.initialBroadcastState, cfg)

			// Use a monkey patch to replace time.Now() with our own time.
			// This will be updated before each tick to simulate time passing.
			testTime := time.Now()
			monkey.Patch(time.Now, func() time.Time { return testTime })
			defer monkey.Unpatch(time.Now)

			sys, err := newBroadcastSystem(
				ctx,
				newDummyStore(),
				cfg,
				logRecorder.log,
				withEventBus(newMockEventBus(func(msg string, args ...interface{}) { logForBroadcast(cfg, logRecorder.log, msg, args...) })),
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

func prepopulatedConfig() *BroadcastConfig {
	return &BroadcastConfig{
		ShutdownActions: "shutdown",
		CameraMac:       2,
	}
}
