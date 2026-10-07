package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
)

func signedBroadcastCheck(t *testing.T, service *oceanTVService, skey int64, body string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := gauth.PutClaims(map[string]interface{}{"iss": cronServiceAccount, "skey": skey}, cronSecret)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "https://tv.example/checkbroadcasts", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	service.checkBroadcastsHandler(w, r)
	return w
}

func TestBroadcastCheckLoadsOnlyRequestedBroadcast(t *testing.T) {
	db := broadcastFixture(t)
	oldSecret := cronSecret
	cronSecret = []byte("test-cron-secret")
	t.Cleanup(func() { cronSecret = oldSecret })
	cfg, other := testBroadcastConfig(), testBroadcastConfig()
	saveTestBroadcast(t, cfg)
	saveTestBroadcast(t, other)
	service, err := newOceanTVService()
	if err != nil {
		t.Fatal(err)
	}
	var checked []string
	service.check = func(ctx Ctx, cfg *Cfg, db Store, events []eventHook, states []stateHook) error {
		checked = append(checked, cfg.UUID)
		return nil
	}
	db.queries = 0
	w := signedBroadcastCheck(t, service, cfg.SKey, fmt.Sprintf(`{"UUID":%q}`, cfg.UUID))
	if w.Code != http.StatusOK {
		t.Fatalf("check failed: %s", w.Body.String())
	}
	if len(checked) != 1 || checked[0] != cfg.UUID {
		t.Fatalf("unexpected checks: %v", checked)
	}
	if db.queries != 0 {
		t.Fatal("checking one broadcast queried the site")
	}
	if err := testBroadcastManager(t, cfg).Save(context.Background(), func(cfg *Cfg) { cfg.Enabled = false }); err != nil {
		t.Fatal(err)
	}
	w = signedBroadcastCheck(t, service, cfg.SKey, fmt.Sprintf(`{"UUID":%q}`, cfg.UUID))
	if w.Code != http.StatusOK || len(checked) != 1 {
		t.Fatal("disabled broadcast was checked")
	}
	for _, test := range []struct {
		skey   int64
		body   string
		status int
	}{
		{43, fmt.Sprintf(`{"UUID":%q}`, cfg.UUID), http.StatusNotFound},
		{42, `{"UUID":"invalid"}`, http.StatusBadRequest},
		{42, `{}`, http.StatusBadRequest},
		{42, `{`, http.StatusBadRequest},
	} {
		if w := signedBroadcastCheck(t, service, test.skey, test.body); w.Code != test.status {
			t.Errorf("body %s returned %d, want %d", test.body, w.Code, test.status)
		}
	}
	w = httptest.NewRecorder()
	service.checkBroadcastsHandler(w, httptest.NewRequest(http.MethodPost, "/checkbroadcasts", strings.NewReader(`{}`)))
	if w.Code != http.StatusUnauthorized {
		t.Fatal("unauthenticated request accepted")
	}
}

func TestBroadcastCheckRetainsLegacySiteChecks(t *testing.T) {
	broadcastFixture(t)
	oldSecret := cronSecret
	cronSecret = []byte("test-cron-secret")
	t.Cleanup(func() { cronSecret = oldSecret })
	ctx := context.Background()
	cfg := testBroadcastConfig()
	cfg.Enabled = false
	cfg.Active, cfg.AttemptingToStart, cfg.Transitioning = true, true, true
	cfg.BroadcastHost = newDummyService().Name()
	saveTestBroadcast(t, cfg)
	if err := model.PutSite(ctx, store, &model.Site{Skey: cfg.SKey, Name: "Test"}); err != nil {
		t.Fatal(err)
	}
	service, err := newOceanTVService()
	if err != nil {
		t.Fatal(err)
	}
	w := signedBroadcastCheck(t, service, cfg.SKey, "")
	if w.Code != http.StatusOK {
		t.Fatalf("legacy check failed: %s", w.Body.String())
	}
	stored, err := getBroadcastConfig(ctx, store, cfg.SKey, cfg.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Active || stored.AttemptingToStart || stored.Transitioning {
		t.Fatal("legacy site check did not clean up the disabled broadcast")
	}
}
