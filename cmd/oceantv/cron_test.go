package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/broadcasthost"
	"github.com/ausocean/cloud/cmd/oceantv/composite"
	"github.com/ausocean/cloud/cmd/oceantv/manager"
	"github.com/ausocean/cloud/cmd/oceantv/notifier"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/google/uuid"
)

type recordingCronScheduler struct {
	jobs []model.Cron
	fail bool
}

func (s *recordingCronScheduler) Set(ctx Ctx, job *model.Cron) error {
	s.jobs = append(s.jobs, *job)
	if s.fail {
		return errors.New("scheduler unavailable")
	}
	return nil
}

type cronTestStore struct {
	Store
	queries int
}

func (s *cronTestStore) GetAll(ctx Ctx, query datastore.Query, dst interface{}) ([]*Key, error) {
	s.queries++
	return s.Store.GetAll(ctx, query, dst)
}

func cronFixture(t *testing.T) (*cronTestStore, *recordingCronScheduler) {
	t.Helper()
	model.RegisterEntities()
	db, err := datastore.NewStore(context.Background(), "file", "broadcast-crons", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tracked := &cronTestStore{Store: db}
	scheduler := &recordingCronScheduler{}
	oldStore, oldCrons, oldSecret, oldNotifier := store, broadcastCrons, cronSecret, notifier.N
	store = composite.AusOceanStore(tracked, tracked)
	broadcastCrons = &broadcastCronManager{store: store, scheduler: scheduler, endpoint: "https://tv.example/checkbroadcasts"}
	cronSecret = []byte("test-cron-secret")
	notifier.N = newMockNotifier()
	t.Cleanup(func() { store, broadcastCrons, cronSecret, notifier.N = oldStore, oldCrons, oldSecret, oldNotifier })
	return tracked, scheduler
}

func cronConfig() *Cfg {
	return &Cfg{UUID: uuid.NewString(), SKey: 42, Name: "Test", Enabled: true, Start: time.Now().Add(-time.Hour), End: time.Now().Add(time.Hour)}
}

func cronManager(t *testing.T, cfg *Cfg) *manager.OceanBroadcast {
	return newOceanBroadcastManager(nil, cfg, store, t.Logf)
}

func requireCron(t *testing.T, cfg *Cfg, enabled bool) *model.Cron {
	t.Helper()
	job, err := model.GetCron(context.Background(), store, cfg.SKey, broadcastCronID(cfg.UUID))
	if err != nil {
		t.Fatal(err)
	}
	if job.Enabled != enabled {
		t.Fatalf("cron enabled = %v, want %v", job.Enabled, enabled)
	}
	if job.TOD != "@every 15s" {
		t.Fatalf("unexpected interval: %s", job.TOD)
	}
	var payload broadcastCheckRequest
	if err := json.Unmarshal([]byte(job.Data), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.UUID != cfg.UUID {
		t.Fatalf("cron targets %s, want %s", payload.UUID, cfg.UUID)
	}
	return job
}

func saveCronConfig(t *testing.T, cfg *Cfg) {
	t.Helper()
	if err := cronManager(t, cfg).Save(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestBroadcastCronLifecycle(t *testing.T) {
	_, scheduler := cronFixture(t)
	ctx := context.Background()
	cfg := cronConfig()
	saveCronConfig(t, cfg)
	requireCron(t, cfg, true)
	man := cronManager(t, cfg)
	calls := len(scheduler.jobs)
	if err := man.Save(ctx, func(cfg *Cfg) { cfg.StartFailures++ }); err != nil {
		t.Fatal(err)
	}
	if len(scheduler.jobs) != calls {
		t.Fatal("routine state save touched the scheduler")
	}
	if err := man.Save(ctx, func(cfg *Cfg) { cfg.Enabled = false }); err != nil {
		t.Fatal(err)
	}
	requireCron(t, cfg, false)
	if scheduler.jobs[len(scheduler.jobs)-1].Enabled {
		t.Fatal("internal disable left job scheduled")
	}
	if err := man.Save(ctx, func(cfg *Cfg) { cfg.Enabled = true; cfg.Name = "Renamed" }); err != nil {
		t.Fatal(err)
	}
	requireCron(t, cfg, true)
	jobs, err := model.GetCronsBySite(ctx, store, cfg.SKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("rename created an extra job: %v", jobs)
	}
	if err := deleteBroadcastConfig(ctx, cfg.SKey, cfg.UUID); err != nil {
		t.Fatal(err)
	}
	if _, err := model.GetCron(ctx, store, cfg.SKey, broadcastCronID(cfg.UUID)); !errors.Is(err, datastore.ErrNoSuchEntity) {
		t.Fatalf("cron not deleted: %v", err)
	}
	if _, err := getBroadcastConfig(ctx, store, cfg.SKey, cfg.UUID); !errors.Is(err, datastore.ErrNoSuchEntity) {
		t.Fatalf("broadcast not deleted: %v", err)
	}
	if scheduler.jobs[len(scheduler.jobs)-1].Enabled {
		t.Fatal("deleted job remains scheduled")
	}
	if err := deleteBroadcastConfig(ctx, cfg.SKey, cfg.UUID); err != nil {
		t.Fatalf("delete retry failed: %v", err)
	}
}

func TestBroadcastCronSaveRetriesSchedulerFailure(t *testing.T) {
	_, scheduler := cronFixture(t)
	cfg := cronConfig()
	man := cronManager(t, cfg)
	scheduler.fail = true
	if err := man.Save(context.Background(), nil); err == nil {
		t.Fatal("scheduler failure was not reported")
	}
	scheduler.fail = false
	man = newOceanBroadcastManager(nil, cfg, store, t.Logf, manager.WithCronSyncOnSave())
	if err := man.Save(context.Background(), func(cfg *Cfg) { cfg.Description = "retry" }); err != nil {
		t.Fatal(err)
	}
	requireCron(t, cfg, true)
	if len(scheduler.jobs) != 2 {
		t.Fatalf("scheduler was not retried: %v", scheduler.jobs)
	}
}

func TestBroadcastCronDeleteRetriesSchedulerFailure(t *testing.T) {
	_, scheduler := cronFixture(t)
	ctx := context.Background()
	cfg := cronConfig()
	saveCronConfig(t, cfg)
	scheduler.fail = true
	if err := deleteBroadcastConfig(ctx, cfg.SKey, cfg.UUID); err == nil {
		t.Fatal("scheduler failure was not reported")
	}
	persisted, err := getBroadcastConfig(ctx, store, cfg.SKey, cfg.UUID)
	if err != nil {
		t.Fatalf("failed delete lost the configuration needed for retry: %v", err)
	}
	if persisted.Enabled {
		t.Fatal("failed delete left the broadcast enabled")
	}
	requireCron(t, cfg, false)
	scheduler.fail = false
	if err := deleteBroadcastConfig(ctx, cfg.SKey, cfg.UUID); err != nil {
		t.Fatal(err)
	}
	if _, err := model.GetCron(ctx, store, cfg.SKey, broadcastCronID(cfg.UUID)); !errors.Is(err, datastore.ErrNoSuchEntity) {
		t.Fatalf("delete retry retained the cron: %v", err)
	}
	if _, err := getBroadcastConfig(ctx, store, cfg.SKey, cfg.UUID); !errors.Is(err, datastore.ErrNoSuchEntity) {
		t.Fatalf("delete retry retained the broadcast: %v", err)
	}
}

func TestLegacySiteCronMigration(t *testing.T) {
	_, scheduler := cronFixture(t)
	ctx := context.Background()
	cfg, disabled, legacyCfg := cronConfig(), cronConfig(), cronConfig()
	disabled.Enabled = false
	legacyCfg.UUID = ""
	legacyCfg.Name = "Legacy"
	for _, cfg := range []*Cfg{cfg, disabled, legacyCfg} {
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		key := cfg.UUID
		if key == "" {
			key = cfg.Name
		}
		if err := model.PutVariable(ctx, store, cfg.SKey, broadcast.Scope+"."+key, string(data)); err != nil {
			t.Fatal(err)
		}
	}
	old := &model.Cron{Skey: 42, ID: legacyBroadcastCronID, TOD: "@every 15s", Action: "rpc", Var: "https://dev.example/checkbroadcasts", Enabled: true}
	if err := model.PutCron(ctx, store, old); err != nil {
		t.Fatal(err)
	}
	scheduler.fail = true
	if err := broadcastCrons.migrateSite(ctx, 42); err == nil {
		t.Fatal("failed replacement was not reported")
	}
	if job, err := model.GetCron(ctx, store, 42, legacyBroadcastCronID); err != nil || !job.Enabled {
		t.Fatalf("legacy job removed before replacements succeeded: %v", err)
	}
	scheduler.fail = false
	service, _ := newOceanTVService()
	service.check = func(Ctx, *Cfg, Store, []eventHook, []stateHook) error {
		t.Fatal("migration tick checked broadcasts")
		return nil
	}
	w := signedBroadcastCheck(t, service, 42, "")
	if w.Code != http.StatusOK {
		t.Fatalf("migration failed: %s", w.Body.String())
	}
	vars, err := model.GetVariablesBySite(ctx, store, 42, broadcast.Scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(vars) != 3 {
		t.Fatalf("migration lost or duplicated broadcasts: %d", len(vars))
	}
	for _, v := range vars {
		var cfg Cfg
		if err := json.Unmarshal([]byte(v.Value), &cfg); err != nil {
			t.Fatal(err)
		}
		if uuid.Validate(cfg.UUID) != nil {
			t.Fatalf("legacy broadcast has no UUID: %s", cfg.UUID)
		}
		job := requireCron(t, &cfg, cfg.Enabled)
		if job.Var != old.Var {
			t.Fatalf("migration changed deployment: %s", job.Var)
		}
	}
	if _, err := model.GetCron(ctx, store, 42, legacyBroadcastCronID); !errors.Is(err, datastore.ErrNoSuchEntity) {
		t.Fatal("legacy cron not deleted")
	}
	if _, err := model.GetVariable(ctx, store, 42, broadcast.Scope+".Legacy"); !errors.Is(err, datastore.ErrNoSuchEntity) {
		t.Fatal("legacy name key not removed")
	}
	if err := broadcastCrons.migrateSite(ctx, 42); err != nil {
		t.Fatalf("migration retry failed: %v", err)
	}
}

func TestSecondaryBroadcastHasIndependentCron(t *testing.T) {
	cronFixture(t)
	ctx := context.Background()
	cfg := cronConfig()
	cfg.UsingVidforward = true
	cfg.CameraMac = 2
	cfg.BID, cfg.SID, cfg.CID, cfg.RTMPKey, cfg.AuthKey = "primary-bid", "primary-sid", "primary-cid", "primary-rtmp", "primary-auth"
	cfg.StorageConfig = &broadcast.StorageConfig{Bucket: "bucket", Provider: "cloudflare", Prefix: "primary-prefix/"}
	saveCronConfig(t, cfg)
	primaryJob := requireCron(t, cfg, true)
	primaryJob.Var = "https://dev.example/checkbroadcasts"
	if err := model.PutCron(ctx, store, primaryJob); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"000000000002.HTTPAddress", "000000000002.Outputs"} {
		if err := model.PutVariable(ctx, store, cfg.SKey, name, ""); err != nil {
			t.Fatal(err)
		}
	}
	man := cronManager(t, cfg)
	if err := man.SetupSecondary(ctx); err != nil {
		t.Fatal(err)
	}
	secondary, err := broadcastByName(cfg.SKey, cfg.Name+broadcast.SecondaryPostfix)
	if err != nil {
		t.Fatal(err)
	}
	if job := requireCron(t, secondary, true); job.Var != primaryJob.Var {
		t.Fatal("secondary did not inherit the primary deployment")
	}
	requireCron(t, cfg, true)
	if err := man.SetupSecondary(ctx); err != nil {
		t.Fatal(err)
	}
	jobs, err := model.GetCronsBySite(ctx, store, cfg.SKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("secondary setup duplicated cron: %d", len(jobs))
	}
}

func TestBroadcastSavePreservesHostConfiguration(t *testing.T) {
	cronFixture(t)
	cfg := cronConfig()
	cfg.BroadcastHost = broadcasthost.OceanMediaHostName
	cfg.AuthKey = "generated-auth-key"
	cfg.StorageConfig = &broadcast.StorageConfig{Bucket: "old-bucket", Provider: "cloudflare", Prefix: "generated-prefix/", Endpoint: "https://storage.example"}
	saveCronConfig(t, cfg)
	incoming := *cfg
	incoming.AuthKey = ""
	incoming.StorageConfig = &broadcast.StorageConfig{Bucket: "new-bucket", Provider: "cloudflare"}
	incoming.AuthKeyVar, incoming.StorageConfigVar, incoming.CameraOutputVar = "camera.AuthKey", "camera.StorageConfig", "camera.Outputs"
	incoming.OnActions = broadcast.ActionVars{{Name: "camera.Mode", Value: "Normal,Shutdown"}}
	data, err := json.Marshal(incoming)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "https://tv.example/broadcast/save", strings.NewReader(string(data)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	broadcastHandler(w, r)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	stored, err := getBroadcastConfig(context.Background(), store, cfg.SKey, cfg.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.BroadcastHost != incoming.BroadcastHost || stored.AuthKey != cfg.AuthKey || stored.StorageConfig.Prefix != cfg.StorageConfig.Prefix || stored.StorageConfig.Endpoint != cfg.StorageConfig.Endpoint {
		t.Fatal("saving cron configuration lost generated host configuration")
	}
	if stored.StorageConfig.Bucket != incoming.StorageConfig.Bucket || stored.AuthKeyVar != incoming.AuthKeyVar || stored.StorageConfigVar != incoming.StorageConfigVar || stored.CameraOutputVar != incoming.CameraOutputVar {
		t.Fatal("saving cron configuration lost editable host fields")
	}
	if len(stored.OnActions) != 1 || stored.OnActions[0] != incoming.OnActions[0] {
		t.Fatal("saving did not preserve the structured action variables")
	}
	requireCron(t, stored, true)
}

func TestBroadcastSaveAndDeleteHandlersCoupleCron(t *testing.T) {
	for _, host := range []string{broadcasthost.YoutubeHostName, broadcasthost.OceanMediaHostName} {
		t.Run(host, func(t *testing.T) {
			cronFixture(t)
			cfg := cronConfig()
			cfg.BroadcastHost = host
			if host == broadcasthost.OceanMediaHostName {
				cfg.StorageConfig = &broadcast.StorageConfig{Bucket: "test", Provider: "cloudflare"}
			}
			call := func(op string) *httptest.ResponseRecorder {
				data, err := json.Marshal(cfg)
				if err != nil {
					t.Fatal(err)
				}
				r := httptest.NewRequest(http.MethodPost, "https://dev.example/broadcast/"+op, strings.NewReader(string(data)))
				r.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				broadcastHandler(w, r)
				return w
			}
			if w := call("save"); w.Code != http.StatusOK {
				t.Fatal(w.Body.String())
			}
			job := requireCron(t, cfg, true)
			if job.Var != "https://dev.example/checkbroadcasts" {
				t.Fatalf("wrong deployment: %s", job.Var)
			}
			if err := cronManager(t, cfg).Save(context.Background(), func(cfg *Cfg) {
				cfg.Active, cfg.AttemptingToStart, cfg.Transitioning = true, true, true
			}); err != nil {
				t.Fatal(err)
			}
			cfg.Enabled = false
			if w := call("save"); w.Code != http.StatusOK {
				t.Fatal(w.Body.String())
			}
			requireCron(t, cfg, false)
			stored, err := getBroadcastConfig(context.Background(), store, cfg.SKey, cfg.UUID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Active || stored.AttemptingToStart || stored.Transitioning {
				t.Fatal("disabling did not perform the final cleanup")
			}
			cfg.Enabled = true
			if w := call("reset-state"); w.Code != http.StatusOK {
				t.Fatal(w.Body.String())
			}
			requireCron(t, cfg, true)
			if w := call("delete"); w.Code != http.StatusOK {
				t.Fatal(w.Body.String())
			}
			if _, err := getBroadcastConfig(context.Background(), store, cfg.SKey, cfg.UUID); !errors.Is(err, datastore.ErrNoSuchEntity) {
				t.Fatal("delete handler retained config")
			}
		})
	}
}

func TestHTTPCronSchedulerSetsAndUnsets(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { paths = append(paths, r.URL.Path) }))
	defer server.Close()
	scheduler := httpCronScheduler{url: server.URL}
	job := &model.Cron{Skey: 42, ID: broadcastCronID(uuid.NewString()), Enabled: true}
	if err := scheduler.Set(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	job.Enabled = false
	if err := scheduler.Set(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "/cron/set/42/"+job.ID || paths[1] != "/cron/unset/42/"+job.ID {
		t.Fatalf("unexpected scheduler requests: %v", paths)
	}
}
