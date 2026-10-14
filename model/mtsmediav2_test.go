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
	"testing"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/stretchr/testify/assert"
)

const (
	testBcv2BroadcastA = "broadcast-v2-a"
	testBcv2BroadcastB = "broadcast-v2-b"
	testBcv2MID        = int64(16)
)

// newMtsMediaV2 creates a new MtsMediaV2 entity, registering cleanup with t.
// Its Date (used for ordering) is set to now by CreateMtsMediaV2.
func newMtsMediaV2(t *testing.T, ctx context.Context, store datastore.Store, broadcastID string, mid, ts, pts int64) {
	t.Helper()
	_, err := CreateMtsMediaV2(ctx, store, MtsMediaV2{
		MID:         mid,
		Timestamp:   ts,
		PTS:         pts,
		Duration:    2541,
		StorageURI:  "https://example.com/bucket/" + broadcastID + "/segment.ts",
		BroadcastID: broadcastID,
		Type:        "video/h264",
	})
	assert.NoError(t, err)
	t.Cleanup(func() {
		_ = DeleteMtsMediaV2(ctx, store, mid, ts)
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
