/*
DESCRIPTION
  Tests for model/broadcast.go.

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
	"os"
	"testing"
	"time"

	"github.com/ausocean/cloud/datastore"
)

const (
	testBroadcastID      = "broadcast-crud-1"
	testBroadcastStartID = "broadcast-start-1"
	testBroadcastEndID   = "broadcast-end-1"
	testConfigID         = "config-1"
	testBroadcastName    = "Original Name"
	testBroadcastDesc    = "Original Description"
	testBroadcastType    = "video"
	testNonExistentID    = "non-existent-id"
	testUpdatedName      = "Updated Name"
	testUpdatedDesc      = "Updated Description"
	testUpdatedType      = "audio"
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

func TestBroadcastCGUD(t *testing.T) {
	for _, kind := range storeKinds {
		t.Run(kind, func(t *testing.T) {
			ctx, store := setupStore(t, kind)
			t.Cleanup(func() {
				_ = DeleteBroadcast(ctx, store, testBroadcastID)
			})

			now := time.Now().Truncate(time.Second)
			testBc := Broadcast{
				ID:                 testBroadcastID,
				ConfigID:           testConfigID,
				StartTime:          now,
				ScheduledStartTime: now.Add(30 * time.Minute),
				EndTime:            now.Add(2 * time.Hour),
				ScheduledEndTime:   now.Add(2 * time.Hour),
				Name:               testBroadcastName,
				Description:        testBroadcastDesc,
				Type:               testBroadcastType,
			}

			// 1. Create Broadcast
			created, err := CreateBroadcast(ctx, store, testBc)
			if err != nil {
				t.Fatalf("CreateBroadcast failed: %v", err)
			}
			if created == nil || created.ID != testBc.ID || created.Name != testBc.Name {
				t.Errorf("unexpected created broadcast: %+v", created)
			}

			// Duplicate Create should fail
			_, err = CreateBroadcast(ctx, store, testBc)
			if err == nil {
				t.Errorf("expected duplicate CreateBroadcast to fail, got nil error")
			}

			// 2. Get Broadcast
			got, err := GetBroadcast(ctx, store, testBc.ID)
			if err != nil {
				t.Fatalf("GetBroadcast failed: %v", err)
			}
			if got.ID != testBc.ID {
				t.Errorf("expected ID %q, got %q", testBc.ID, got.ID)
			}
			if got.ConfigID != testBc.ConfigID {
				t.Errorf("expected ConfigID %q, got %q", testBc.ConfigID, got.ConfigID)
			}
			if !got.StartTime.Equal(testBc.StartTime) {
				t.Errorf("expected StartTime %v, got %v", testBc.StartTime, got.StartTime)
			}
			if !got.ScheduledStartTime.Equal(testBc.ScheduledStartTime) {
				t.Errorf("expected ScheduledStartTime %v, got %v", testBc.ScheduledStartTime, got.ScheduledStartTime)
			}
			if !got.EndTime.Equal(testBc.EndTime) {
				t.Errorf("expected EndTime %v, got %v", testBc.EndTime, got.EndTime)
			}
			if !got.ScheduledEndTime.Equal(testBc.ScheduledEndTime) {
				t.Errorf("expected ScheduledEndTime %v, got %v", testBc.ScheduledEndTime, got.ScheduledEndTime)
			}
			if got.Name != testBc.Name {
				t.Errorf("expected Name %q, got %q", testBc.Name, got.Name)
			}
			if got.Description != testBc.Description {
				t.Errorf("expected Description %q, got %q", testBc.Description, got.Description)
			}
			if got.Type != testBc.Type {
				t.Errorf("expected Type %q, got %q", testBc.Type, got.Type)
			}

			// Get non-existent Broadcast should fail
			_, err = GetBroadcast(ctx, store, testNonExistentID)
			if err == nil {
				t.Errorf("expected GetBroadcast for non-existent ID to fail, got nil error")
			}

			// 3. Update Broadcast
			updatedFields := &Broadcast{
				ID:                 testBc.ID,
				StartTime:          now.Add(10 * time.Minute),
				ScheduledStartTime: now.Add(40 * time.Minute),
				EndTime:            now.Add(3 * time.Hour),
				ScheduledEndTime:   now.Add(3 * time.Hour),
				Name:               testUpdatedName,
				Description:        testUpdatedDesc,
				Type:               testUpdatedType,
			}

			updated, err := UpdateBroadcast(ctx, store, updatedFields)
			if err != nil {
				t.Fatalf("UpdateBroadcast failed: %v", err)
			}
			if updated.Name != updatedFields.Name || updated.Description != updatedFields.Description || updated.Type != updatedFields.Type {
				t.Errorf("UpdateBroadcast returned unexpected data: %+v", updated)
			}

			// Verify update in store
			gotUpdated, err := GetBroadcast(ctx, store, testBc.ID)
			if err != nil {
				t.Fatalf("GetBroadcast after update failed: %v", err)
			}
			if gotUpdated.Name != updatedFields.Name {
				t.Errorf("expected updated Name %q, got %q", updatedFields.Name, gotUpdated.Name)
			}
			if gotUpdated.Description != updatedFields.Description {
				t.Errorf("expected updated Description %q, got %q", updatedFields.Description, gotUpdated.Description)
			}
			if gotUpdated.Type != updatedFields.Type {
				t.Errorf("expected updated Type %q, got %q", updatedFields.Type, gotUpdated.Type)
			}
			if !gotUpdated.StartTime.Equal(updatedFields.StartTime) {
				t.Errorf("expected updated StartTime %v, got %v", updatedFields.StartTime, gotUpdated.StartTime)
			}
			if !gotUpdated.ScheduledStartTime.Equal(updatedFields.ScheduledStartTime) {
				t.Errorf("expected updated ScheduledStartTime %v, got %v", updatedFields.ScheduledStartTime, gotUpdated.ScheduledStartTime)
			}
			if !gotUpdated.EndTime.Equal(updatedFields.EndTime) {
				t.Errorf("expected updated EndTime %v, got %v", updatedFields.EndTime, gotUpdated.EndTime)
			}
			if !gotUpdated.ScheduledEndTime.Equal(updatedFields.ScheduledEndTime) {
				t.Errorf("expected updated ScheduledEndTime %v, got %v", updatedFields.ScheduledEndTime, gotUpdated.ScheduledEndTime)
			}

			// Update non-existent broadcast should fail
			_, err = UpdateBroadcast(ctx, store, &Broadcast{ID: testNonExistentID})
			if err == nil {
				t.Errorf("expected UpdateBroadcast for non-existent ID to fail, got nil error")
			}

			// 4. Delete Broadcast
			err = DeleteBroadcast(ctx, store, testBc.ID)
			if err != nil {
				t.Fatalf("DeleteBroadcast failed: %v", err)
			}

			// Verify deleted
			_, err = GetBroadcast(ctx, store, testBc.ID)
			if err == nil {
				t.Errorf("expected GetBroadcast after DeleteBroadcast to fail, got nil error")
			}
		})
	}
}

func TestStartBroadcast(t *testing.T) {
	for _, kind := range storeKinds {
		t.Run(kind, func(t *testing.T) {
			ctx, store := setupStore(t, kind)
			t.Cleanup(func() {
				_ = DeleteBroadcast(ctx, store, testBroadcastStartID)
			})

			testBc := Broadcast{
				ID:   testBroadcastStartID,
				Name: "Start Test",
			}

			_, err := CreateBroadcast(ctx, store, testBc)
			if err != nil {
				t.Fatalf("CreateBroadcast failed: %v", err)
			}

			before := time.Now().Add(-1 * time.Second)
			err = StartBroadcast(ctx, store, testBc.ID)
			if err != nil {
				t.Fatalf("StartBroadcast failed: %v", err)
			}
			after := time.Now().Add(1 * time.Second)

			got, err := GetBroadcast(ctx, store, testBc.ID)
			if err != nil {
				t.Fatalf("GetBroadcast failed: %v", err)
			}
			if got.StartTime.Before(before) || got.StartTime.After(after) {
				t.Errorf("expected StartTime to be between %v and %v, got %v", before, after, got.StartTime)
			}

			// Start non-existent broadcast should fail
			err = StartBroadcast(ctx, store, testNonExistentID)
			if err == nil {
				t.Errorf("expected StartBroadcast on non-existent ID to fail, got nil error")
			}
		})
	}
}

func TestEndBroadcast(t *testing.T) {
	for _, kind := range storeKinds {
		t.Run(kind, func(t *testing.T) {
			ctx, store := setupStore(t, kind)
			t.Cleanup(func() {
				_ = DeleteBroadcast(ctx, store, testBroadcastEndID)
			})

			testBc := Broadcast{
				ID:   testBroadcastEndID,
				Name: "End Test",
			}

			_, err := CreateBroadcast(ctx, store, testBc)
			if err != nil {
				t.Fatalf("CreateBroadcast failed: %v", err)
			}

			before := time.Now().Add(-1 * time.Second)
			err = EndBroadcast(ctx, store, testBc.ID)
			if err != nil {
				t.Fatalf("EndBroadcast failed: %v", err)
			}
			after := time.Now().Add(1 * time.Second)

			got, err := GetBroadcast(ctx, store, testBc.ID)
			if err != nil {
				t.Fatalf("GetBroadcast failed: %v", err)
			}
			if got.EndTime.Before(before) || got.EndTime.After(after) {
				t.Errorf("expected EndTime to be between %v and %v, got %v", before, after, got.EndTime)
			}

			// End non-existent broadcast should fail
			err = EndBroadcast(ctx, store, testNonExistentID)
			if err == nil {
				t.Errorf("expected EndBroadcast on non-existent ID to fail, got nil error")
			}
		})
	}
}
