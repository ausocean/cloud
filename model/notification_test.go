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
	"testing"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotification(t *testing.T) {
	t.Run("Copy", func(t *testing.T) {
		orig := &model.Notification{
			UUID:      uuid.NewString(),
			Service:   "telemetry-service",
			Emails:    []string{"admin@ausocean.org"},
			Kind:      "alert",
			Msg:       "High temperature detected",
			CreatedAt: time.Now().Truncate(time.Millisecond),
		}

		copiedEntity, err := orig.Copy(nil)
		assert.NoError(t, err)

		copied, ok := copiedEntity.(*model.Notification)
		require.True(t, ok)
		assert.Equal(t, orig.UUID, copied.UUID)
		assert.Equal(t, orig.Service, copied.Service)
		assert.Equal(t, orig.Emails, copied.Emails)
		assert.Equal(t, orig.Kind, copied.Kind)
		assert.Equal(t, orig.Msg, copied.Msg)
		assert.Equal(t, orig.CreatedAt, copied.CreatedAt)
	})

	kinds := []string{"file", "cloud"}

	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			store, cleanup := setup(t, kind)
			t.Cleanup(cleanup)
			ctx := t.Context()

			t.Run("InvalidInputs", func(t *testing.T) {
				// Put nil notification
				err := model.PutNotification(ctx, store, nil)
				assert.ErrorIs(t, err, datastore.ErrInvalidValue)

				// Get with invalid UUID string
				_, err = model.GetNotification(ctx, store, "invalid-uuid")
				assert.ErrorIs(t, err, datastore.ErrInvalidStoreID)
			})

			t.Run("AutoGenerateUUIDAndTimestamp", func(t *testing.T) {
				n := &model.Notification{
					// Empty UUID triggers auto-generation
					Service: "sensor-service",
					Kind:    "info",
					Msg:     "heartbeat",
				}

				beforePut := time.Now().Add(-1 * time.Second)
				err := model.PutNotification(ctx, store, n)
				assert.NoError(t, err)

				// Verify UUID auto-generated and valid
				assert.NoError(t, uuid.Validate(n.UUID))

				// Verify CreatedAt updated
				assert.True(t, n.CreatedAt.After(beforePut))

				// Clean up
				_ = model.DeleteNotification(ctx, store, n.UUID)
			})

			t.Run("DeleteNotification", func(t *testing.T) {
				create := func(ctx context.Context, store datastore.Store, id uuid.UUID) error {
					n := &model.Notification{
						UUID:    id.String(),
						Service: "test-service",
						Kind:    "alert",
						Msg:     "test notification",
					}
					return model.PutNotification(ctx, store, n)
				}

				delete := func(ctx context.Context, store datastore.Store, id uuid.UUID) error {
					return model.DeleteNotification(ctx, store, id.String())
				}

				assertCreated := func(t *testing.T, ctx context.Context, store datastore.Store, id uuid.UUID) {
					n, err := model.GetNotification(ctx, store, id.String())
					assert.NoError(t, err)
					assert.NotNil(t, n)
					assert.Equal(t, id.String(), n.UUID)
				}

				assertDeletedByID := func(t *testing.T, ctx context.Context, store datastore.Store, id uuid.UUID) {
					n, err := model.GetNotification(ctx, store, id.String())
					assert.Error(t, err)
					assert.Nil(t, n)
				}

				assertDeletedByAll := func(t *testing.T, ctx context.Context, store datastore.Store, id uuid.UUID) {
					assertDeletedByID(t, ctx, store, id)
				}

				testDeleteTypeUUID(
					t,
					ctx,
					store,
					create,
					delete,
					assertCreated,
					assertDeletedByID,
					assertDeletedByAll,
				)
			})
		})
	}
}
