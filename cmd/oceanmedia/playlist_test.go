/*
DESCRIPTION
  Tests for OceanMedia playlist handling.

AUTHORS
  Elliot Shine <elliot@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean).

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
  or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
  License for more details.

  You should have received a copy of the GNU General Public License in
  gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package main

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBroadcastID = "c7cfd21b-d69d-437f-969c-34cd4ac15bd5"
	testSegmentURL  = "https://acct.r2.cloudflarestorage.com/bucket/" + testBroadcastID + "/seg%d.ts"
)

var testJWTSecret = []byte("test-jwt-secret")

// fakeSigner returns deterministic signed URLs, rejecting invalid URIs.
type fakeSigner struct{}

func (fakeSigner) SignURL(_ context.Context, rawURI string, _ time.Duration) (string, error) {
	if !strings.HasPrefix(rawURI, "https://") {
		return "", fmt.Errorf("invalid storage URI %q", rawURI)
	}
	return "https://signed.example.com/" + rawURI + "?sig=x", nil
}

// countingSigner counts SignURL calls for cache tests.
type countingSigner struct {
	mu    sync.Mutex
	calls int
}

func (c *countingSigner) SignURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return "https://signed.example.com/" + strconv.Itoa(c.calls), nil
}

func (c *countingSigner) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func testSegments(n int, discontinuityAt int) []model.MtsMediaV2 {
	segs := make([]model.MtsMediaV2, n)
	for i := range segs {
		segs[i] = model.MtsMediaV2{
			MID:           16,
			Timestamp:     1790596912 + int64(i*3),
			Duration:      2500,
			StorageURI:    fmt.Sprintf(testSegmentURL, i),
			BroadcastID:   testBroadcastID,
			Discontinuity: discontinuityAt != -1 && i == discontinuityAt,
			Type:          "video/h264",
		}
	}
	return segs
}

func testToken(t *testing.T, sub string, exp time.Time) string {
	t.Helper()
	tok, err := gauth.PutClaims(map[string]interface{}{
		"sub": sub,
		"exp": exp.Unix(),
	}, testJWTSecret)
	require.NoError(t, err)
	return tok
}

func newTestServer() *server {
	s := newServer(nil, fakeSigner{}, testJWTSecret, time.Hour)
	s.log = func(string, ...any) {}
	return s
}

func TestServerSignURLCache(t *testing.T) {
	cs := &countingSigner{}
	s := newServer(nil, cs, testJWTSecret, time.Hour)

	now := time.Now()
	s.now = func() time.Time { return now }

	u1, err := s.signURL(context.Background(), "https://example.com/bucket/key.ts")
	require.NoError(t, err)
	u2, err := s.signURL(context.Background(), "https://example.com/bucket/key.ts")
	require.NoError(t, err)
	assert.Equal(t, u1, u2)
	assert.Equal(t, 1, cs.callCount())

	// Past half the time to live the URL should be re-signed.
	now = now.Add(31 * time.Minute)
	u3, err := s.signURL(context.Background(), "https://example.com/bucket/key.ts")
	require.NoError(t, err)
	assert.NotEqual(t, u1, u3)
	assert.Equal(t, 2, cs.callCount())
}

func TestRenderPlaylistVOD(t *testing.T) {
	s := newTestServer()
	ctx := context.Background()
	event := &model.BroadcastEvent{ID: testBroadcastID, EndTime: time.Now()}

	pl, err := s.renderPlaylist(ctx, event, testSegments(3, -1), 0, false, false)
	require.NoError(t, err)

	assert.Contains(t, pl, "#EXTM3U")
	assert.Contains(t, pl, "#EXT-X-VERSION:9")
	assert.Contains(t, pl, "#EXT-X-TARGETDURATION:3")
	assert.Contains(t, pl, "#EXT-X-MEDIA-SEQUENCE:0")
	assert.Contains(t, pl, "#EXT-X-PLAYLIST-TYPE:EVENT")
	assert.Contains(t, pl, "#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES")
	assert.Contains(t, pl, "#EXT-X-ENDLIST")
	assert.Equal(t, 3, strings.Count(pl, "#EXTINF:2.500,"))
	assert.Equal(t, 3, strings.Count(pl, "https://signed.example.com/"))
	assert.NotContains(t, pl, "#EXT-X-SKIP")
	assert.NotContains(t, pl, "#EXT-X-DISCONTINUITY")
}

func TestRenderPlaylistLive(t *testing.T) {
	s := newTestServer()
	event := &model.BroadcastEvent{ID: testBroadcastID}

	pl, err := s.renderPlaylist(context.Background(), event, testSegments(2, -1), 0, false, false)
	require.NoError(t, err)
	assert.NotContains(t, pl, "#EXT-X-ENDLIST")
	assert.Contains(t, pl, "#EXT-X-PLAYLIST-TYPE:EVENT")
}

func TestRenderPlaylistLiveEmpty(t *testing.T) {
	s := newTestServer()
	event := &model.BroadcastEvent{ID: testBroadcastID}

	pl, err := s.renderPlaylist(context.Background(), event, nil, 0, false, false)
	require.NoError(t, err)
	assert.Contains(t, pl, "#EXTM3U")
	assert.Contains(t, pl, "#EXT-X-TARGETDURATION:1")
	assert.NotContains(t, pl, "#EXTINF")
	assert.NotContains(t, pl, "#EXT-X-ENDLIST")
}

func TestRenderPlaylistDiscontinuity(t *testing.T) {
	s := newTestServer()
	event := &model.BroadcastEvent{ID: testBroadcastID, EndTime: time.Now()}

	pl, err := s.renderPlaylist(context.Background(), event, testSegments(3, 1), 0, false, false)
	require.NoError(t, err)

	assert.Equal(t, 1, strings.Count(pl, "#EXT-X-DISCONTINUITY"))
	disc := strings.Index(pl, "#EXT-X-DISCONTINUITY")
	seg0 := strings.Index(pl, "seg0.ts")
	seg1 := strings.Index(pl, "seg1.ts")
	assert.Greater(t, disc, seg0)
	assert.Less(t, disc, seg1)
}

func TestRenderPlaylistDelta(t *testing.T) {
	s := newTestServer()
	event := &model.BroadcastEvent{ID: testBroadcastID}

	pl, err := s.renderPlaylist(context.Background(), event, testSegments(3, -1), 2, true, true)
	require.NoError(t, err)

	assert.Contains(t, pl, "#EXT-X-MEDIA-SEQUENCE:0")
	assert.Contains(t, pl, "#EXT-X-SKIP:SKIPPED-SEGMENTS:2")
	assert.Equal(t, 1, strings.Count(pl, "#EXTINF:"))
	assert.Contains(t, pl, "seg2.ts")
	assert.NotContains(t, pl, "seg0.ts")
	assert.NotContains(t, pl, "seg1.ts")
}

func TestRenderPlaylistSignError(t *testing.T) {
	s := newTestServer()
	event := &model.BroadcastEvent{ID: testBroadcastID}
	segs := testSegments(1, -1)
	segs[0].StorageURI = "not a uri"

	_, err := s.renderPlaylist(context.Background(), event, segs, 0, false, false)
	assert.Error(t, err)
}

func newTestApp(s *server) *fiber.App {
	app := fiber.New()
	app.Get("/:file", s.handlePlaylist)
	return app
}

func doRequest(t *testing.T, app *fiber.App, url string) (int, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", url, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

func TestHandlePlaylistSuccess(t *testing.T) {
	s := newTestServer()
	s.loadEvent = func(context.Context, string) (*model.BroadcastEvent, error) {
		return &model.BroadcastEvent{ID: testBroadcastID, EndTime: time.Now()}, nil
	}
	s.loadSegments = func(context.Context, string) ([]model.MtsMediaV2, error) {
		return testSegments(2, -1), nil
	}
	app := newTestApp(s)

	token := testToken(t, testBroadcastID, time.Now().Add(time.Hour))
	status, body := doRequest(t, app, "/"+testBroadcastID+".m3u8?token="+token)

	assert.Equal(t, fiber.StatusOK, status)
	assert.Contains(t, body, "#EXTM3U")
	assert.Contains(t, body, "#EXT-X-ENDLIST")
}

func TestHandlePlaylistAuth(t *testing.T) {
	s := newTestServer()
	s.loadEvent = func(context.Context, string) (*model.BroadcastEvent, error) {
		return &model.BroadcastEvent{ID: testBroadcastID}, nil
	}
	s.loadSegments = func(context.Context, string) ([]model.MtsMediaV2, error) {
		return testSegments(1, -1), nil
	}
	app := newTestApp(s)

	t.Run("Missing", func(t *testing.T) {
		status, _ := doRequest(t, app, "/"+testBroadcastID+".m3u8")
		assert.Equal(t, fiber.StatusUnauthorized, status)
	})

	t.Run("Expired", func(t *testing.T) {
		token := testToken(t, testBroadcastID, time.Now().Add(-time.Hour))
		status, _ := doRequest(t, app, "/"+testBroadcastID+".m3u8?token="+token)
		assert.Equal(t, fiber.StatusUnauthorized, status)
	})

	t.Run("WrongBroadcast", func(t *testing.T) {
		token := testToken(t, "some-other-broadcast", time.Now().Add(time.Hour))
		status, _ := doRequest(t, app, "/"+testBroadcastID+".m3u8?token="+token)
		assert.Equal(t, fiber.StatusForbidden, status)
	})

	t.Run("BearerHeader", func(t *testing.T) {
		token := testToken(t, testBroadcastID, time.Now().Add(time.Hour))
		req := httptest.NewRequest("GET", "/"+testBroadcastID+".m3u8", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
}

func TestHandlePlaylistNotFound(t *testing.T) {
	s := newTestServer()
	s.loadEvent = func(context.Context, string) (*model.BroadcastEvent, error) {
		return nil, fmt.Errorf("failed to get BroadcastEvent: %w", datastore.ErrNoSuchEntity)
	}
	app := newTestApp(s)

	token := testToken(t, testBroadcastID, time.Now().Add(time.Hour))
	status, _ := doRequest(t, app, "/"+testBroadcastID+".m3u8?token="+token)
	assert.Equal(t, fiber.StatusNotFound, status)
}

func TestHandlePlaylistBadRequest(t *testing.T) {
	s := newTestServer()
	s.loadEvent = func(context.Context, string) (*model.BroadcastEvent, error) {
		return &model.BroadcastEvent{ID: testBroadcastID}, nil
	}
	s.loadSegments = func(context.Context, string) ([]model.MtsMediaV2, error) {
		return testSegments(2, -1), nil
	}
	app := newTestApp(s)
	token := testToken(t, testBroadcastID, time.Now().Add(time.Hour))

	t.Run("InvalidMSN", func(t *testing.T) {
		status, _ := doRequest(t, app, "/"+testBroadcastID+".m3u8?token="+token+"&_HLS_msn=abc")
		assert.Equal(t, fiber.StatusBadRequest, status)
	})

	t.Run("PartUnsupported", func(t *testing.T) {
		status, _ := doRequest(t, app, "/"+testBroadcastID+".m3u8?token="+token+"&_HLS_part=0")
		assert.Equal(t, fiber.StatusBadRequest, status)
	})

	t.Run("TooFarAhead", func(t *testing.T) {
		status, _ := doRequest(t, app, "/"+testBroadcastID+".m3u8?token="+token+"&_HLS_msn=5")
		assert.Equal(t, fiber.StatusBadRequest, status)
	})

	t.Run("TooFarAheadEmpty", func(t *testing.T) {
		s.loadSegments = func(context.Context, string) ([]model.MtsMediaV2, error) { return nil, nil }
		status, _ := doRequest(t, app, "/"+testBroadcastID+".m3u8?token="+token+"&_HLS_msn=1")
		assert.Equal(t, fiber.StatusBadRequest, status)
	})

	t.Run("EndedNoBlocking", func(t *testing.T) {
		// An ended broadcast will never receive new segments, so requesting
		// the next media sequence number must not block; it is an error.
		s.loadSegments = func(context.Context, string) ([]model.MtsMediaV2, error) {
			return testSegments(2, -1), nil
		}
		s.loadEvent = func(context.Context, string) (*model.BroadcastEvent, error) {
			return &model.BroadcastEvent{ID: testBroadcastID, EndTime: time.Now()}, nil
		}
		s.blockTimeout = time.Hour // Would hang if blocking were attempted.
		status, _ := doRequest(t, app, "/"+testBroadcastID+".m3u8?token="+token+"&_HLS_msn=2")
		assert.Equal(t, fiber.StatusBadRequest, status)
	})
}

func TestHandlePlaylistBlockingReload(t *testing.T) {
	s := newTestServer()
	s.blockPoll = time.Millisecond
	s.blockTimeout = 2 * time.Second
	s.loadEvent = func(context.Context, string) (*model.BroadcastEvent, error) {
		return &model.BroadcastEvent{ID: testBroadcastID}, nil
	}
	var calls int
	s.loadSegments = func(context.Context, string) ([]model.MtsMediaV2, error) {
		calls++
		n := 1
		if calls > 1 {
			n = 2
		}
		return testSegments(n, -1), nil
	}
	app := newTestApp(s)

	token := testToken(t, testBroadcastID, time.Now().Add(time.Hour))
	status, body := doRequest(t, app, "/"+testBroadcastID+".m3u8?token="+token+"&_HLS_msn=1")

	assert.Equal(t, fiber.StatusOK, status)
	assert.GreaterOrEqual(t, calls, 2)
	assert.Equal(t, 2, strings.Count(body, "#EXTINF:"))
	assert.Contains(t, body, "seg1.ts")
}

func TestHandlePlaylistNotM3U8(t *testing.T) {
	s := newTestServer()
	app := newTestApp(s)
	status, _ := doRequest(t, app, "/"+testBroadcastID)
	assert.Equal(t, fiber.StatusNotFound, status)
}
