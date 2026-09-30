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

	"github.com/ausocean/cloud/datastore"
	"github.com/stretchr/testify/assert"
)

const (
	testBcv2BroadcastA = "broadcast-v2-a"
	testBcv2BroadcastB = "broadcast-v2-b"
	testBcv2MID        = int64(16)
)

// newMtsMediaV2 creates a new MtsMediaV2 entity with the given broadcast
// ID and timestamp, registering cleanup with t.
func newMtsMediaV2(t *testing.T, ctx context.Context, store datastore.Store, broadcastID string, mid, ts int64) {
	t.Helper()
	_, err := CreateMtsMediaV2(ctx, store, MtsMediaV2{
		MID:         mid,
		Timestamp:   ts,
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

			for _, ts := range []int64{1000, 1005, 1010, 1015} {
				newMtsMediaV2(t, ctx, store, testBcv2BroadcastA, testBcv2MID, ts)
			}
			newMtsMediaV2(t, ctx, store, testBcv2BroadcastB, testBcv2MID, 2000)

			timestamps := func(media []MtsMediaV2) []int64 {
				out := make([]int64, len(media))
				for i, m := range media {
					out[i] = m.Timestamp
				}
				return out
			}

			t.Run("AllOrdered", func(t *testing.T) {
				got, err := GetMtsMediaV2ByBroadcast(ctx, store, testBcv2BroadcastA, nil, 0)
				assert.NoError(t, err)
				assert.Equal(t, []int64{1000, 1005, 1010, 1015}, timestamps(got))
			})

			t.Run("BroadcastFilter", func(t *testing.T) {
				got, err := GetMtsMediaV2ByBroadcast(ctx, store, testBcv2BroadcastB, nil, 0)
				assert.NoError(t, err)
				assert.Equal(t, []int64{2000}, timestamps(got))
			})

			t.Run("FromTimestamp", func(t *testing.T) {
				got, err := GetMtsMediaV2ByBroadcast(ctx, store, testBcv2BroadcastA, []int64{1005}, 0)
				assert.NoError(t, err)
				assert.Equal(t, []int64{1005, 1010, 1015}, timestamps(got))
			})

			t.Run("TimestampRange", func(t *testing.T) {
				got, err := GetMtsMediaV2ByBroadcast(ctx, store, testBcv2BroadcastA, []int64{1005, 1015}, 0)
				assert.NoError(t, err)
				assert.Equal(t, []int64{1005, 1010}, timestamps(got))
			})

			t.Run("Limit", func(t *testing.T) {
				got, err := GetMtsMediaV2ByBroadcast(ctx, store, testBcv2BroadcastA, nil, 2)
				assert.NoError(t, err)
				assert.Equal(t, []int64{1000, 1005}, timestamps(got))
			})

			t.Run("NoResults", func(t *testing.T) {
				got, err := GetMtsMediaV2ByBroadcast(ctx, store, "no-such-broadcast", nil, 0)
				assert.NoError(t, err)
				assert.Empty(t, got)
			})

			t.Run("EmptyBroadcastID", func(t *testing.T) {
				_, err := GetMtsMediaV2ByBroadcast(ctx, store, "", nil, 0)
				assert.Error(t, err)
			})
		})
	}
}
