/*
DESCRIPTION
  Tests for model/mtsmediav2.go.

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

package model

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/stretchr/testify/assert"
)

const (
	testBcv2BroadcastA = "broadcast-v2-a"
	testBcv2BroadcastB = "broadcast-v2-b"
	testBcv2MID        = int64(16)

	testIdemMID         = int64(101)
	testIdemTimestamp   = int64(1700000000)
	testIdemClipID      = "clip-idempotent"
	testIdemStorageURI  = "https://bucket.example/clip-idempotent.ts"
	testIdemBroadcastID = "test-mtsmediav2-idempotent"

	testGetMID         = int64(102)
	testGetTimestamp   = int64(1700000001)
	testGetClipID      = "clip-get"
	testGetBroadcastID = "test-mtsmediav2-get"
)

// newMtsMediaV2 creates a new MtsMediaV2 entity, registering cleanup with t.
// Its Date (used for ordering) is set to now by CreateMtsMediaV2.
func newMtsMediaV2(t *testing.T, ctx context.Context, store datastore.Store, broadcastID string, mid, ts, pts int64) {
	t.Helper()
	clipID := fmt.Sprintf("clip-%d-%d", ts, pts)
	_, err := CreateMtsMediaV2(ctx, store, MtsMediaV2{
		MID:         mid,
		ClipID:      clipID,
		Timestamp:   ts,
		PTS:         pts,
		Duration:    2541,
		StorageURI:  fmt.Sprintf("https://example.com/bucket/%s/segment-%d.ts", broadcastID, ts),
		BroadcastID: broadcastID,
		Type:        "video/h264",
	})
	assert.NoError(t, err)
	t.Cleanup(func() {
		_ = DeleteMtsMediaV2(ctx, store, mid, clipID)
	})
}

func TestGetMtsMediaV2ByBroadcast(t *testing.T) {
	for _, kind := range storeKinds {
		t.Run(kind, func(t *testing.T) {
			ctx, store := setupStore(t, kind)

			// Creation order deliberately disagrees with timestamp and PTS
			// order, so that ordering by Date (creation order) is exercised.
			newMtsMediaV2(t, ctx, store, testBcv2BroadcastA, testBcv2MID, 1010, 400)
			time.Sleep(time.Millisecond)
			newMtsMediaV2(t, ctx, store, testBcv2BroadcastA, testBcv2MID, 1000, 300)
			time.Sleep(time.Millisecond)
			newMtsMediaV2(t, ctx, store, testBcv2BroadcastA, testBcv2MID, 1015, 200)
			time.Sleep(time.Millisecond)
			newMtsMediaV2(t, ctx, store, testBcv2BroadcastA, testBcv2MID, 1005, 100)
			newMtsMediaV2(t, ctx, store, testBcv2BroadcastB, testBcv2MID, 2000, 10)

			timestamps := func(media []MtsMediaV2) []int64 {
				out := make([]int64, len(media))
				for i, m := range media {
					out[i] = m.Timestamp
				}
				return out
			}

			t.Run("OrderedByDate", func(t *testing.T) {
				got, err := GetMtsMediaV2ByBroadcast(ctx, store, testBcv2BroadcastA, 0)
				assert.NoError(t, err)
				assert.Equal(t, []int64{1010, 1000, 1015, 1005}, timestamps(got))
			})

			t.Run("BroadcastFilter", func(t *testing.T) {
				got, err := GetMtsMediaV2ByBroadcast(ctx, store, testBcv2BroadcastB, 0)
				assert.NoError(t, err)
				assert.Equal(t, []int64{2000}, timestamps(got))
			})

			t.Run("Limit", func(t *testing.T) {
				// The limit is a safety cap. FileStore applies it before
				// sorting in the model layer, so only assert the cap here.
				got, err := GetMtsMediaV2ByBroadcast(ctx, store, testBcv2BroadcastA, 2)
				assert.NoError(t, err)
				assert.Len(t, got, 2)
			})

			t.Run("NoResults", func(t *testing.T) {
				got, err := GetMtsMediaV2ByBroadcast(ctx, store, "no-such-broadcast", 0)
				assert.NoError(t, err)
				assert.Empty(t, got)
			})

			t.Run("EmptyBroadcastID", func(t *testing.T) {
				_, err := GetMtsMediaV2ByBroadcast(ctx, store, "", 0)
				assert.Error(t, err)
			})
		})
	}
}

// TestCreateMtsMediaV2Idempotent checks that creating an MtsMediaV2 entity for
// a clip that has already been stored (same MID and ClipID) returns the
// existing entity rather than creating a duplicate.
func TestCreateMtsMediaV2Idempotent(t *testing.T) {
	for _, kind := range storeKinds {
		t.Run(kind, func(t *testing.T) {
			ctx, store := setupStore(t, kind)

			mid := testIdemMID
			base := testIdemTimestamp
			clipID := testIdemClipID
			uri := testIdemStorageURI
			broadcastID := testIdemBroadcastID
			t.Cleanup(func() {
				_ = DeleteMtsMediaV2(ctx, store, mid, clipID)
			})

			first, err := CreateMtsMediaV2(ctx, store, MtsMediaV2{
				MID:         mid,
				ClipID:      clipID,
				Timestamp:   base,
				StorageURI:  uri,
				BroadcastID: broadcastID,
				Duration:    2000,
			})
			assert.NoError(t, err)
			assert.NotNil(t, first)

			// A retry of the same clip, with a different receipt timestamp,
			// must not create a duplicate.
			_, err = CreateMtsMediaV2(ctx, store, MtsMediaV2{
				MID:         mid,
				ClipID:      clipID,
				Timestamp:   base + 1,
				StorageURI:  uri,
				BroadcastID: broadcastID,
				Duration:    2000,
			})
			assert.Error(t, err)

			media, err := GetMtsMediaV2ByBroadcast(ctx, store, broadcastID, 0)
			assert.NoError(t, err)
			assert.Len(t, media, 1, "expected exactly one entity for the clip")
		})
	}
}

// TestGetMtsMediaV2 checks that an entity can be retrieved by its clip ID and
// that a missing clip returns ErrNoSuchEntity.
func TestGetMtsMediaV2(t *testing.T) {
	for _, kind := range storeKinds {
		t.Run(kind, func(t *testing.T) {
			ctx, store := setupStore(t, kind)

			mid := testGetMID
			base := testGetTimestamp
			clipID := testGetClipID
			t.Cleanup(func() {
				_ = DeleteMtsMediaV2(ctx, store, mid, clipID)
			})

			want := MtsMediaV2{
				MID:         mid,
				ClipID:      clipID,
				Timestamp:   base,
				StorageURI:  "https://bucket.example/clip-get.ts",
				BroadcastID: testGetBroadcastID,
				Duration:    2000,
			}
			_, err := CreateMtsMediaV2(ctx, store, want)
			assert.NoError(t, err)

			got, err := GetMtsMediaV2(ctx, store, mid, clipID)
			assert.NoError(t, err)
			assert.Equal(t, want.ClipID, got.ClipID)
			assert.Equal(t, want.MID, got.MID)
			assert.Equal(t, want.StorageURI, got.StorageURI)

			_, err = GetMtsMediaV2(ctx, store, mid, "no-such-clip")
			assert.True(t, errors.Is(err, datastore.ErrNoSuchEntity), "expected ErrNoSuchEntity, got %v", err)
		})
	}
}
