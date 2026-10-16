package main

import (
	"context"
	"errors"
	"testing"

	"github.com/ausocean/cloud/cmd/oceantv/broadcasthost"
	"github.com/ausocean/cloud/cmd/oceantv/registry"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/google/uuid"
)

type deletionTestHost struct {
	*dummyService
	name        string
	unavailable bool
	complete    func(string) error
}

func (h *deletionTestHost) Name() string { return h.name }

func (h *deletionTestHost) New(...any) (any, error) {
	if h.unavailable {
		return nil, errors.New("host unavailable")
	}
	return h, nil
}

func (h *deletionTestHost) CompleteBroadcast(_ Ctx, id string) error {
	return h.complete(id)
}

func TestBroadcastDeleteCompletesLiveBroadcastBeforeRemovingConfiguration(t *testing.T) {
	for _, retry := range []bool{false, true} {
		name := "success"
		if retry {
			name = "retry after host unavailable"
		}
		t.Run(name, func(t *testing.T) {
			_, scheduler := cronFixture(t)
			ctx := context.Background()
			cfg := cronConfig()
			cfg.BID = "live-broadcast"
			cfg.Active, cfg.AttemptingToStart, cfg.Transitioning = true, true, true
			host := &deletionTestHost{dummyService: newDummyService(WithStatus(broadcasthost.StatusLive)), name: "delete-test-" + uuid.NewString()}
			cfg.BroadcastHost = host.Name()
			completed := 0
			host.complete = func(id string) error {
				completed++
				if id != cfg.BID {
					t.Fatalf("completed broadcast %q, want %q", id, cfg.BID)
				}
				persisted, err := getBroadcastConfig(ctx, store, cfg.SKey, cfg.UUID)
				if err != nil {
					t.Fatalf("configuration removed before live broadcast was completed: %v", err)
				}
				if persisted.Enabled || persisted.Active || persisted.AttemptingToStart || persisted.Transitioning {
					t.Fatal("broadcast was not disabled and cleared before completion")
				}
				requireCron(t, persisted, false)
				if scheduler.jobs[len(scheduler.jobs)-1].Enabled {
					t.Fatal("scheduler was not stopped before completion")
				}
				return nil
			}
			if err := registry.Register(host); err != nil {
				t.Fatal(err)
			}
			saveCronConfig(t, cfg)
			if retry {
				host.unavailable = true
				if err := deleteBroadcastConfig(ctx, cfg.SKey, cfg.UUID); err == nil {
					t.Fatal("host initialization failure was not reported")
				}
				persisted, err := getBroadcastConfig(ctx, store, cfg.SKey, cfg.UUID)
				if err != nil {
					t.Fatalf("failed cleanup removed the configuration: %v", err)
				}
				if persisted.Enabled || persisted.BID != cfg.BID {
					t.Fatal("failed cleanup did not preserve the disabled broadcast for retry")
				}
				requireCron(t, persisted, false)
				host.unavailable = false
			}
			if err := deleteBroadcastConfig(ctx, cfg.SKey, cfg.UUID); err != nil {
				t.Fatal(err)
			}
			if completed != 1 {
				t.Fatalf("live broadcast completed %d times, want 1", completed)
			}
			if _, err := getBroadcastConfig(ctx, store, cfg.SKey, cfg.UUID); !errors.Is(err, datastore.ErrNoSuchEntity) {
				t.Fatalf("completed broadcast configuration retained: %v", err)
			}
			if _, err := model.GetCron(ctx, store, cfg.SKey, broadcastCronID(cfg.UUID)); !errors.Is(err, datastore.ErrNoSuchEntity) {
				t.Fatalf("completed broadcast cron retained: %v", err)
			}
		})
	}
}
