/*
AUTHORS
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean).

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
	"time"

	"github.com/ausocean/cloud/utils"
	"github.com/ausocean/openfish/datastore"
)

const (
	typeSubscriber = "Subscriber" // Subscriber datastore type.
)

var errDuplicateSubscriberEmails = errors.New("more than one subscriber exists for a given email")

// Subscriber is an entity in the datastore representing a user who subscribes to AusOcean.TV.
type Subscriber struct {
	ID              int64     // AusOcean assigned Subscriber ID.
	AccountID       string    // Google Account ID. NB: May not be necessary.
	Email           string    // Subscriber's email address.
	GivenName       string    // Subscriber's given name.
	FamilyName      string    // Subscriber's family name.
	Area            []string  // Subscriber’s area(s) of interest.
	DemographicInfo string    // Optional demographic info about the subscriber, e.g., their postcode.
	PaymentInfo     string    // Info required to use a payments platform.
	Created         time.Time // Time the subscriber entity was created.
}

// Copy copies a Subscriber to dst, or returns a copy of the Subscriber when dst is nil.
func (s *Subscriber) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var s2 *Subscriber
	if dst == nil {
		s2 = new(Subscriber)
	} else {
		var ok bool
		s2, ok = dst.(*Subscriber)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*s2 = *s
	return s2, nil
}

// GetCache returns nil, indicating no caching.
func (s *Subscriber) GetCache() datastore.Cache {
	return nil
}

// CreateSubscriber creates a new subscriber from the passed subscriber (s).
//
// If the passed subscriber has an ID it will try to create a subscriber with that ID,
// which may result in ErrEntityExists.
//
// If the passed subscriber does not have an ID, a unique ID will be generated.
//
// NOTE: The Created field will be overwritten with the current time.
func CreateSubscriber(ctx context.Context, store datastore.Store, s *Subscriber) error {
	// Set the Created time.
	s.Created = time.Now()

	// If the subscriber has an ID, use that.
	if s.ID != 0 {
		key := store.IDKey(typeSubscriber, s.ID)
		return store.Create(ctx, key, s)
	}

	// Otherwise generate and use a unique ID.
	for {
		s.ID = utils.GenerateInt64ID()
		key := store.IDKey(typeSubscriber, s.ID)
		err := store.Create(ctx, key, s)
		if err == nil {
			return nil
		} else if err != datastore.ErrEntityExists {
			return fmt.Errorf("could not create subscriber: %v", err)
		}
	}
}

// GetSubscriberByEmail returns the subscriber with the given email if it exists.
func GetSubscriberByEmail(ctx context.Context, store datastore.Store, email string) (*Subscriber, error) {
	q := store.NewQuery(typeSubscriber, false)
	q.FilterField("Email", "=", email)
	var subs []Subscriber
	_, err := store.GetAll(ctx, q, &subs)
	if err != nil {
		return nil, fmt.Errorf("failed to get all subscribers: %w", err)
	}

	if len(subs) == 0 {
		return nil, datastore.ErrNoSuchEntity
	}

	if len(subs) > 1 {
		return nil, errDuplicateSubscriberEmails
	}

	return &subs[0], err
}

// GetSubscriber gets the subscriber with the given ID.
func GetSubscriber(ctx context.Context, store datastore.Store, id int64) (*Subscriber, error) {
	key := store.IDKey(typeSubscriber, id)
	sub := &Subscriber{}
	err := store.Get(ctx, key, sub)
	if err != nil {
		return nil, fmt.Errorf("unable to get Subscriber with id: %d: %v", id, err)
	}

	return sub, nil
}
