/*
AUTHORS
  Elliot Shine <elliot@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package hardware

import (
	"context"
	"errors"
	"testing"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/broadcasthost"
	"github.com/ausocean/cloud/cmd/oceantv/registry"
	"github.com/ausocean/cloud/datastore"
)

// testHost is a minimal broadcasthost.Host registered under a name that is not
// a supported broadcast host. It ensures extStart rejects registered-but-
// unsupported hosts rather than falling back to any host found in the registry.
type testHost struct {
	broadcasthost.Host
	protocol string
}

func (h *testHost) Name() string                      { return "revid-test-host" }
func (h *testHost) New(_ ...interface{}) (any, error) { return &testHost{protocol: "rtmp"}, nil }
func (h *testHost) Protocol() string                  { return h.protocol }

func init() {
	err := registry.Register(&testHost{})
	if err != nil {
		panic(err)
	}
}

// actionCapture records the actions passed to a fake SetActionVars function.
type actionCapture struct {
	acts []broadcast.ActionVar
	err  error
}

func newActionCapture() (*actionCapture, func(context.Context, int64, []broadcast.ActionVar, datastore.Store, func(string, ...interface{})) error) {
	c := &actionCapture{}
	set := func(_ context.Context, _ int64, acts []broadcast.ActionVar, _ datastore.Store, _ func(string, ...interface{})) error {
		c.acts = acts
		return c.err
	}
	return c, set
}

func TestExtStart(t *testing.T) {
	storageCfg := &broadcast.StorageConfig{Bucket: "bucket", Prefix: "prefix", Provider: "cloudflare"}

	tests := []struct {
		name    string
		cfg     *broadcast.Config
		wantLen int
		wantErr error
	}{
		{
			name:    "empty on actions is a no-op",
			cfg:     &broadcast.Config{BroadcastHost: "youtube"},
			wantLen: 0,
		},
		{
			name: "youtube host appends RTMP and output actions",
			cfg: &broadcast.Config{
				SKey:             1,
				OnActions:        broadcast.ActionVars{{Name: "ESP.Power2", Value: "true"}},
				RTMPVar:          "Camera.RTMPURL",
				RTMPKey:          "key",
				AuthKeyVar:       "Camera.AuthKey",
				AuthKey:          "auth",
				StorageConfigVar: "Camera.StorageConfig",
				StorageConfig:    storageCfg,
				CameraOutputVar:  "Camera.Output",
				BroadcastHost:    "youtube",
			},
			wantLen: 3,
		},
		{
			name: "oceanmedia host appends auth key, storage config and output actions",
			cfg: &broadcast.Config{
				SKey:             1,
				OnActions:        broadcast.ActionVars{{Name: "ESP.Power2", Value: "true"}},
				RTMPVar:          "Camera.RTMPURL",
				RTMPKey:          "key",
				AuthKeyVar:       "Camera.AuthKey",
				AuthKey:          "auth",
				StorageConfigVar: "Camera.StorageConfig",
				StorageConfig:    storageCfg,
				CameraOutputVar:  "Camera.Output",
				BroadcastHost:    "oceanmedia",
			},
			wantLen: 4,
		},
		{
			name: "oceanmedia nil storage config returns error",
			cfg: &broadcast.Config{
				SKey:            1,
				OnActions:       broadcast.ActionVars{{Name: "ESP.Power2", Value: "true"}},
				AuthKeyVar:      "Camera.AuthKey",
				AuthKey:         "auth",
				CameraOutputVar: "Camera.Output",
				BroadcastHost:   "oceanmedia",
			},
			wantErr: errors.New("storage config not set"),
		},
		{
			name: "unknown broadcast host returns error",
			cfg: &broadcast.Config{
				SKey:          1,
				OnActions:     broadcast.ActionVars{{Name: "ESP.Power2", Value: "true"}},
				BroadcastHost: "revid-test-host",
			},
			wantErr: errors.New("unknown broadcast host: revid-test-host"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, set := newActionCapture()
			err := extStart(context.Background(), nil, tt.cfg, func(string, ...interface{}) {}, set)
			if tt.wantErr != nil {
				if err == nil || err.Error() != tt.wantErr.Error() {
					t.Fatalf("extStart() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("extStart() error = %v", err)
			}
			if len(c.acts) != tt.wantLen {
				t.Fatalf("got %d actions, want %d: %+v", len(c.acts), tt.wantLen, c.acts)
			}
			if tt.wantLen == 0 {
				return
			}

			// The configured actions must come first, in order.
			if c.acts[0] != tt.cfg.OnActions[0] {
				t.Errorf("first action = %+v, want %+v", c.acts[0], tt.cfg.OnActions[0])
			}

			// The storage config must survive intact, including its commas.
			for _, a := range c.acts {
				if a.Name != "Camera.StorageConfig" {
					continue
				}
				if a.Value != storageCfg.JSON() {
					t.Errorf("storage config value = %q, want %q", a.Value, storageCfg.JSON())
				}
			}
		})
	}
}

func TestExtStartSetActionVarsError(t *testing.T) {
	wantErr := errors.New("boom")
	c, set := newActionCapture()
	c.err = wantErr

	cfg := &broadcast.Config{
		SKey:            1,
		OnActions:       broadcast.ActionVars{{Name: "ESP.Power2", Value: "true"}},
		RTMPVar:         "Camera.RTMPURL",
		AuthKeyVar:      "Camera.AuthKey",
		CameraOutputVar: "Camera.Output",
		BroadcastHost:   "youtube",
	}
	err := extStart(context.Background(), nil, cfg, func(string, ...interface{}) {}, set)
	if !errors.Is(err, wantErr) {
		t.Fatalf("extStart() error = %v, want %v", err, wantErr)
	}
}

func TestExtShutdown(t *testing.T) {
	tests := []struct {
		name    string
		acts    broadcast.ActionVars
		wantErr error
		wantLen int
	}{
		{
			name:    "skip",
			acts:    broadcast.ActionVars{{Name: broadcast.SkipAction}},
			wantErr: broadcast.WarnSkipShutdown,
		},
		{
			name:    "empty",
			acts:    nil,
			wantErr: ErrNoShutdownActions,
		},
		{
			name:    "actions",
			acts:    broadcast.ActionVars{{Name: "Camera.mode", Value: "Shutdown"}},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, set := newActionCapture()
			cfg := &broadcast.Config{ShutdownActions: tt.acts}
			err := extShutdown(context.Background(), nil, cfg, func(string, ...interface{}) {}, set)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("extShutdown() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("extShutdown() unexpected error: %v", err)
			}
			if len(c.acts) != tt.wantLen {
				t.Fatalf("got %d actions, want %d", len(c.acts), tt.wantLen)
			}
		})
	}
}

func TestExtStop(t *testing.T) {
	t.Run("empty is a no-op", func(t *testing.T) {
		c, set := newActionCapture()
		err := extStop(context.Background(), nil, &broadcast.Config{}, func(string, ...interface{}) {}, set)
		if err != nil {
			t.Fatalf("extStop() error = %v", err)
		}
		if len(c.acts) != 0 {
			t.Errorf("got %d actions, want 0", len(c.acts))
		}
	})

	t.Run("actions", func(t *testing.T) {
		c, set := newActionCapture()
		cfg := &broadcast.Config{OffActions: broadcast.ActionVars{{Name: "Camera.mode", Value: "Paused"}}}
		err := extStop(context.Background(), nil, cfg, func(string, ...interface{}) {}, set)
		if err != nil {
			t.Fatalf("extStop() error = %v", err)
		}
		if len(c.acts) != 1 {
			t.Fatalf("got %d actions, want 1", len(c.acts))
		}
	})
}
