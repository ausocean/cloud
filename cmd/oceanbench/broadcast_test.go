package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/google/uuid"
)

func TestDeleteBroadcastUsesOceanTV(t *testing.T) {
	ctx := context.Background()
	model.RegisterEntities()
	db, err := datastore.NewStore(ctx, "file", "broadcast-delete", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := broadcast.Config{UUID: uuid.NewString(), SKey: 42, Name: "Test"}
	key := broadcastScope + "." + cfg.UUID
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := model.PutVariable(ctx, db, cfg.SKey, key, string(data)); err != nil {
		t.Fatal(err)
	}
	status := http.StatusInternalServerError
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/broadcast/delete" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected delete request: %s %s", r.Method, r.URL.Path)
		}
		var sent broadcast.Config
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Error(err)
		}
		if sent.UUID != cfg.UUID || sent.SKey != cfg.SKey {
			t.Errorf("delete sent the wrong broadcast: %+v", sent)
		}
		if status == http.StatusOK {
			if err := model.DeleteVariable(r.Context(), db, sent.SKey, key); err != nil {
				t.Error(err)
			}
		}
		w.WriteHeader(status)
	}))
	defer backend.Close()
	oldURL := tvURL
	tvURL = backend.URL
	t.Cleanup(func() { tvURL = oldURL })
	req := &broadcastRequest{CurrentBroadcast: cfg}
	if err := deleteBroadcast(ctx, req, db); err == nil {
		t.Fatal("OceanTV failure was not reported")
	}
	if req.CurrentBroadcast.UUID != cfg.UUID {
		t.Fatal("failed delete cleared the form")
	}
	if _, err := model.GetVariable(ctx, db, cfg.SKey, key); err != nil {
		t.Fatalf("failed delete removed the configuration locally: %v", err)
	}
	status = http.StatusOK
	if err := deleteBroadcast(ctx, req, db); err != nil {
		t.Fatal(err)
	}
	if req.CurrentBroadcast.UUID != "" || len(req.BroadcastVars) != 0 {
		t.Fatal("successful delete did not refresh the form and broadcast list")
	}
	if _, err := model.GetVariable(ctx, db, cfg.SKey, key); !errors.Is(err, datastore.ErrNoSuchEntity) {
		t.Fatalf("backend did not delete the configuration: %v", err)
	}
}
