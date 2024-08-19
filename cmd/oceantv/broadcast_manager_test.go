package main

import (
	"testing"
	"time"
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
