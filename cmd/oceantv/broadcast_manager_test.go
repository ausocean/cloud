package main

import (
	"context"
	"testing"
	"time"

	"github.com/ausocean/cloud/notify"
)

func TestBroadcastCanBeReused(t *testing.T) {
	tests := []struct {
		name          string
		svc           BroadcastService
		cfg           *BroadcastConfig
		expectedReuse bool
	}{
		{
			name: "empty status",
			svc:  newDummyService(WithStart(time.Now())), // DummyService always returns an empty status.
			cfg: &BroadcastConfig{
				ID:  "1",
				SID: "2",
			},
			expectedReuse: false,
		},
		{
			name: "good status",
			svc:  newDummyService(WithStart(time.Now()), WithStatus("upcoming")),
			cfg: &BroadcastConfig{
				ID:  "1",
				SID: "2",
			},
			expectedReuse: true,
		},
		{
			name: "empty ID, good status",
			svc:  newDummyService(WithStart(time.Now()), WithStatus("upcoming")),
			cfg: &BroadcastConfig{
				ID:  "",
				SID: "2",
			},
			expectedReuse: false,
		},
		{
			name: "good status, old broadcast",
			svc:  newDummyService(WithStart(time.Now().Add(-24*time.Hour)), WithStatus("upcoming")),
			cfg: &BroadcastConfig{
				ID:  "1",
				SID: "2",
			},
			expectedReuse: false,
		},
		{
			name: "good status, today's broadcast",
			svc:  newDummyService(WithStart(time.Now()), WithStatus("upcoming")),
			cfg: &BroadcastConfig{
				ID:  "1",
				SID: "2",
			},
			expectedReuse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newDummyStore()
			logFunc := func(msg string, args ...interface{}) { t.Logf(msg+"\n", args) }
			m := newOceanBroadcastManager(tt.svc, tt.cfg, store, logFunc)

			b := m.broadcastCanBeReused(m.cfg, m.svc)

			if b != tt.expectedReuse {
				t.Errorf("broadcastCanBeReused() test failed for %s: expected %v, got %v", tt.name, tt.expectedReuse, b)
			}
		})
	}
}

func TestCreateBroadcast(t *testing.T) {
	t.Skip("todo(#426): using obsolete setup code")
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
			desc: "create broadcast",
			cfg: func(c *BroadcastConfig) {
				c.CameraMac = 2
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.ID = "1"
				c.SID = "2"
			},
			initialState:   &directIdle{},
			finalState:     &directLive{},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			logRecorder := newLogRecorder(t)

			ctx, _ := context.WithCancel(context.Background())
			const hardwareHealthy = true

			// Apply broadcast config modifications
			// and update the broadcast state based on the initial state.
			cfg := &BroadcastConfig{}
			tt.cfg(cfg)
			updateBroadcastBasedOnState(tt.initialState, cfg)

			svc := newDummyService()

			limiter := &OceanTokenBucketLimiter{
				ID:             "test-limiter",
				Tokens:         10,
				MaxTokens:      10,
				RefillRate:     1,
				LastRefillTime: time.Now(),
			}

			store := newDummyStore(WithTokenBucketLimiter(limiter))
			bm := newOceanBroadcastManager(svc, cfg, store, t.Logf)

			sys, err := newBroadcastSystem(
				ctx,
				store,
				cfg,
				logRecorder.log,
				withEventBus(newMockEventBus(func(msg string, args ...interface{}) { logForBroadcast(cfg, logRecorder.log, msg, args...) })),
				withBroadcastManager(bm),
				withBroadcastService(svc),
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
					t.Errorf("failed to reach expected state after %d ticks, current state: %v, expected: %v", maxTicks, stateToString(sys.sm.currentState), stateToString(tt.finalState))
					return
				}
				t.Logf("tick %d: current state: %v", tick, stateToString(sys.sm.currentState))
				err = sys.tick()
				if err != nil {
					t.Errorf("failed to tick broadcast system: %v", err)
					return
				}
				if stateToString(sys.sm.currentState) == stateToString(tt.finalState) {
					break
				}
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
