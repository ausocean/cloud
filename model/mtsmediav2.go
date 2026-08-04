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
	"fmt"
	"time"

	"github.com/ausocean/cloud/datastore"
)

const (
	typeMtsMediaV2 = "MtsMediaV2" // MtsMediaV2 datastore type.
)

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
