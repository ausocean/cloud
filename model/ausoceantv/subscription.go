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

import "time"

// Subscription is an entity in the datastore that represents the relationship between a subscriber and a feed.
type Subscription struct {
	SubscriberID string    // Subscriber’s ID.
	FeedID       string    // Feed’s ID.
	Class        string    // Subscription class, e.g., “Day”, “Month”, or “Year”.
	Prefs        string    // User’s preferences for the presentation of this stream, e.g., “Top, Favorite”.
	Start        time.Time // Start time of the subscription.
	Finish       time.Time // Finish time of the subscription.
	Renew        bool      // True if the subscription should auto-renew.
}
