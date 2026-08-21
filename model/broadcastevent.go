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
	typeBroadcastEvent = "BroadcastEvent" // BroadcastEvent datastore type.
)

// A BroadcastEvent is a distinct livestream event, such as an entire day of
// livestreams at a specific location.
type BroadcastEvent struct {
	ID                 string    // BroadcastEvent ID, a UUID.
	ConfigID           string    // The ID of the broadcast configuration that this event is an instance of.
	ScheduledStartTime time.Time // Scheduled broadcast start time (server local time).
	ScheduledEndTime   time.Time // Scheduled broadcast end time (server local time).
	StartTime          time.Time // Actual broadcast start time (server local time).
	EndTime            time.Time // Actual broadcast end time (server local time).
	Type               string    // Broadcast type (eg audio or video)
	datastore.NoCache
}

// Implements Copy from the Entity interface.
func (b *BroadcastEvent) Copy(dst datastore.Entity) (datastore.Entity, error) {
	return datastore.CopyEntity(b, dst)
}

// broadcastEventKey returns a datastore key for a BroadcastEvent entity.
func broadcastEventKey(store datastore.Store, id string) *datastore.Key {
	return store.NameKey(typeBroadcastEvent, id)
}

// CreateBroadcastEvent creates a new BroadcastEvent entity.
func CreateBroadcastEvent(ctx context.Context, store datastore.Store, b BroadcastEvent) (*BroadcastEvent, error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	key := broadcastEventKey(store, b.ID)
	err := store.Create(ctx, key, &b)
	if err != nil {
		return nil, fmt.Errorf("failed to create BroadcastEvent: %w", err)
	}
	return &b, nil
}

// GetBroadcastEvent retrieves a BroadcastEvent entity from the datastore.
func GetBroadcastEvent(ctx context.Context, store datastore.Store, id string) (*BroadcastEvent, error) {
	key := broadcastEventKey(store, id)
	b := &BroadcastEvent{}
	err := store.Get(ctx, key, b)
	if err != nil {
		return nil, fmt.Errorf("failed to get BroadcastEvent: %w", err)
	}
	return b, nil
}

// UpdateBroadcastEvent updates a BroadcastEvent entity.
func UpdateBroadcastEvent(ctx context.Context, store datastore.Store, b *BroadcastEvent) (*BroadcastEvent, error) {
	key := broadcastEventKey(store, b.ID)
	out := &BroadcastEvent{}
	err := store.Update(ctx, key, func(e datastore.Entity) {
		_b := e.(*BroadcastEvent)
		_b.ConfigID = b.ConfigID
		_b.ScheduledStartTime = b.ScheduledStartTime
		_b.ScheduledEndTime = b.ScheduledEndTime
		_b.Type = b.Type
	}, out)
	if err != nil {
		return nil, fmt.Errorf("failed to update BroadcastEvent: %w", err)
	}
	return out, nil
}

// DeleteBroadcastEvent deletes a BroadcastEvent entity from the datastore.
func DeleteBroadcastEvent(ctx context.Context, store datastore.Store, id string) error {
	if id == "" {
		return datastore.ErrNoSuchEntity
	}
	key := broadcastEventKey(store, id)
	err := store.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete BroadcastEvent: %w", err)
	}
	return nil
}

// SetBroadcastEventStart sets the broadcast event start time to now in the system timezone.
func SetBroadcastEventStart(ctx context.Context, store datastore.Store, id string) error {
	key := broadcastEventKey(store, id)
	out := &BroadcastEvent{}
	err := store.Update(ctx, key, func(e datastore.Entity) {
		b := e.(*BroadcastEvent)
		b.StartTime = time.Now()
	}, out)
	if err != nil {
		return fmt.Errorf("failed to update BroadcastEvent: %w", err)
	}
	return nil
}

// SetBroadcastEventEnd sets the broadcast event end time to now in the system timezone.
func SetBroadcastEventEnd(ctx context.Context, store datastore.Store, id string) error {
	key := broadcastEventKey(store, id)
	out := &BroadcastEvent{}
	err := store.Update(ctx, key, func(e datastore.Entity) {
		b := e.(*BroadcastEvent)
		b.EndTime = time.Now()
	}, out)
	if err != nil {
		return fmt.Errorf("failed to update BroadcastEvent: %w", err)
	}
	return nil
}
