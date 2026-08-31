/*
DESCRIPTION
  Service notification type.

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

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package model

import (
	"context"
	"fmt"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/google/uuid"
)

const TypeNotification = "Notification"

// Notification stores the information about a service notification to be sent,
// or which has already been sent.
type Notification struct {
	UUID      string    `json:"uuid"`
	Service   string    `json:"service"` // Unique name of the service responsible for the notification.
	Emails    []string  `json:"emails"`  // Email addresses of recipients
	Kind      string    `json:"kind"`    // Notification kind.
	Msg       string    `json:"msg"`     // Notification body.
	CreatedAt time.Time `json:"created-at"`
	datastore.NoCache
}

// Copy implements Entity interface.
func (n *Notification) Copy(dst datastore.Entity) (datastore.Entity, error) {
	return datastore.CopyEntity(n, dst)
}

// PutNotification puts the passed notification into the datastore, overwriting if it already exists.
// A UUID will be generated for the notification if it does not already have a valid UUID. The CreatedAt
// field will always be updated.
func PutNotification(ctx context.Context, store datastore.Store, n *Notification) error {
	if n == nil {
		return fmt.Errorf("notification must not be nil: %w", datastore.ErrInvalidValue)
	}
	if uuid.Validate(n.UUID) != nil {
		n.UUID = uuid.NewString()
	}
	n.CreatedAt = time.Now()

	key := store.NameKey(TypeNotification, n.UUID)
	_, err := store.Put(ctx, key, n)
	return err
}

// GetNotification gets a notification by its UUID.
func GetNotification(ctx context.Context, store datastore.Store, UUID string) (*Notification, error) {
	if uuid.Validate(UUID) != nil {
		return nil, datastore.ErrInvalidStoreID
	}

	key := store.NameKey(TypeNotification, UUID)
	n := &Notification{}
	err := store.Get(ctx, key, n)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	return n, nil
}

// DeleteNotification deletes a notification by its UUID.
func DeleteNotification(ctx context.Context, store datastore.Store, UUID string) error {
	if uuid.Validate(UUID) != nil {
		return datastore.ErrInvalidStoreID
	}

	key := store.NameKey(TypeNotification, UUID)
	return store.Delete(ctx, key)
}
