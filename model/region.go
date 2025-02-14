/*
AUTHORS
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2025 the Australian Ocean Lab (AusOcean).

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
	"errors"
	"fmt"

	"github.com/ausocean/openfish/datastore"
)

const (
	TypeSubscriberRegion = "SubscriberRegion" // SubscriberRegion datastore type.
)

// SubscriberRegion is an entity in the datastore that represents information about a particular regional location.
// This type is to be used for results from the (regions) type from Autocomplete using Google Places API.
type SubscriberRegion struct {
	SubscriberID             int64  `json:"subscriber_id"`
	Locality                 string `json:"locality"`
	Sublocality              string `json:"sublocality"`
	PostalCode               string `json:"postal_code"`
	Country                  string `json:"country"`
	AdministrativeAreaLevel1 string `json:"administrative_area_level_1"` // State eg. South Australia.
	AdministrativeAreaLevel2 string `json:"administrative_area_level_2"` // Council eg. City of Onkaparinga.
}

// Copy copies a SubscriberRegion to dst, or returns a copy of the SubscriberRegion when dst is nil.
func (r *SubscriberRegion) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var r2 *SubscriberRegion
	if dst == nil {
		r2 = new(SubscriberRegion)
	} else {
		var ok bool
		r2, ok = dst.(*SubscriberRegion)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*r2 = *r
	return r2, nil
}

// GetCache returns nil, indicating no caching.
func (r *SubscriberRegion) GetCache() datastore.Cache {
	return nil
}

// CreateSubscriberRegion creates a new SubscriberRegion in the datastore.
func CreateSubscriberRegion(ctx context.Context, store datastore.Store, r *SubscriberRegion) error {
	if r == nil {
		return errors.New("SubscriberRegion is nil")
	}

	// If the SubscriberRegion has an ID, use that.
	if r.SubscriberID == 0 {
		return fmt.Errorf("SubscriberRegion must have a non-zero SubscriberID")
	}
	// Since datastore uses kind and key to reference a particular entity, using the subscriberID as the SubscriberRegion key is fine.
	key := store.IDKey(TypeSubscriberRegion, r.SubscriberID)
	err := store.Create(ctx, key, r)
	if err != nil {
		return fmt.Errorf("error creating SubscriberRegion: %w", err)
	}
	return nil
}
