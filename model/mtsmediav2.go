/*
DESCRIPTION
  MtsMediaV2 datastore type and functions.

AUTHORS
  Elliot Shine <elliot@ausocean.org>

LICENSE
  Copyright (C) 2019-2026 the Australian Ocean Lab (AusOcean).

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
	"sort"
	"time"

	"github.com/ausocean/cloud/datastore"
)

const (
	typeMtsMediaV2 = "MtsMediaV2" // MtsMediaV2 datastore type.
)

// maxMtsMediaV2 is the default maximum number of MtsMediaV2 entities
// returned for a broadcast when no limit is supplied. It is sized to
// accommodate a full day of OceanMedia segments (an 8 hour broadcast at
// roughly 2.5 second segments is around 12,000 segments).
const maxMtsMediaV2 = 20000

type MtsMediaV2 struct {
	MID           int64     `json:"mid"`           // Media ID.
	Geohash       string    `json:"geohash"`       // Geohash, if any.
	Timestamp     int64     `json:"timestamp"`     // Timestamp (in seconds).
	Duration      int64     `json:"duration"`      // Duration of the clip (in milliseconds).
	StorageURI    string    `json:"storageURI"`    // URI of the clip in the storage bucket.
	BroadcastID   string    `json:"broadcastID"`   // OceanMedia broadcast ID.
	Discontinuity bool      `json:"discontinuity"` // True if this clip has a discontinuity from the previous one.
	PTS           int64     `json:"pts"`           // The PTS at the start of the clip.
	Type          string    `json:"type"`          // MIME type of the clip.
	Date          time.Time `json:"date"`          // Date/time this record was created.
	datastore.NoCache
}

// Implements Copy from the Entity interface.
func (mts *MtsMediaV2) Copy(dst datastore.Entity) (datastore.Entity, error) {
	return datastore.CopyEntity(mts, dst)
}

// mtsMediaV2Key returns a datastore key for a MtsMediaV2 entity.
func mtsMediaV2Key(store datastore.Store, mid int64, timestamp int64) *datastore.Key {
	return store.NameKey(typeMtsMediaV2, fmt.Sprintf("%d.%d", mid, timestamp))
}

// CreateMtsMediaV2 creates a new MtsMediaV2 entity.
func CreateMtsMediaV2(ctx context.Context, store datastore.Store, mts MtsMediaV2) (*MtsMediaV2, error) {
	key := mtsMediaV2Key(store, mts.MID, mts.Timestamp)
	mts.Date = time.Now()
	err := store.Create(ctx, key, &mts)
	if err != nil {
		return nil, fmt.Errorf("failed to create MtsMediaV2: %w", err)
	}
	return &mts, nil
}

// GetMtsMediaV2 retrieves a MtsMediaV2 entity from the datastore.
func GetMtsMediaV2(ctx context.Context, store datastore.Store, mid int64, timestamp int64) (*MtsMediaV2, error) {
	key := mtsMediaV2Key(store, mid, timestamp)
	mts := &MtsMediaV2{}
	err := store.Get(ctx, key, mts)
	if err != nil {
		return nil, fmt.Errorf("failed to get MtsMediaV2: %w", err)
	}
	return mts, nil
}

// DeleteMtsMediaV2 deletes a MtsMediaV2 entity from the datastore.
func DeleteMtsMediaV2(ctx context.Context, store datastore.Store, mid int64, timestamp int64) error {
	key := mtsMediaV2Key(store, mid, timestamp)
	err := store.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete MtsMediaV2: %w", err)
	}
	return nil
}

// GetMtsMediaV2ByBroadcast retrieves the MtsMediaV2 entities belonging
// to a broadcast, optionally filtered by timestamp(s). One timestamp
// represents a lower bound (inclusive) on the segment timestamp,
// whereas two represents a time range. Results are ordered by segment
// timestamp ascending.
//
// A limit of zero or less uses a default size to cover the whole
// broadcast.
func GetMtsMediaV2ByBroadcast(ctx context.Context, store datastore.Store, broadcastID string, ts []int64, limit int) ([]MtsMediaV2, error) {
	if broadcastID == "" {
		return nil, errors.New("empty broadcast ID")
	}
	if limit <= 0 {
		limit = maxMtsMediaV2
	}

	// NB: no key parts are declared because BroadcastID is an entity
	// field, not part of the MtsMediaV2 key (which is "<mid>.<timestamp>").
	q := store.NewQuery(typeMtsMediaV2, false)
	err := q.FilterField("BroadcastID", "=", broadcastID)
	if err != nil {
		return nil, fmt.Errorf("failed to filter MtsMediaV2 by broadcast ID: %w", err)
	}
	if len(ts) > 0 {
		err = q.FilterField("Timestamp", ">=", ts[0])
		if err != nil {
			return nil, fmt.Errorf("failed to filter MtsMediaV2 by timestamp: %w", err)
		}
	}
	if len(ts) > 1 {
		err = q.FilterField("Timestamp", "<", ts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to filter MtsMediaV2 by timestamp: %w", err)
		}
	}
	q.Order("Timestamp")
	q.Limit(limit)

	var media []MtsMediaV2
	_, err = store.GetAll(ctx, q, &media)
	if err != nil {
		return nil, fmt.Errorf("failed to get MtsMediaV2 for broadcast %s: %w", broadcastID, err)
	}

	// FileStore orders by key name, which is not guaranteed to match a
	// segment's timestamp, so sort explicitly.
	sort.Slice(media, func(i, j int) bool {
		return media[i].Timestamp < media[j].Timestamp
	})
	return media, nil
}
