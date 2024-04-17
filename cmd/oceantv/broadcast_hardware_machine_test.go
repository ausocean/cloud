package main

import (
	"context"
	"reflect"
	"testing"
)

func TestGetHardwareStateStorage(t *testing.T) {
	tests := []struct {
		name         string
		initialState state
	}{
		{"test hardware off", newHardwareOff()},
		{"test hardware on", newHardwareOn()},
		{"test hardware starting", newHardwareStarting(&broadcastContext{camera: &dummyHardwareManager{}})},
		{"test hardware stopping", newHardwareStopping(&broadcastContext{})},
		{"test hardware restarting", newHardwareRestarting(&broadcastContext{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateStr := hardwareStateToString(tt.initialState)
			gotState := getHardwareState(&broadcastContext{cfg: &BroadcastConfig{HardwareState: stateStr}})
			if reflect.TypeOf(gotState) != reflect.TypeOf(tt.initialState) {
				t.Errorf("expected state %v, got %v", tt.initialState, gotState)
			}
		})
	}
}

func TestHandleHardwareStoppedEvent(t *testing.T) {
	bCtx := &broadcastContext{
		store:  &dummyStore{},
		svc:    &dummyService{},
		camera: &dummyHardwareManager{},
	}

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
	}{
		{
			desc:          "hardwareStopping transitions to hardwareOff",
			initialState:  newHardwareStopping(bCtx),
			expectedState: newHardwareOff(),
		},
		{
			desc:          "hardwareStarting transitions to hardwareOff",
			initialState:  newHardwareStarting(bCtx),
			expectedState: newHardwareOff(),
		},
		{
			desc:          "hardwareOn transitions to hardwareOff",
			initialState:  newHardwareOn(),
			expectedState: newHardwareOff(),
		},
		{
			desc:          "hardwareRestarting remains hardwareRestarting",
			initialState:  newHardwareRestarting(bCtx),
			expectedState: newHardwareRestarting(bCtx), // Assuming this state remains unchanged.
		},
		// Add other states and their transitions as needed.
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(msg string, args ...interface{}) {})

			bCtx.man = NewDummyManager(t)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &BroadcastConfig{}
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)
			bus.publish(hardwareStoppedEvent{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling stopped event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleHardwareStopFailedEvent(t *testing.T) {
	bCtx := &broadcastContext{
		store:  &dummyStore{},
		svc:    &dummyService{},
		camera: &dummyHardwareManager{},
	}

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
	}{
		{
			desc:          "hardwareStopping transitions to hardwareOn",
			initialState:  newHardwareStopping(bCtx),
			expectedState: newHardwareOn(),
		},
		{
			desc:          "hardwareRestarting transitions to hardwareOn",
			initialState:  newHardwareRestarting(bCtx),
			expectedState: newHardwareOn(),
		},
		{
			desc:          "hardwareStarting remains hardwareStarting",
			initialState:  newHardwareStarting(bCtx),
			expectedState: newHardwareStarting(bCtx), // Assuming this state remains unchanged.
		},
		// Add other states and their transitions as needed.
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(msg string, args ...interface{}) {})

			bCtx.man = NewDummyManager(t)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &BroadcastConfig{}
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)
			bus.publish(hardwareStopFailedEvent{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling stop failed event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleHardwareStartFailedEvent(t *testing.T) {
	bCtx := &broadcastContext{
		store:  &dummyStore{},
		svc:    &dummyService{},
		camera: &dummyHardwareManager{},
	}

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
	}{
		{
			desc:          "hardwareStarting transitions to hardwareOff",
			initialState:  newHardwareStarting(bCtx),
			expectedState: newHardwareOff(),
		},
		{
			desc:          "hardwareRestarting transitions to hardwareOff",
			initialState:  newHardwareRestarting(bCtx),
			expectedState: newHardwareOff(),
		},
		// For other states that don't match the above, you might want a generic test to ensure
		// they don't transition. Here's an example for hardwareOn:
		{
			desc:          "hardwareOn remains hardwareOn",
			initialState:  newHardwareOn(),
			expectedState: newHardwareOn(), // This assumes it remains unchanged.
		},
		// Add other states and their transitions as needed.
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(msg string, args ...interface{}) {})

			bCtx.man = NewDummyManager(t)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &BroadcastConfig{}
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)
			bus.publish(hardwareStartFailedEvent{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling start failed event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleHardwareStartedEvent(t *testing.T) {
	bCtx := &broadcastContext{
		store:  &dummyStore{},
		svc:    &dummyService{},
		camera: &dummyHardwareManager{},
	}

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
	}{
		{
			desc:          "hardwareStarting transitions to hardwareOn",
			initialState:  newHardwareStarting(bCtx),
			expectedState: newHardwareOn(),
		},
		{
			desc:          "hardwareRestarting transitions to hardwareOn",
			initialState:  newHardwareRestarting(bCtx),
			expectedState: newHardwareOn(),
		},
		// For other states that don't match the above, you might want a generic test to ensure
		// they don't transition. Here's an example for hardwareOff:
		{
			desc:          "hardwareOff remains hardwareOff",
			initialState:  newHardwareOff(),
			expectedState: newHardwareOff(), // This assumes it remains unchanged.
		},
		// Add other states and their transitions as needed.
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(msg string, args ...interface{}) {})

			bCtx.man = NewDummyManager(t)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &BroadcastConfig{}
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)
			bus.publish(hardwareStartedEvent{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling started event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleHardwareResetRequestEvent(t *testing.T) {
	bCtx := &broadcastContext{
		store:  &dummyStore{},
		svc:    &dummyService{},
		camera: &dummyHardwareManager{hardwareHealthy: true},
	}

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
	}{
		{
			desc:          "hardwareOn transitions to hardwareRestarting",
			initialState:  newHardwareOn(),
			expectedState: newHardwareRestarting(bCtx),
		},
		{
			desc:          "hardwareOff transitions to hardwareStarting",
			initialState:  newHardwareOff(),
			expectedState: newHardwareStarting(bCtx),
		},
		{
			desc:          "hardwareRestarting remains hardwareRestarting",
			initialState:  newHardwareRestarting(bCtx),
			expectedState: newHardwareRestarting(bCtx), // This assumes it remains unchanged.
		},
		{
			desc:          "hardwareStarting remains hardwareStarting",
			initialState:  newHardwareStarting(bCtx),
			expectedState: newHardwareStarting(bCtx), // This assumes it remains unchanged.
		},
		{
			desc:          "hardwareStopping remains hardwareStopping",
			initialState:  newHardwareStopping(bCtx),
			expectedState: newHardwareStopping(bCtx), // This assumes it remains unchanged.
		},
		// Add other states and their transitions as needed.
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(msg string, args ...interface{}) {})

			bCtx.man = NewDummyManager(t)
			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &BroadcastConfig{}
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)
			bus.publish(hardwareResetRequestEvent{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling reset request event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}
