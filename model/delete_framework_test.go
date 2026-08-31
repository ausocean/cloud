/*
AUTHORS
  David Sutton <davidsutton@ausocean.org>

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

package model_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/ausocean/cloud/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T, kind string) (datastore.Store, func()) {
	var err error
	var store datastore.Store
	var cleanup func()
	switch kind {
	case "cloud":
		if os.Getenv("AUSOCEAN_CREDENTIALS") == "" {
			t.Skip("AUSOCEAN_CREDENTIALS required to access cloud datastore")
		}
		store, err = datastore.NewStore(t.Context(), kind, "ausocean/test", "")
		cleanup = func() {}
	case "file":
		// We return the cleanup function for the filestore to abstract the
		// directory name from the caller.
		cleanup = func() { os.RemoveAll("teststore") }
		cleanup()
		store, err = datastore.NewStore(t.Context(), kind, "teststore", "")
	}

	if err != nil {
		t.Fatalf("could not create store (kind: %s): %v", kind, err)
	}
	return store, cleanup
}

// create is the signature required for creating an entity to delete in the test.
type createInt64ID func(ctx context.Context, store datastore.Store, id int64) error

// delete is the signature required for deleting entities in the test.
type deleteInt64ID func(ctx context.Context, store datastore.Store, id int64) error

// assertCreatedInt64ID asserts that the entity was created. This is typically a thin wrapper around
// a get method.
//
// The closure should cause test failures rather than returning an error.
type assertCreatedInt64ID func(t *testing.T, ctx context.Context, store datastore.Store, id int64)

// assertDeletedByIDInt64ID asserts that the entity was deleted such that gets by the id fail. This is
// typically a wrapper of the getX method.
//
// The closure should cause test failures rather than returning an error.
type assertDeletedByIDInt64ID func(t *testing.T, ctx context.Context, store datastore.Store, id int64)

// assertDeletedByAllInt64ID asserts that the entity and all indexes were deleted from the store. This is
// typically a wrapper around all getX and getXbyY methods on an entity.
//
// The closure should cause test failures rather than returning an error.
type assertDeletedByAllInt64ID func(t *testing.T, ctx context.Context, store datastore.Store, id int64)

// testDeleteTypeInt64ID implements the common test cases for testing entity types against ADR 0002.
func testDeleteTypeInt64ID(t *testing.T, ctx context.Context, store datastore.Store,
	create createInt64ID,
	delete deleteInt64ID,
	assertCreated assertCreatedInt64ID,
	assertDeletedByID assertDeletedByIDInt64ID,
	assertDeletedByAll assertDeletedByAllInt64ID,
) {
	const (
		// Share a decimal prefix of 333333333. Both fit under 32-bit limits.
		idA int64 = 333333333
		idB int64 = 3333333330

		nonExistentID int64 = 555555555
	)

	isGracefulErr := func(err error) bool {
		return err == nil || errors.Is(err, datastore.ErrNoSuchEntity) || os.IsNotExist(err)
	}

	t.Run("CollisionSafety", func(t *testing.T) {
		// Create entities.
		err := create(ctx, store, idA)
		assert.NoError(t, err)
		err = create(ctx, store, idB)
		assert.NoError(t, err)

		// Register cleanup.
		t.Cleanup(func() {
			_ = delete(ctx, store, idA)
			_ = delete(ctx, store, idB)
		})

		// Delete entity A.
		err = delete(ctx, store, idA)
		assert.NoError(t, err)

		// Assert that entity A was deleted.
		assertDeletedByID(t, ctx, store, idA)

		// Assert that entity B was created, and still exists.
		assertCreated(t, ctx, store, idB)
	})

	t.Run("NonExistentID", func(t *testing.T) {
		// Delete non-existent entity and expect a graceful error.
		err := delete(ctx, store, nonExistentID)
		assert.True(t, isGracefulErr(err))
	})

	t.Run("DoubleDeleteRetry", func(t *testing.T) {
		// Create entity.
		err := create(ctx, store, idA)
		assert.NoError(t, err)

		// Register cleanup.
		t.Cleanup(func() {
			_ = delete(ctx, store, idA)
		})

		// Delete Entity.
		err = delete(ctx, store, idA)
		assert.NoError(t, err)

		// Assert that the entity is deleted.
		assertDeletedByID(t, ctx, store, idA)

		// Retry the delete, and expect a graceful error.
		err = delete(ctx, store, idA)
		assert.True(t, isGracefulErr(err))
	})

	t.Run("Persistence", func(t *testing.T) {
		// Create entity.
		err := create(ctx, store, idA)
		assert.NoError(t, err)

		// Assert that the entity was created.
		assertCreated(t, ctx, store, idA)

		// Register cleanup.
		t.Cleanup(func() {
			_ = delete(ctx, store, idA)
		})

		// Delete entity.
		err = delete(ctx, store, idA)

		// Assert the entity, and all indices are correctly deleted.
		assertDeletedByAll(t, ctx, store, idA)
	})

	t.Run("ZeroIDDoesNotWipeOthers", func(t *testing.T) {
		// Create entity.
		err := create(ctx, store, idA)
		assert.NoError(t, err)

		// Register cleanup.
		t.Cleanup(func() {
			_ = delete(ctx, store, idA)
		})

		// Delete entity with the zero ID.
		err = delete(ctx, store, 0)
		assert.True(t, isGracefulErr(err))

		// Assert that the created entity was not deleted.
		assertCreated(t, ctx, store, idA)
	})
}

// createUUID is the signature required for creating an entity to delete in the test.
type createUUID func(ctx context.Context, store datastore.Store, id uuid.UUID) error

// deleteUUID is the signature required for deleting entities in the test.
type deleteUUID func(ctx context.Context, store datastore.Store, id uuid.UUID) error

// assertCreatedUUID asserts that the entity was created. This is typically a thin wrapper around
// a get method.
//
// The closure should cause test failures rather than returning an error.
type assertCreatedUUID func(t *testing.T, ctx context.Context, store datastore.Store, id uuid.UUID)

// assertDeletedByIDUUID asserts that the entity was deleted such that gets by the id fail. This is
// typically a wrapper of the getX method.
//
// The closure should cause test failures rather than returning an error.
type assertDeletedByIDUUID func(t *testing.T, ctx context.Context, store datastore.Store, id uuid.UUID)

// assertDeletedByAllUUID asserts that the entity and all indexes were deleted from the store. This is
// typically a wrapper around all getX and getXbyY methods on an entity.
//
// The closure should cause test failures rather than returning an error.
type assertDeletedByAllUUID func(t *testing.T, ctx context.Context, store datastore.Store, id uuid.UUID)

// testDeleteTypeUUID implements the common test cases for testing UUID entity types against ADR 0002.
func testDeleteTypeUUID(t *testing.T, ctx context.Context, store datastore.Store,
	create createUUID,
	delete deleteUUID,
	assertCreated assertCreatedUUID,
	assertDeletedByID assertDeletedByIDUUID,
	assertDeletedByAll assertDeletedByAllUUID,
) {
	var (
		// Share a common prefix to verify prefix matching/collision safety.
		idA = uuid.MustParse("33333333-3333-3333-3333-333333333333")
		idB = uuid.MustParse("33333333-3333-3333-3333-333333333330")

		nonExistentID = uuid.MustParse("55555555-5555-5555-5555-555555555555")
	)

	isGracefulErr := func(err error) bool {
		return err == nil || errors.Is(err, datastore.ErrNoSuchEntity) || os.IsNotExist(err)
	}

	t.Run("CollisionSafety", func(t *testing.T) {
		// Create entities.
		err := create(ctx, store, idA)
		assert.NoError(t, err)
		err = create(ctx, store, idB)
		assert.NoError(t, err)

		// Register cleanup.
		t.Cleanup(func() {
			_ = delete(ctx, store, idA)
			_ = delete(ctx, store, idB)
		})

		// Delete entity A.
		err = delete(ctx, store, idA)
		assert.NoError(t, err)

		// Assert that entity A was deleted.
		assertDeletedByID(t, ctx, store, idA)

		// Assert that entity B was created, and still exists.
		assertCreated(t, ctx, store, idB)
	})

	t.Run("NonExistentID", func(t *testing.T) {
		// Delete non-existent entity and expect a graceful error.
		err := delete(ctx, store, nonExistentID)
		assert.True(t, isGracefulErr(err))
	})

	t.Run("DoubleDeleteRetry", func(t *testing.T) {
		// Create entity.
		err := create(ctx, store, idA)
		assert.NoError(t, err)

		// Register cleanup.
		t.Cleanup(func() {
			_ = delete(ctx, store, idA)
		})

		// Delete Entity.
		err = delete(ctx, store, idA)
		assert.NoError(t, err)

		// Assert that the entity is deleted.
		assertDeletedByID(t, ctx, store, idA)

		// Retry the delete, and expect a graceful error.
		err = delete(ctx, store, idA)
		assert.True(t, isGracefulErr(err))
	})

	t.Run("Persistence", func(t *testing.T) {
		// Create entity.
		err := create(ctx, store, idA)
		assert.NoError(t, err)

		// Assert that the entity was created.
		assertCreated(t, ctx, store, idA)

		// Register cleanup.
		t.Cleanup(func() {
			_ = delete(ctx, store, idA)
		})

		// Delete entity.
		err = delete(ctx, store, idA)

		// Assert the entity, and all indices are correctly deleted.
		assertDeletedByAll(t, ctx, store, idA)
	})

	t.Run("ZeroIDDoesNotWipeOthers", func(t *testing.T) {
		// Create entity.
		err := create(ctx, store, idA)
		assert.NoError(t, err)

		// Register cleanup.
		t.Cleanup(func() {
			_ = delete(ctx, store, idA)
		})

		// Delete entity with the zero/nil UUID.
		err = delete(ctx, store, uuid.Nil)
		assert.True(t, isGracefulErr(err))

		// Assert that the created entity was not deleted.
		assertCreated(t, ctx, store, idA)
	})
}
