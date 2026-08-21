/*
DESCRIPTION
  Tests for model/broadcastevent.go.

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
	"os"
	"testing"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/stretchr/testify/assert"
)

const (
	testBroadcastEventID      = "broadcast-event-1"
	testBroadcastEventStartID = "broadcast-event-start-1"
	testBroadcastEventEndID   = "broadcast-event-end-1"
	testConfigID              = "config-1"
	testBroadcastEventType    = "video"
	testNonExistentID         = "non-existent-id"
	testUpdatedConfigID       = "config-updated"
	testUpdatedType           = "audio"
)

var storeKinds = []string{"file", "cloud"}

func setupStore(t *testing.T, kind string) (context.Context, datastore.Store) {
	ctx := context.Background()
	var (
		store datastore.Store
		err   error
	)
	switch kind {
	case "file":
		store, err = datastore.NewStore(ctx, "file", "test", t.TempDir())
	case "cloud":
		if os.Getenv("AUSOCEAN_CREDENTIALS") == "" {
			t.Skip("AUSOCEAN_CREDENTIALS required to access AusOcean datastore")
		}
		store, err = datastore.NewStore(ctx, "cloud", "ausocean/test", "")
	default:
		t.Fatalf("unknown store kind: %s", kind)
	}
	if err != nil {
		t.Fatalf("failed to create %s store: %v", kind, err)
	}
	RegisterEntities()
	return ctx, store
}

// isAcceptableDeleteError checks if the error is within the acceptable union
// for ghost / zero-input deletes per ADR-0002 (nil, ErrNoSuchEntity, or *os.PathError).
func isAcceptableDeleteError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		return true
	}
	if os.IsNotExist(err) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return true
	}
	return false
}

func assertAcceptableDeleteError(t *testing.T, err error) {
	t.Helper()
	if !isAcceptableDeleteError(err) {
		assert.Fail(t, "unexpected error from DeleteBroadcastEvent", "got: %v", err)
	}
}

func TestCreateBroadcastEvent(t *testing.T) {
	for _, kind := range storeKinds {
		t.Run(kind, func(t *testing.T) {
			ctx, store := setupStore(t, kind)
			t.Cleanup(func() {
				_ = DeleteBroadcastEvent(ctx, store, testBroadcastEventID)
			})

			now := time.Now().UTC().Truncate(time.Second)
			testBc := BroadcastEvent{
				ID:                 testBroadcastEventID,
				ConfigID:           testConfigID,
				ScheduledStartTime: now.Add(30 * time.Minute),
				ScheduledEndTime:   now.Add(2 * time.Hour),
				StartTime:          now,
				EndTime:            now.Add(2 * time.Hour),
				Type:               testBroadcastEventType,
			}

			// Create with explicit ID
			created, err := CreateBroadcastEvent(ctx, store, testBc)
			assert.NoError(t, err)
			assert.Equal(t, &testBc, created)

			// Duplicate create should fail
			_, err = CreateBroadcastEvent(ctx, store, testBc)
			assert.Error(t, err)

			// Create without ID (auto-generates UUID)
			autoBc := BroadcastEvent{
				ConfigID: testConfigID,
				Type:     testBroadcastEventType,
			}
			createdAuto, err := CreateBroadcastEvent(ctx, store, autoBc)
			assert.NoError(t, err)
			assert.NotEmpty(t, createdAuto.ID)
			t.Cleanup(func() {
				_ = DeleteBroadcastEvent(ctx, store, createdAuto.ID)
			})
		})
	}
}

func TestGetBroadcastEvent(t *testing.T) {
	for _, kind := range storeKinds {
		t.Run(kind, func(t *testing.T) {
			ctx, store := setupStore(t, kind)
			t.Cleanup(func() {
				_ = DeleteBroadcastEvent(ctx, store, testBroadcastEventID)
			})

			now := time.Now().UTC().Truncate(time.Second)
			testBc := BroadcastEvent{
				ID:                 testBroadcastEventID,
				ConfigID:           testConfigID,
				ScheduledStartTime: now.Add(30 * time.Minute),
				ScheduledEndTime:   now.Add(2 * time.Hour),
				StartTime:          now,
				EndTime:            now.Add(2 * time.Hour),
				Type:               testBroadcastEventType,
			}

			_, err := CreateBroadcastEvent(ctx, store, testBc)
			assert.NoError(t, err)

			got, err := GetBroadcastEvent(ctx, store, testBc.ID)
			assert.NoError(t, err)
			assert.Equal(t, &testBc, got)

			// Get non-existent should fail
			gotNonExistent, err := GetBroadcastEvent(ctx, store, testNonExistentID)
			assert.Error(t, err)
			assert.Nil(t, gotNonExistent)
		})
	}
}

func TestUpdateBroadcastEvent(t *testing.T) {
	for _, kind := range storeKinds {
		t.Run(kind, func(t *testing.T) {
			ctx, store := setupStore(t, kind)
			t.Cleanup(func() {
				_ = DeleteBroadcastEvent(ctx, store, testBroadcastEventID)
			})

			now := time.Now().UTC().Truncate(time.Second)
			testBc := BroadcastEvent{
				ID:                 testBroadcastEventID,
				ConfigID:           testConfigID,
				ScheduledStartTime: now.Add(30 * time.Minute),
				ScheduledEndTime:   now.Add(2 * time.Hour),
				StartTime:          now,
				EndTime:            now.Add(2 * time.Hour),
				Type:               testBroadcastEventType,
			}

			_, err := CreateBroadcastEvent(ctx, store, testBc)
			assert.NoError(t, err)

			updatedFields := &BroadcastEvent{
				ID:                 testBc.ID,
				ConfigID:           testUpdatedConfigID,
				ScheduledStartTime: now.Add(40 * time.Minute),
				ScheduledEndTime:   now.Add(3 * time.Hour),
				Type:               testUpdatedType,
			}

			updated, err := UpdateBroadcastEvent(ctx, store, updatedFields)
			assert.NoError(t, err)
			assert.Equal(t, updatedFields.ConfigID, updated.ConfigID)
			assert.Equal(t, updatedFields.ScheduledStartTime, updated.ScheduledStartTime)
			assert.Equal(t, updatedFields.ScheduledEndTime, updated.ScheduledEndTime)
			assert.Equal(t, updatedFields.Type, updated.Type)

			// Verify in store
			got, err := GetBroadcastEvent(ctx, store, testBc.ID)
			assert.NoError(t, err)
			assert.Equal(t, updatedFields.ConfigID, got.ConfigID)
			assert.Equal(t, updatedFields.ScheduledStartTime, got.ScheduledStartTime)
			assert.Equal(t, updatedFields.ScheduledEndTime, got.ScheduledEndTime)
			assert.Equal(t, updatedFields.Type, got.Type)
			assert.True(t, testBc.StartTime.Equal(got.StartTime))
			assert.True(t, testBc.EndTime.Equal(got.EndTime))

			// Update non-existent should fail
			_, err = UpdateBroadcastEvent(ctx, store, &BroadcastEvent{ID: testNonExistentID})
			assert.Error(t, err)
		})
	}
}

func TestDeleteBroadcastEvent(t *testing.T) {
	for _, kind := range storeKinds {
		t.Run(kind, func(t *testing.T) {
			ctx, store := setupStore(t, kind)

			const (
				targetID    = "broadcast-event-delete-1"
				bystanderID = "broadcast-event-delete-10"
				persistID   = "broadcast-event-delete-persist"
				unrelatedID = "broadcast-event-delete-unrelated"
			)

			// Pre-emptive cleanup for test IDs from any previous failed runs
			_ = DeleteBroadcastEvent(ctx, store, targetID)
			_ = DeleteBroadcastEvent(ctx, store, bystanderID)
			_ = DeleteBroadcastEvent(ctx, store, persistID)
			_ = DeleteBroadcastEvent(ctx, store, unrelatedID)

			t.Cleanup(func() {
				_ = DeleteBroadcastEvent(ctx, store, targetID)
				_ = DeleteBroadcastEvent(ctx, store, bystanderID)
				_ = DeleteBroadcastEvent(ctx, store, persistID)
				_ = DeleteBroadcastEvent(ctx, store, unrelatedID)
			})

			t.Run("CollisionSafety", func(t *testing.T) {
				target := BroadcastEvent{ID: targetID, ConfigID: testConfigID, Type: testBroadcastEventType}
				bystander := BroadcastEvent{ID: bystanderID, ConfigID: testConfigID, Type: testBroadcastEventType}

				_, err := CreateBroadcastEvent(ctx, store, target)
				assert.NoError(t, err)
				_, err = CreateBroadcastEvent(ctx, store, bystander)
				assert.NoError(t, err)

				err = DeleteBroadcastEvent(ctx, store, targetID)
				assert.NoError(t, err)

				// Target must be gone
				gotTarget, err := GetBroadcastEvent(ctx, store, targetID)
				assert.Error(t, err)
				assert.Nil(t, gotTarget)

				// Prefix-similar bystander must still be present
				gotBystander, err := GetBroadcastEvent(ctx, store, bystanderID)
				assert.NoError(t, err)
				assert.NotNil(t, gotBystander)
				assert.Equal(t, bystanderID, gotBystander.ID)
			})

			t.Run("NonExistentID", func(t *testing.T) {
				// First call on non-existent key
				err := DeleteBroadcastEvent(ctx, store, testNonExistentID)
				assertAcceptableDeleteError(t, err)

				// Second call (mimicking retry / double-delete)
				err = DeleteBroadcastEvent(ctx, store, testNonExistentID)
				assertAcceptableDeleteError(t, err)
			})

			t.Run("Persistence", func(t *testing.T) {
				entity := BroadcastEvent{ID: persistID, ConfigID: testConfigID, Type: testBroadcastEventType}

				// Insert and assert it exists
				_, err := CreateBroadcastEvent(ctx, store, entity)
				assert.NoError(t, err)

				got, err := GetBroadcastEvent(ctx, store, persistID)
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, persistID, got.ID)

				// Delete and assert it is gone
				err = DeleteBroadcastEvent(ctx, store, persistID)
				assert.NoError(t, err)

				gotPostDelete, err := GetBroadcastEvent(ctx, store, persistID)
				assert.Error(t, err)
				assert.Nil(t, gotPostDelete)
			})

			t.Run("ZeroIDDoesNotWipeOthers", func(t *testing.T) {
				// Create an unrelated entity beforehand
				unrelated := BroadcastEvent{ID: unrelatedID, ConfigID: testConfigID, Type: testBroadcastEventType}
				_, err := CreateBroadcastEvent(ctx, store, unrelated)
				assert.NoError(t, err)

				// Call Delete with zero value (empty string)
				err = DeleteBroadcastEvent(ctx, store, "")
				assertAcceptableDeleteError(t, err)

				// Positive assertion that unrelated entity is still present
				gotUnrelated, err := GetBroadcastEvent(ctx, store, unrelatedID)
				assert.NoError(t, err)
				assert.NotNil(t, gotUnrelated)
				assert.Equal(t, unrelatedID, gotUnrelated.ID)
			})
		})
	}
}

func TestSetBroadcastEventStart(t *testing.T) {
	for _, kind := range storeKinds {
		t.Run(kind, func(t *testing.T) {
			ctx, store := setupStore(t, kind)
			t.Cleanup(func() {
				_ = DeleteBroadcastEvent(ctx, store, testBroadcastEventStartID)
			})

			testBc := BroadcastEvent{
				ID:   testBroadcastEventStartID,
				Type: testBroadcastEventType,
			}

			_, err := CreateBroadcastEvent(ctx, store, testBc)
			assert.NoError(t, err)

			before := time.Now().UTC().Add(-1 * time.Second)
			err = SetBroadcastEventStart(ctx, store, testBc.ID)
			assert.NoError(t, err)
			after := time.Now().UTC().Add(1 * time.Second)

			got, err := GetBroadcastEvent(ctx, store, testBc.ID)
			assert.NoError(t, err)
			assert.True(t, got.StartTime.After(before) && got.StartTime.Before(after))

			// Start non-existent broadcast event should fail
			err = SetBroadcastEventStart(ctx, store, testNonExistentID)
			assert.Error(t, err)
		})
	}
}

func TestSetBroadcastEventEnd(t *testing.T) {
	for _, kind := range storeKinds {
		t.Run(kind, func(t *testing.T) {
			ctx, store := setupStore(t, kind)
			t.Cleanup(func() {
				_ = DeleteBroadcastEvent(ctx, store, testBroadcastEventEndID)
			})

			testBc := BroadcastEvent{
				ID:   testBroadcastEventEndID,
				Type: testBroadcastEventType,
			}

			_, err := CreateBroadcastEvent(ctx, store, testBc)
			assert.NoError(t, err)

			before := time.Now().UTC().Add(-1 * time.Second)
			err = SetBroadcastEventEnd(ctx, store, testBc.ID)
			assert.NoError(t, err)
			after := time.Now().UTC().Add(1 * time.Second)

			got, err := GetBroadcastEvent(ctx, store, testBc.ID)
			assert.NoError(t, err)
			assert.True(t, got.EndTime.After(before) && got.EndTime.Before(after))

			// End non-existent broadcast event should fail
			err = SetBroadcastEventEnd(ctx, store, testNonExistentID)
			assert.Error(t, err)
		})
	}
}
