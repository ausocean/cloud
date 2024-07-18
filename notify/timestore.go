/*
LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  It is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package notify

import (
	"context"
	"errors"
	"time"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

// TimeStore is an interface for notification persistence
type TimeStore interface {
	Sendable(context.Context, int64, time.Duration, string) (bool, error) // Returns true if a message is sendable.
	Sent(context.Context, int64, string) error                            // Records the time a message was sent.
}

// timeStore implements a TimeStore that uses a datastore for persistence.
type timeStore struct {
	store datastore.Store
}

// NewStore returns a TimeStore that uses a datastore for
// notification persistence.
func NewStore(store datastore.Store) TimeStore {
	return &timeStore{store: store}
}

// NewTimeStore is retained for backwards compatibility.
// The notification period is now set by the WithPeriod option.
func NewTimeStore(store datastore.Store, period time.Duration) TimeStore {
	return &timeStore{store: store}
}

// Sendable retrieves a notification time stored in a datastore
// variable and returns true either if (1) the specified period has
// elapsed since the last time a message for the given site and key
// was sent or (2) a message is being sent for the first time.
func (ts *timeStore) Sendable(ctx context.Context, skey int64, period time.Duration, key string) (bool, error) {
	v, err := model.GetVariable(ctx, ts.store, skey, "_"+key) // Prepend an underscore to keep the variable private.

	switch {
	case err == nil:
		return time.Since(v.Updated) >= period, nil
	case errors.Is(err, datastore.ErrNoSuchEntity):
		return true, nil // No record of sending this kind of message.
	default:
		return true, err // Unexpected datastore error.
	}
}

// Sent records the time that a message with the given site and key
// was sent.
func (ts *timeStore) Sent(ctx context.Context, skey int64, key string) error {
	return model.PutVariable(ctx, ts.store, skey, "_"+key, "") // Automatically updates the time.
}
