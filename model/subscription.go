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

	"github.com/ausocean/openfish/datastore"
)

const (
	SubscriptionDay   = "Day"
	SubscriptionMonth = "Month"
	SubscriptionYear  = "Year"
)

const (
	NoFeedID = 0 // Corresponds to a subscription witout a specified Feed ID.
)

const (
	typeSubscription = "Subscription" // Subscription datastore type.
)

var errDuplicateSubscriptions = errors.New("multiple subscriptions exist for given SubscriberID and FeedID")

// Subscription is an entity in the datastore that represents the relationship between a subscriber and a feed.
type Subscription struct {
	SubscriberID int64     // Subscriber’s ID.
	FeedID       int64     // Feed’s ID.
	Class        string    // Subscription class, e.g., “Day”, “Month”, or “Year”.
	Prefs        string    // User’s preferences for the presentation of this stream, e.g., “Top, Favorite”.
	Start        time.Time // Start time of the subscription.
	Finish       time.Time // Finish time of the subscription.
	Renew        bool      // True if the subscription should auto-renew.
}

// Copy copies a Subscription to dst, or returns a copy of the Subscription when dst is nil.
func (s *Subscription) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var s2 *Subscription
	if dst == nil {
		s2 = new(Subscription)
	} else {
		var ok bool
		s2, ok = dst.(*Subscription)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*s2 = *s
	return s2, nil
}

// GetCache returns nil, indicating no caching.
func (s *Subscription) GetCache() datastore.Cache {
	return nil
}

// GetSubscription gets a subscription for a given subscriberID (sid) and feedID (fid).
func GetSubscription(ctx context.Context, store datastore.Store, sid, fid int64) (*Subscription, error) {
	q := store.NewQuery(typeSubscription, false, "SubscriptionID", "FeedID")
	q.FilterField("SubscriberID", "=", sid)
	q.FilterField("FeedID", "=", fid)

	var subscriptions []Subscription
	_, err := store.GetAll(ctx, q, &subscriptions)

	if err != nil {
		return nil, fmt.Errorf("unable to get subscription with subscriberID: %d, feedID: %d: %w", sid, fid, err)
	}

	if len(subscriptions) > 1 {
		return nil, fmt.Errorf("for SubscriberID: %d, and FeedID: %d, failed with error: %w", sid, fid, errDuplicateSubscriptions)
	}

	return &subscriptions[0], nil
}

// CreateSubscription creates a subscription for a given subscriber ID and feed ID.
//
// NOTE: For a month subscription, the date of renewal will not always be the same each month, and the month gets normalised.
// see time.Time.AddDate() for further details.
func CreateSubscription(ctx context.Context, store datastore.Store, sid, fid int64, class, prefs string, renew bool) error {
	// Calculate characteristics of the subscription.
	start := time.Now().Truncate(time.Hour * 24).UTC() // Start the subscription at the start of the current day.
	var end time.Time
	switch class {
	case SubscriptionDay:
		end = start.AddDate(0, 0, 1)
	case SubscriptionMonth:
		end = start.AddDate(0, 1, 0)
	case SubscriptionYear:
		end = start.AddDate(1, 0, 0)
	}

	s := &Subscription{sid, fid, class, prefs, start, end, renew}

	key := store.NameKey(typeSubscription, fmt.Sprintf("%d.%d", sid, fid))
	return store.Create(ctx, key, s)
}

// UpdateSubscription updates a subscription.
func UpdateSubscription(ctx context.Context, store datastore.Store, s *Subscription) error {
	key := store.NameKey(typeSubscription, fmt.Sprintf("%d.%d", s.SubscriberID, s.FeedID))
	_, err := store.Put(ctx, key, s)
	return err
}

// GetSubscriptions returns all the subscriptions for a given subscriber ID (sid).
func GetSubscriptions(ctx context.Context, store datastore.Store, sid int64) ([]Subscription, error) {
	q := store.NewQuery(typeSubscription, false, "SubscriberID", "FeedID")
	q.FilterField("SubscriberID", "=", sid)

	var subs = []Subscription{}
	_, err := store.GetAll(ctx, q, &subs)
	return subs, err
}
