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
	"time"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

// timeStore implements a TimeStore that uses a datastore for persistence.
type timeStore struct {
	store datastore.Store
}

// NewTimeStore returns a TimeStore that uses a datastore for peristence.
func NewTimeStore(store datastore.Store) TimeStore {
	return &timeStore{store: store}
}

// Get retrieves a notification time stored in a datastore variable.
// We prepend an underscore to keep the variable private.
func (ts *timeStore) Get(skey int64, key string) (time.Time, error) {
	v, err := model.GetVariable(context.Background(), ts.store, skey, "_"+key)
	switch err {
	case nil:
		return v.Updated, nil
	case datastore.ErrNoSuchEntity:
		return time.Time{}, nil // We've never sent this kind of notice previously.
	default:
		return time.Time{}, err // Unexpected datastore error.
	}
}

// Set updates a notification time stored in a datatore variable.
func (ts *timeStore) Set(skey int64, key string, t time.Time) error {
	return model.PutVariable(context.Background(), ts.store, skey, "_"+key, "")
}
