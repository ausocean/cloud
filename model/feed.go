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
	"time"

	"github.com/ausocean/openfish/datastore"
)

const (
	typeFeed = "Feed" // Feed datastore type.
)

// Feed is an entity in the datastore that represents information about a particular feed.
type Feed struct {
	ID      string    // AusOcean assigned Feed ID.
	Name    string    // Display name, e.g., “Rapid Bay Live Stream”.
	Area    string    // Location, e.g., “SA” or “FNQ”.
	Class   string    // Feed class, e.g., “Video” or “Data”.
	Source  string    // Feed source URL, e.g., a YouTube URL, or a URL to an AusOcean data stream (such as weather data).
	Params  string    // Optional params to be applied to the source.
	Bundle  []string  // Feed IDs of other feeds bundled with this feed, or nil.
	Created time.Time // Time the feed entity was created.
}

// Copy copies a Feed to dst, or returns a copy of the Feed when dst is nil.
func (f *Feed) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var f2 *Feed
	if dst == nil {
		f2 = new(Feed)
	} else {
		var ok bool
		f2, ok = dst.(*Feed)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*f2 = *f
	return f2, nil
}

// GetCache returns nil, indicating no caching.
func (f *Feed) GetCache() datastore.Cache {
	return nil
}
