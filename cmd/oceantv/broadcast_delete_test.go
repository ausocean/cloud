package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ausocean/cloud/datastore"
)

func TestBroadcastDeleteHandlerRemovesConfiguration(t *testing.T) {
	broadcastFixture(t)
	cfg, other := testBroadcastConfig(), testBroadcastConfig()
	saveTestBroadcast(t, cfg)
	saveTestBroadcast(t, other)
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/broadcast/delete", strings.NewReader(string(data)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		broadcastHandler(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("delete returned %d: %s", w.Code, w.Body.String())
		}
	}
	ctx := context.Background()
	if _, err := getBroadcastConfig(ctx, store, cfg.SKey, cfg.UUID); !errors.Is(err, datastore.ErrNoSuchEntity) {
		t.Fatalf("deleted configuration retained: %v", err)
	}
	if _, err := getBroadcastConfig(ctx, store, other.SKey, other.UUID); err != nil {
		t.Fatalf("delete removed another broadcast: %v", err)
	}
}
