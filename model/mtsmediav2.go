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
	MID           int64     // Media ID.
	Geohash       string    // Geohash, if any.
	Timestamp     int64     // Timestamp (in seconds).
	Duration      int64     // Duration of the clip (in milliseconds).
	StorageURI    string    // URI of the clip in the storage bucket.
	BroadcastID   string    // OceanMedia broadcast ID.
	Discontinuity bool      // True if this clip has a discontinuity from the previous one.
	PTS           int64     // The PTS at the start of the clip.
	Type          string    // MIME type of the clip.
	Date          time.Time // Date/time this record was created.
	datastore.NoCache
}

// Implements Copy from the Entity interface.
func (mts *MtsMediaV2) Copy(dst datastore.Entity) (datastore.Entity, error) {
	return datastore.CopyEntity(mts, dst)
}

func MtsMediaV2Key(store datastore.Store, mid int64, timestamp int64) *datastore.Key {
	return store.NameKey(typeMtsMediaV2, fmt.Sprintf("%d.%d", mid, timestamp))
}

func CreateMtsMediaV2(ctx context.Context, store datastore.Store, mts MtsMediaV2) (*MtsMediaV2, error) {
	key := MtsMediaV2Key(store, mts.MID, mts.Timestamp)
	mts.Date = time.Now()
	err := store.Create(ctx, key, &mts)
	if err != nil {
		return nil, fmt.Errorf("failed to create MtsMediaV2: %w", err)
	}
	return &mts, nil
}

func GetMtsMediaV2(ctx context.Context, store datastore.Store, mid int64, timestamp int64) (*MtsMediaV2, error) {
	key := MtsMediaV2Key(store, mid, timestamp)
	mts := &MtsMediaV2{}
	err := store.Get(ctx, key, mts)
	if err != nil {
		return nil, fmt.Errorf("failed to get MtsMediaV2: %w", err)
	}
	return mts, nil
}

func DeleteMtsMediaV2(ctx context.Context, store datastore.Store, mid int64, timestamp int64) error {
	key := MtsMediaV2Key(store, mid, timestamp)
	err := store.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete MtsMediaV2: %w", err)
	}
	return nil
}
