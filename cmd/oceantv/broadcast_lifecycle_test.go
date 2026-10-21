package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/composite"
	"github.com/ausocean/cloud/cmd/oceantv/manager"
	"github.com/ausocean/cloud/cmd/oceantv/notifier"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/google/uuid"
)

type broadcastTestStore struct {
	Store
	queries int
}

func (s *broadcastTestStore) GetAll(ctx Ctx, query datastore.Query, dst interface{}) ([]*Key, error) {
	s.queries++
	return s.Store.GetAll(ctx, query, dst)
}

func broadcastFixture(t *testing.T) *broadcastTestStore {
	t.Helper()
	model.RegisterEntities()
	db, err := datastore.NewStore(context.Background(), "file", "broadcast-lifecycle", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tracked := &broadcastTestStore{Store: db}
	oldStore, oldNotifier := store, notifier.N
	store = composite.AusOceanStore(tracked, tracked)
	notifier.N = newMockNotifier()
	t.Cleanup(func() { store, notifier.N = oldStore, oldNotifier })
	return tracked
}

func testBroadcastConfig() *Cfg {
	return &Cfg{UUID: uuid.NewString(), SKey: 42, Name: "Test", Enabled: true, Start: time.Now().Add(-time.Hour), End: time.Now().Add(time.Hour)}
}

func testBroadcastManager(t *testing.T, cfg *Cfg) *manager.OceanBroadcast {
	t.Helper()
	return manager.NewOceanBroadcast(nil, cfg, store, t.Logf, setVar, broadcastByName)
}

func saveTestBroadcast(t *testing.T, cfg *Cfg) {
	t.Helper()
	if err := testBroadcastManager(t, cfg).Save(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestBroadcastSaveAssignsUUIDToLegacyConfiguration(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "replace"
		if partial {
			name = "update"
		}
		t.Run(name, func(t *testing.T) {
			broadcastFixture(t)
			ctx := context.Background()
			cfg := testBroadcastConfig()
			cfg.UUID = ""
			data, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			oldKey := broadcast.Scope + "." + cfg.Name
			if err := model.PutVariable(ctx, store, cfg.SKey, oldKey, string(data)); err != nil {
				t.Fatal(err)
			}
			var update func(*Cfg)
			if partial {
				update = func(cfg *Cfg) { cfg.Description = "updated" }
			}
			if err := testBroadcastManager(t, cfg).Save(ctx, update); err != nil {
				t.Fatal(err)
			}
			if uuid.Validate(cfg.UUID) != nil {
				t.Fatalf("save did not assign UUID: %q", cfg.UUID)
			}
			variable, err := model.GetVariable(ctx, store, cfg.SKey, broadcast.Scope+"."+cfg.UUID)
			if err != nil {
				t.Fatal(err)
			}
			var persisted Cfg
			if err := json.Unmarshal([]byte(variable.Value), &persisted); err != nil {
				t.Fatal(err)
			}
			if persisted.UUID != cfg.UUID || persisted.Name != cfg.Name || persisted.Description != cfg.Description {
				t.Fatal("stored configuration differs from manager configuration")
			}
			if _, err := model.GetVariable(ctx, store, cfg.SKey, oldKey); !errors.Is(err, datastore.ErrNoSuchEntity) {
				t.Fatalf("legacy key retained: %v", err)
			}
		})
	}
}

func TestSecondaryBroadcastIdentityAndReuse(t *testing.T) {
	broadcastFixture(t)
	ctx := context.Background()
	cfg := testBroadcastConfig()
	cfg.UsingVidforward = true
	cfg.CameraMac = 2
	cfg.BID, cfg.SID, cfg.CID, cfg.RTMPKey, cfg.AuthKey = "primary-bid", "primary-sid", "primary-cid", "primary-rtmp", "primary-auth"
	cfg.StorageConfig = &broadcast.StorageConfig{Bucket: "bucket", Provider: "cloudflare", Prefix: "primary-prefix/"}
	saveTestBroadcast(t, cfg)
	for _, name := range []string{"000000000002.HTTPAddress", "000000000002.Outputs"} {
		if err := model.PutVariable(ctx, store, cfg.SKey, name, ""); err != nil {
			t.Fatal(err)
		}
	}
	man := testBroadcastManager(t, cfg)
	if err := man.SetupSecondary(ctx); err != nil {
		t.Fatal(err)
	}
	secondary, err := broadcastByName(cfg.SKey, cfg.Name+broadcast.SecondaryPostfix)
	if err != nil {
		t.Fatal(err)
	}
	if secondary.UUID == cfg.UUID {
		t.Fatal("secondary reused primary UUID")
	}
	if secondary.BID != "" || secondary.SID != "" || secondary.CID != "" || secondary.RTMPKey != "" || secondary.AuthKey != "" || secondary.StorageConfig.Prefix != "" {
		t.Fatal("secondary inherited primary runtime credentials or identifiers")
	}
	if cfg.StorageConfig.Prefix != "primary-prefix/" {
		t.Fatal("creating secondary changed the primary storage configuration")
	}
	if secondary.BroadcastState != "vidforwardSecondaryIdle" || secondary.HardwareState != "hardwareOff" {
		t.Fatal("secondary did not start idle with hardware off")
	}
	secondaryUUID := secondary.UUID
	if err := man.SetupSecondary(ctx); err != nil {
		t.Fatal(err)
	}
	reused, err := broadcastByName(cfg.SKey, cfg.Name+broadcast.SecondaryPostfix)
	if err != nil {
		t.Fatal(err)
	}
	if reused.UUID != secondaryUUID {
		t.Fatal("secondary setup replaced the existing broadcast")
	}
	configs, err := model.GetVariablesBySite(ctx, store, cfg.SKey, broadcast.Scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 2 {
		t.Fatalf("secondary setup duplicated configuration: %d", len(configs))
	}
}
