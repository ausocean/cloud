package main

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestUpdateBroadcastBasedOnState(t *testing.T) {
	// Helper to construct test case StateData field.
	marshal := func(s state) []byte {
		d, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("failed to marshal state %s: %v", stateToString(s), err)
		}
		return d
	}

	tests := []struct {
		name        string
		state       state
		expectedCfg BroadcastConfig
	}{
		{
			name:  "vidforwardPermanentLive",
			state: &vidforwardPermanentLive{},
			expectedCfg: BroadcastConfig{
				Active:            true,
				Slate:             false,
				UsingVidforward:   true,
				AttemptingToStart: false,
				Unhealthy:         false,
				BroadcastState:    "main.vidforwardPermanentLive",
				StateData:         marshal(&vidforwardPermanentLive{}),
			},
		},
		{
			name:  "vidforwardPermanentLiveUnhealthy",
			state: &vidforwardPermanentLiveUnhealthy{},
			expectedCfg: BroadcastConfig{
				Active:            true,
				Slate:             false,
				UsingVidforward:   true,
				AttemptingToStart: false,
				Unhealthy:         true,
				BroadcastState:    "main.vidforwardPermanentLiveUnhealthy",
				StateData:         marshal(&vidforwardPermanentLiveUnhealthy{}),
			},
		},
		{
			name:  "vidforwardPermanentSlate",
			state: &vidforwardPermanentSlate{},
			expectedCfg: BroadcastConfig{
				Active:            true,
				Slate:             true,
				UsingVidforward:   true,
				AttemptingToStart: false,
				Unhealthy:         false,
				BroadcastState:    "main.vidforwardPermanentSlate",
				StateData:         marshal(&vidforwardPermanentSlate{}),
			},
		},
		{
			name:  "vidforwardPermanentSlateUnhealthy",
			state: &vidforwardPermanentSlateUnhealthy{},
			expectedCfg: BroadcastConfig{
				Active:            true,
				Slate:             true,
				UsingVidforward:   true,
				AttemptingToStart: false,
				Unhealthy:         true,
				BroadcastState:    "main.vidforwardPermanentSlateUnhealthy",
				StateData:         marshal(&vidforwardPermanentSlateUnhealthy{}),
			},
		},
		{
			name:  "vidforwardPermanentIdle",
			state: &vidforwardPermanentIdle{},
			expectedCfg: BroadcastConfig{
				Active:            false,
				Slate:             false,
				UsingVidforward:   true,
				AttemptingToStart: false,
				Unhealthy:         false,
				BroadcastState:    "main.vidforwardPermanentIdle",
				StateData:         marshal(&vidforwardPermanentIdle{}),
			},
		},
		{
			name:  "vidforwardPermanentStarting",
			state: &vidforwardPermanentStarting{},
			expectedCfg: BroadcastConfig{
				Active:            false,
				Slate:             false,
				UsingVidforward:   true,
				AttemptingToStart: true,
				Unhealthy:         false,
				BroadcastState:    "main.vidforwardPermanentStarting",
				StateData:         marshal(&vidforwardPermanentStarting{}),
			},
		},
		{
			name:  "vidforwardSecondaryLive",
			state: &vidforwardSecondaryLive{},
			expectedCfg: BroadcastConfig{
				Active:            true,
				Slate:             false,
				UsingVidforward:   true,
				AttemptingToStart: false,
				Unhealthy:         false,
				BroadcastState:    "main.vidforwardSecondaryLive",
				StateData:         marshal(&vidforwardSecondaryLive{}),
			},
		},
		{
			name:  "vidforwardSecondaryLiveUnhealthy",
			state: &vidforwardSecondaryLiveUnhealthy{},
			expectedCfg: BroadcastConfig{
				Active:            true,
				Slate:             false,
				UsingVidforward:   true,
				AttemptingToStart: false,
				Unhealthy:         true,
				BroadcastState:    "main.vidforwardSecondaryLiveUnhealthy",
				StateData:         marshal(&vidforwardSecondaryLiveUnhealthy{}),
			},
		},
		{
			name:  "vidforwardSecondaryIdle",
			state: &vidforwardSecondaryIdle{},
			expectedCfg: BroadcastConfig{
				Active:            false,
				Slate:             false,
				UsingVidforward:   true,
				AttemptingToStart: false,
				Unhealthy:         false,
				BroadcastState:    "main.vidforwardSecondaryIdle",
				StateData:         marshal(&vidforwardSecondaryIdle{}),
			},
		},
		{
			name:  "vidforwardSecondaryStarting",
			state: &vidforwardSecondaryStarting{},
			expectedCfg: BroadcastConfig{
				Active:            false,
				Slate:             false,
				UsingVidforward:   true,
				AttemptingToStart: true,
				Unhealthy:         false,
				BroadcastState:    "main.vidforwardSecondaryStarting",
				StateData:         marshal(&vidforwardSecondaryStarting{}),
			},
		},
		{
			name:  "directLive",
			state: &directLive{},
			expectedCfg: BroadcastConfig{
				Active:            true,
				Slate:             false,
				UsingVidforward:   false,
				AttemptingToStart: false,
				Unhealthy:         false,
				BroadcastState:    "main.directLive",
				StateData:         marshal(&directLive{}),
			},
		},
		{
			name:  "directLiveUnhealthy",
			state: &directLiveUnhealthy{},
			expectedCfg: BroadcastConfig{
				Active:            true,
				Slate:             false,
				UsingVidforward:   false,
				AttemptingToStart: false,
				Unhealthy:         true,
				BroadcastState:    "main.directLiveUnhealthy",
				StateData:         marshal(&directLiveUnhealthy{}),
			},
		},
		{
			name:  "directIdle",
			state: &directIdle{},
			expectedCfg: BroadcastConfig{
				Active:            false,
				Slate:             false,
				UsingVidforward:   false,
				AttemptingToStart: false,
				Unhealthy:         false,
				BroadcastState:    "main.directIdle",
				StateData:         marshal(&directIdle{}),
			},
		},
		{
			name:  "directStarting",
			state: &directStarting{},
			expectedCfg: BroadcastConfig{
				Active:            false,
				Slate:             false,
				UsingVidforward:   false,
				AttemptingToStart: true,
				Unhealthy:         false,
				BroadcastState:    "main.directStarting",
				StateData:         marshal(&directStarting{}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &BroadcastConfig{}
			updateBroadcastBasedOnState(tt.state, cfg)
			if !reflect.DeepEqual(*cfg, tt.expectedCfg) {
				t.Errorf("for state %v, expected cfg %v, got %v", tt.name, tt.expectedCfg, *cfg)
			}
		})
	}
}

func TestBroadcastCfgToState(t *testing.T) {
	ctx := minimalMockBroadcastContext(t)
	tests := []struct {
		name string
		cfg  BroadcastConfig
		want state
	}{
		{
			name: "Vidforward Permanent Live",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: true, Active: true, Slate: false, Unhealthy: false, AttemptingToStart: false, Transitioning: false},
			want: newVidforwardPermanentLive(),
		},
		{
			name: "Vidforward Permanent Transition Live To Slate",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: true, Active: true, Slate: false, Unhealthy: false, AttemptingToStart: false, Transitioning: true},
			want: newVidforwardPermanentTransitionLiveToSlate(ctx),
		},
		{
			name: "Vidforward Permanent Live Unhealthy",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: true, Active: true, Slate: false, Unhealthy: true, AttemptingToStart: false, Transitioning: false},
			want: newVidforwardPermanentLiveUnhealthy(ctx),
		},
		{
			name: "Vidforward Permanent Failure",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: true, Active: true, Slate: true, Unhealthy: false, AttemptingToStart: false, Transitioning: false, InFailure: true},
			want: newVidforwardPermanentFailure(ctx),
		},
		{
			name: "Vidforward Permanent Slate",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: true, Active: true, Slate: true, Unhealthy: false, AttemptingToStart: false, Transitioning: false},
			want: newVidforwardPermanentSlate(),
		},
		{
			name: "Vidforward Permanent Transition Slate To Live",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: true, Active: true, Slate: true, Unhealthy: false, AttemptingToStart: false, Transitioning: true},
			want: newVidforwardPermanentTransitionSlateToLive(ctx),
		},
		{
			name: "Vidforward Permanent Slate Unhealthy",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: true, Active: true, Slate: true, Unhealthy: true, AttemptingToStart: false, Transitioning: false},
			want: newVidforwardPermanentSlateUnhealthy(ctx),
		},
		{
			name: "Vidforward Permanent Idle",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: true, Active: false, Slate: false, Unhealthy: false, AttemptingToStart: false, Transitioning: false},
			want: newVidforwardPermanentIdle(ctx),
		},
		{
			name: "Vidforward Permanent Starting",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: true, Active: false, Slate: false, Unhealthy: false, AttemptingToStart: true, Transitioning: false},
			want: newVidforwardPermanentStarting(ctx),
		},
		{
			name: "Vidforward Secondary Live",
			cfg:  BroadcastConfig{Name: "Broadcast" + secondaryBroadcastPostfix, UsingVidforward: true, Active: true, Slate: false, Unhealthy: false, AttemptingToStart: false, Transitioning: false},
			want: newVidforwardSecondaryLive(ctx),
		},
		{
			name: "Vidforward Secondary Live Unhealthy",
			cfg:  BroadcastConfig{Name: "Broadcast" + secondaryBroadcastPostfix, UsingVidforward: true, Active: true, Slate: false, Unhealthy: true, AttemptingToStart: false, Transitioning: false},
			want: newVidforwardSecondaryLiveUnhealthy(),
		},
		{
			name: "Vidforward Secondary Idle",
			cfg:  BroadcastConfig{Name: "Broadcast" + secondaryBroadcastPostfix, UsingVidforward: true, Active: false, Slate: false, Unhealthy: false, AttemptingToStart: false, Transitioning: false},
			want: newVidforwardSecondaryIdle(ctx),
		},
		{
			name: "Vidforward Secondary Starting",
			cfg:  BroadcastConfig{Name: "Broadcast" + secondaryBroadcastPostfix, UsingVidforward: true, Active: false, Slate: false, Unhealthy: false, AttemptingToStart: true, Transitioning: false},
			want: newVidforwardSecondaryStarting(ctx),
		},
		{
			name: "Direct Live",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: false, Active: true, Slate: false, Unhealthy: false, AttemptingToStart: false, Transitioning: false},
			want: newDirectLive(ctx),
		},
		{
			name: "Direct Live Unhealthy",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: false, Active: true, Slate: false, Unhealthy: true, AttemptingToStart: false, Transitioning: false},
			want: newDirectLiveUnhealthy(ctx),
		},
		{
			name: "Direct Idle",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: false, Active: false, Slate: false, Unhealthy: false, AttemptingToStart: false, Transitioning: false},
			want: newDirectIdle(ctx),
		},
		{
			name: "Direct Starting",
			cfg:  BroadcastConfig{Name: "", UsingVidforward: false, Active: false, Slate: false, Unhealthy: false, AttemptingToStart: true, Transitioning: false},
			want: newDirectStarting(ctx),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx.cfg = &tt.cfg
			got := broadcastCfgToState(ctx)
			gotType := reflect.TypeOf(got)
			wantType := reflect.TypeOf(tt.want)
			if gotType != wantType {
				t.Errorf("broadcastCfgToState() = %v, want %v", gotType, wantType)
			}
		})
	}
}

func TestStateMarshalUnmarshal(t *testing.T) {
	ctx := minimalMockBroadcastContext(t)
	tests := []struct {
		desc  string
		s     state
		equal func(a, b state) bool
	}{
		{
			desc: "vidforwardPermanentStarting",
			s:    &vidforwardPermanentStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(ctx, time.Now())},
			equal: func(a, b state) bool {
				return a.(*vidforwardPermanentStarting).LastEntered.Equal(b.(*vidforwardPermanentStarting).LastEntered)
			},
		},
		{
			desc: "vidforwardPermanentLive",
			s:    &vidforwardPermanentLive{},
			equal: func(a, b state) bool {
				return true
			},
		},
		{
			desc: "vidforwardPermanentTransitionLiveToSlate",
			s:    &vidforwardPermanentTransitionLiveToSlate{stateWithTimeoutFields: stateWithTimeoutFields{broadcastContext: ctx}, HardwareStopped: true},
			equal: func(a, b state) bool {
				return a.(*vidforwardPermanentTransitionLiveToSlate).HardwareStopped == b.(*vidforwardPermanentTransitionLiveToSlate).HardwareStopped
			},
		},
		{
			desc: "vidforwardPermanentTransitionSlateToLive",
			s:    &vidforwardPermanentTransitionSlateToLive{stateWithTimeoutFields: stateWithTimeoutFields{broadcastContext: ctx}, HardwareStarted: true},
			equal: func(a, b state) bool {
				return a.(*vidforwardPermanentTransitionSlateToLive).HardwareStarted == b.(*vidforwardPermanentTransitionSlateToLive).HardwareStarted
			},
		},
		{
			desc: "vidforwardPermanentLiveUnhealthy",
			s:    &vidforwardPermanentLiveUnhealthy{broadcastContext: ctx, LastResetAttempt: time.Now()},
			equal: func(a, b state) bool {
				return a.(*vidforwardPermanentLiveUnhealthy).LastResetAttempt.Equal(b.(*vidforwardPermanentLiveUnhealthy).LastResetAttempt)
			},
		},
		{
			desc: "vidforwardPermanentFailure",
			s:    &vidforwardPermanentFailure{broadcastContext: ctx},
			equal: func(a, b state) bool {
				return true
			},
		},
		{
			desc:  "vidforwardPermanentSlate",
			s:     &vidforwardPermanentSlate{},
			equal: func(a, b state) bool { return true },
		},
		{
			desc: "vidforwardPermanentSlateUnhealthy",
			s:    &vidforwardPermanentSlateUnhealthy{broadcastContext: ctx, LastResetAttempt: time.Now()},
			equal: func(a, b state) bool {
				return a.(*vidforwardPermanentSlateUnhealthy).LastResetAttempt.Equal(b.(*vidforwardPermanentSlateUnhealthy).LastResetAttempt)
			},
		},
		{
			desc: "vidforwardPermanentIdle",
			s:    &vidforwardPermanentIdle{broadcastContext: ctx},
			equal: func(a, b state) bool {
				return true
			},
		},
		{
			desc: "vidforwardSecondaryLive",
			s:    &vidforwardSecondaryLive{broadcastContext: ctx},
			equal: func(a, b state) bool {
				return true
			},
		},
		{
			desc: "vidforwardSecondaryLiveUnhealthy",
			s:    &vidforwardSecondaryLiveUnhealthy{},
			equal: func(a, b state) bool {
				return true
			},
		},
		{
			desc: "vidforwardSecondaryStarting",
			s:    &vidforwardSecondaryStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(ctx, time.Time{})},
			equal: func(a, b state) bool {
				return true
			},
		},
		{
			desc: "vidforwardSecondaryIdle",
			s:    &vidforwardSecondaryIdle{broadcastContext: ctx},
			equal: func(a, b state) bool {
				return true
			},
		},
		{
			desc: "directLive",
			s:    &directLive{broadcastContext: ctx},
			equal: func(a, b state) bool {
				return true
			},
		},
		{
			desc: "directLiveUnhealthy",
			s:    newDirectLiveUnhealthy(ctx),
			equal: func(a, b state) bool {
				return true
			},
		},
		{
			desc: "directStarting",
			s:    &directStarting{stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(ctx, time.Time{})},
			equal: func(a, b state) bool {
				return true
			},
		},
		{
			desc: "directIdle",
			s:    &directIdle{broadcastContext: ctx},
			equal: func(a, b state) bool {
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var cfg BroadcastConfig
			updateBroadcastBasedOnState(tt.s, &cfg)
			state := broadcastCfgToState(&broadcastContext{cfg: &cfg, logOutput: t.Log, notifier: newMockNotifier()})
			if !tt.equal(tt.s, state) {
				t.Errorf("expected state %v, got %v", tt.s, state)
			}
		})
	}
}

// TestRateLimited tests the behaviour of a broadcast when it is being rate limited by a RateLimiter.
func TestRateLimited(t *testing.T) {

	tests := []struct {
		desc      string
		limited   bool
		expEvents []event
	}{
		{
			desc:      "Not rate Limited",
			limited:   false,
			expEvents: []event{hardwareStartRequestEvent{}},
		},
		{
			desc:      "Rate Limited",
			limited:   true,
			expEvents: []event{criticalFailureEvent{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			limiter := newMockLimiter(tt.limited)
			cfg := &BroadcastConfig{}
			bus := newMockEventBus(t.Logf)
			ctx := broadcastContext{
				cfg, newDummyManager(t, cfg, withRateLimiter(limiter)),
				newDummyStore(),
				newDummyService(),
				newDummyForwardingService(),
				bus,
				newDummyHardwareManager(),
				t.Log,
				newMockNotifier(),
			}
			createBroadcastAndRequestHardware(&ctx, cfg, nil)
			err := bus.checkEvents(tt.expEvents)
			if err != nil {
				t.Error(err)
			}
		})

	}

}
