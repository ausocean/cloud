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

type subscriptionOption func(*Subscription) error

var errDuplicateSubscriptions = errors.New("multiple subscriptions exist for given SubscriberID and FeedID")

// Subscription is an entity in the datastore that represents the relationship between a subscriber and a feed.
type Subscription struct {
	ID           string    // Stripe ID for the Subscription.
	SubscriberID string    // Subscriber’s ID.
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
func GetSubscription(ctx context.Context, store datastore.Store, sid string, fid int64) (*Subscription, error) {
	q := store.NewQuery(typeSubscription, false, "SubscriberID", "FeedID")
	q.FilterField("SubscriberID", "=", sid)
	q.FilterField("FeedID", "=", fid)

	var subscriptions []Subscription
	_, err := store.GetAll(ctx, q, &subscriptions)

	if err != nil {
		return nil, fmt.Errorf("unable to get subscription with subscriberID: %s, feedID: %d: %w", sid, fid, err)
	}

	if len(subscriptions) == 0 {
		return nil, datastore.ErrNoSuchEntity
	}

	if len(subscriptions) > 1 {
		return nil, fmt.Errorf("for SubscriberID: %s, and FeedID: %d, failed with error: %w", sid, fid, errDuplicateSubscriptions)
	}

	return &subscriptions[0], nil
}

// WithStartEnd sets the start and finish time of the subscription to the passed unix timestamps. This should be
// used whenever a subscription is linked with an external service which sets the start and end times.
func WithStartEnd(start, end int64) subscriptionOption {
	return func(s *Subscription) error {
		s.Start = time.Unix(start, 0).UTC()
		s.Finish = time.Unix(end, 0).UTC()

		// Approximate durations
		oneDay := 24 * time.Hour
		oneMonth := oneDay * 27 // All months are at least 27 days.
		oneYear := oneDay * 364 // A year is MORE than 364 days.

		diff := s.Finish.Sub(s.Start)
		switch {
		case diff >= oneYear:
			s.Class = SubscriptionYear
		case diff >= oneMonth:
			s.Class = SubscriptionMonth
		case diff == oneDay:
			s.Class = SubscriptionDay
		default:
			return errors.New("bad length of subscription")
		}
		return nil
	}
}

// WithSubscriptionClass sets the start and finish times for the subscription by using time offsets.
// NOTE: This is only the preferred method for setting day subscriptions. All other subscriptions should
// be set with WithStartEnd.
func WithSubscriptionClass(class string) subscriptionOption {
	return func(s *Subscription) error {
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

		s.Start = start
		s.Finish = end
		s.Class = class
		return nil
	}
}

// CreateSubscription creates a subscription for a given subscriber ID and feed ID.
//
// NOTE: For a month subscription, the date of renewal will not always be the same each month, and the month gets normalised.
// see time.Time.AddDate() for further details.
func CreateSubscription(ctx context.Context, store datastore.Store, id string, sid string, fid int64, prefs string, renew bool, opts ...subscriptionOption) error {
	s := &Subscription{ID: id, SubscriberID: sid, FeedID: fid, Prefs: prefs, Renew: renew}

	for i, opt := range opts {
		err := opt(s)
		if err != nil {
			return fmt.Errorf("error applying opt[%d]: %w", i, err)
		}
	}

	key := store.NameKey(typeSubscription, fmt.Sprintf("%s.%d", sid, fid))
	return store.Create(ctx, key, s)
}

// UpdateSubscription updates a subscription.
func UpdateSubscription(ctx context.Context, store datastore.Store, s *Subscription) error {
	key := store.NameKey(typeSubscription, fmt.Sprintf("%s.%d", s.SubscriberID, s.FeedID))
	_, err := store.Put(ctx, key, s)
	return err
}

// GetSubscriptions returns all the subscriptions for a given subscriber ID (sid).
func GetSubscriptions(ctx context.Context, store datastore.Store, sid string) ([]Subscription, error) {
	q := store.NewQuery(typeSubscription, false, "SubscriberID", "FeedID")
	q.FilterField("SubscriberID", "=", sid)

	var subs = []Subscription{}
	_, err := store.GetAll(ctx, q, &subs)
	return subs, err
}
