/*
DESCRIPTION
  Broadcast datastore type and functions.

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
	"github.com/google/uuid"
)

const (
	typeBroadcast = "Broadcast" // Broadcast datastore type.
)

// A Broadcast is a distinct livestream event, such as an entire day of
// livestreams at a specific location.
type Broadcast struct {
	ID                 string    // Broadcast ID.
	StartTime          time.Time // Actual broadcast start time.
	ScheduledStartTime time.Time // Scheduled broadcast start time.
	ScheduledEndTime   time.Time // Scheduled broadcast end time.
	EndTime            time.Time // Actual broadcast end time.
	Name               string    // Broadcast name.
	Description        string    // Broadcast description.
	Type               string    // Broadcast type (eg audio or video)
	datastore.NoCache
}

// Implements Copy from the Entity interface.
func (b *Broadcast) Copy(dst datastore.Entity) (datastore.Entity, error) {
	return datastore.CopyEntity(b, dst)
}

// broadcastKey returns a datastore key for a Broadcast entity.
func broadcastKey(store datastore.Store, id string) *datastore.Key {
	return store.NameKey(typeBroadcast, id)
}

// CreateBroadcast creates a new Broadcast entity.
func CreateBroadcast(ctx context.Context, store datastore.Store, b Broadcast) (*Broadcast, error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	key := broadcastKey(store, b.ID)
	err := store.Create(ctx, key, &b)
	if err != nil {
		return nil, fmt.Errorf("failed to create Broadcast: %w", err)
	}
	return &b, nil
}

// GetBroadcast retrieves a Broadcast entity from the datastore.
func GetBroadcast(ctx context.Context, store datastore.Store, id string) (*Broadcast, error) {
	key := broadcastKey(store, id)
	b := &Broadcast{}
	err := store.Get(ctx, key, b)
	if err != nil {
		return nil, fmt.Errorf("failed to get Broadcast: %w", err)
	}
	return b, nil
}

// UpdateBroadcast updates a Broadcast entity.
func UpdateBroadcast(ctx context.Context, store datastore.Store, b *Broadcast) (*Broadcast, error) {
	key := broadcastKey(store, b.ID)
	out := &Broadcast{}
	err := store.Update(ctx, key, func(e datastore.Entity) {
		_b := e.(*Broadcast)
		_b.StartTime = b.StartTime
		_b.ScheduledStartTime = b.ScheduledStartTime
		_b.EndTime = b.EndTime
		_b.ScheduledEndTime = b.ScheduledEndTime
		_b.Name = b.Name
		_b.Description = b.Description
		_b.Type = b.Type
	}, out)
	if err != nil {
		return nil, fmt.Errorf("failed to update Broadcast: %w", err)
	}
	return out, nil
}

// DeleteBroadcast deletes a Broadcast entity from the datastore.
func DeleteBroadcast(ctx context.Context, store datastore.Store, id string) error {
	key := broadcastKey(store, id)
	err := store.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete Broadcast: %w", err)
	}
	return nil
}

// StartBroadcast sets the broadcast start time to now.
func StartBroadcast(ctx context.Context, store datastore.Store, id string) error {
	key := broadcastKey(store, id)
	out := &Broadcast{}
	err := store.Update(ctx, key, func(e datastore.Entity) {
		b := e.(*Broadcast)
		b.StartTime = time.Now()
	}, out)
	if err != nil {
		return fmt.Errorf("failed to update Broadcast: %w", err)
	}
	return nil
}

// EndBroadcast sets the broadcast end time to now.
func EndBroadcast(ctx context.Context, store datastore.Store, id string) error {
	key := broadcastKey(store, id)
	out := &Broadcast{}
	err := store.Update(ctx, key, func(e datastore.Entity) {
		b := e.(*Broadcast)
		b.EndTime = time.Now()
	}, out)
	if err != nil {
		return fmt.Errorf("failed to update Broadcast: %w", err)
	}
	return nil
}
