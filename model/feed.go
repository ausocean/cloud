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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/openfish/datastore"
)

const (
	typeFeed = "Feed" // Feed datastore type.
)

// Feed is an entity in the datastore that represents information about a particular feed.
type Feed struct {
	ID      int64     // AusOcean assigned Feed ID.
	Name    string    // Display name, e.g., “Rapid Bay Live Stream”.
	Area    string    // Location, e.g., “SA” or “FNQ”.
	Class   string    // Feed class, e.g., “Video” or “Data”.
	Source  string    // Feed source URL, e.g., a YouTube URL, or a URL to an AusOcean data stream (such as weather data).
	Params  string    // Optional params to be applied to the source.
	Bundle  []int64   // Feed IDs of other feeds bundled with this feed, or nil.
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

// GetFeed retrieves a Feed entity from the datastore by its ID.
func GetFeed(ctx context.Context, store datastore.Store, id int64) (*Feed, error) {
	key := store.IDKey(typeFeed, id)

	feed := &Feed{}
	err := store.Get(ctx, key, feed)
	if err != nil {
		return nil, fmt.Errorf("error getting feed by ID (%d): %w", id, err)
	}

	return feed, nil
}

// GetAllFeeds retrieves all Feed entities from the datastore.
func GetAllFeeds(ctx context.Context, store datastore.Store) ([]Feed, error) {
	q := store.NewQuery(typeFeed, false, "ID")
	feeds := []Feed{}
	_, err := store.GetAll(ctx, q, &feeds)
	if err != nil {
		return nil, fmt.Errorf("error getting all feeds: %w", err)
	}

	return feeds, nil
}

// CreateFeed creates a feed, or returns an error if a feed with the given ID exists.
func CreateFeed(ctx context.Context, store datastore.Store, feed *Feed) error {
	key := store.IDKey(typeFeed, feed.ID)
	return store.Create(ctx, key, feed)
}

// UpdateFeed updates a feed, or returns an error if the feed does not exist.
func UpdateFeed(ctx context.Context, store datastore.Store, feed *Feed) (*Feed, error) {
	key := store.IDKey(typeFeed, feed.ID)
	updated := &Feed{}
	err := store.Update(ctx, key, func(e datastore.Entity) {
		_feed := e.(*Feed)
		_feed.ID = feed.ID
		_feed.Name = feed.Name
		_feed.Area = feed.Area
		_feed.Class = feed.Class
		_feed.Source = feed.Source
		_feed.Params = feed.Params
		_feed.Bundle = feed.Bundle
	}, updated)
	return updated, err
}

// SetFeedSource updates the source of a Feed for the given FeedID with the provided source.
func SetFeedSource(ctx context.Context, store datastore.Store, fid int64, source string) error {
	key := store.IDKey(typeFeed, fid)
	return store.Update(ctx, key, func(e datastore.Entity) {
		feed := e.(*Feed)
		feed.Source = source
	}, &Feed{})
}

// DeleteFeed deletes a feed, or returns an error if the feed does not exist.
func DeleteFeed(ctx context.Context, store datastore.Store, id int64) error {
	key := store.IDKey(typeFeed, id)
	return store.Delete(ctx, key)
}

// Constants for feed source types.
const (
	SourceAusOcean = iota
	SourceSubFeed
	SourceYouTube
	SourceUnknown
)

// SourceType returns the type of the feed's source.
func (f *Feed) SourceType() int {
	if strings.Contains(f.Source, "cloudblue") {
		return SourceAusOcean
	}
	if strings.Contains(f.Source, "youtube") {
		return SourceYouTube
	}
	if _, err := strconv.ParseInt(f.Source, 10, 64); err == nil {
		return SourceSubFeed
	}
	return SourceUnknown
}
