package main

import (
	"testing"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
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
			svc:  newDummyService(),
			cfg: &BroadcastConfig{
				ID:  "1",
				SID: "2",
			},
			expectedReuse: false,
		},
		{
			name: "good status",
			svc:  newDummyGoodService(),
			cfg: &BroadcastConfig{
				ID:  "1",
				SID: "2",
			},
			expectedReuse: true,
		},
		{
			name: "empty ID good status",
			svc:  newDummyGoodService(),
			cfg: &BroadcastConfig{
				ID:  "",
				SID: "2",
			},
			expectedReuse: false,
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

// dummyGoodService is a dummy implementation of the BroadcastService interface.
// It mostly does nothing execpt return true for and is used to test the broadcast functions.
type dummyGoodService struct{}

func newDummyGoodService() *dummyGoodService { return &dummyGoodService{} }

func (d *dummyGoodService) CreateBroadcast(
	ctx Ctx,
	broadcastName, description, streamName, privacy, resolution string,
	start, end time.Time,
	opts ...BroadcastOption,
) (ServerResponse, broadcast.IDs, string, error) {
	return nil, broadcast.IDs{}, "", nil
}

func (d *dummyGoodService) StartBroadcast(
	name, bID, sID string,
	saveLink func(key, link string) error,
	extStart, extStop func() error,
	notify func(msg string) error,
	onLiveActions func() error,
) error {
	return nil
}
func (d *dummyGoodService) BroadcastStatus(ctx Ctx, id string) (string, error) {
	return "upcoming", nil
}
func (d *dummyGoodService) BroadcastHealth(ctx Ctx, id string) (string, error) { return "", nil }
func (d *dummyGoodService) RTMPKey(ctx Ctx, streamName string) (string, error) { return "", nil }
func (d *dummyGoodService) CompleteBroadcast(ctx Ctx, id string) error         { return nil }
func (d *dummyGoodService) PostChatMessage(id, msg string) error               { return nil }
