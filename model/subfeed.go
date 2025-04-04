/*
AUTHORS
  David Sutton <davidsutton@ausocean.org>

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
	"fmt"
	"time"

	"github.com/ausocean/openfish/datastore"
)

const typeSubFeed = "SubFeed" // SubFeed datastore type.

// SubFeed captures all the information about a short-lived instance of a feed.
type SubFeed struct {
	ID     int64     // Unique 10 digit ID.
	FeedID int64     // Parent Feed ID.
	Source string    // Feed source URL, e.g., a YouTube URL, or a URL to an AusOcean data stream (such as weather data).
	Active bool      // True if active, false if historical.
	Start  time.Time // Start time.
	Finish time.Time // Finish time.
}

// Copy copies a SubFeed to dst, or returns a copy of the SubFeed when dst is nil.
func (f *SubFeed) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var f2 *SubFeed
	if dst == nil {
		f2 = new(SubFeed)
	} else {
		var ok bool
		f2, ok = dst.(*SubFeed)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*f2 = *f
	return f2, nil
}

// GetCache returns nil, indicating no caching.
func (f *SubFeed) GetCache() datastore.Cache {
	return nil
}

// GetSubFeed retrieves a SubFeed entity from the datastore by its ID.
func GetSubFeed(ctx context.Context, store datastore.Store, ID, feedID int64) (*SubFeed, error) {
	key := store.NameKey(typeSubFeed, fmt.Sprintf("%d.%d", feedID, ID))

	subfeed := &SubFeed{}
	err := store.Get(ctx, key, subfeed)
	if err != nil {
		return nil, fmt.Errorf("error getting subfeed by ID (%d): %w", ID, err)
	}

	return subfeed, nil
}

// GetAllSubFeeds retrieves all SubFeed entities from the datastore for a given FeedID.
func GetSubFeedsByFeed(ctx context.Context, store datastore.Store, feedID int64) ([]SubFeed, error) {
	q := store.NewQuery(typeSubFeed, false, "FeedID", "ID")
	q.FilterField("FeedID", "=", feedID)
	subfeeds := []SubFeed{}
	_, err := store.GetAll(ctx, q, &subfeeds)
	if err != nil {
		return nil, fmt.Errorf("error getting all subfeeds: %w", err)
	}

	return subfeeds, nil
}

// CreateSubFeed creates a subfeed, or returns an error if a subfeed with the given ID exists.
func CreateSubFeed(ctx context.Context, store datastore.Store, subfeed *SubFeed) error {
	key := store.NameKey(typeSubFeed, fmt.Sprintf("%d.%d", subfeed.FeedID, subfeed.ID))
	return store.Create(ctx, key, subfeed)
}

// UpdateSubFeed updates a subfeed, or returns an error if the subfeed does not exist.
func UpdateSubFeed(ctx context.Context, store datastore.Store, subfeed *SubFeed) (*SubFeed, error) {
	key := store.NameKey(typeSubFeed, fmt.Sprintf("%d.%d", subfeed.FeedID, subfeed.ID))
	updated := &SubFeed{}
	err := store.Update(ctx, key, func(e datastore.Entity) {
		_subfeed := e.(*SubFeed)
		_subfeed.ID = subfeed.ID
		_subfeed.FeedID = subfeed.FeedID
		_subfeed.Source = subfeed.Source
		_subfeed.Active = subfeed.Active
		_subfeed.Start = subfeed.Start
		_subfeed.Finish = subfeed.Finish
	}, updated)
	return updated, err
}

// MarkSubFeedInactive updates the Active field of a given subfeed to false.
func MarkSubFeedInactive(ctx context.Context, store datastore.Store, ID, feedID int64) error {
	key := store.NameKey(typeSubFeed, fmt.Sprintf("%d.%d", feedID, ID))
	return store.Update(ctx, key, func(e datastore.Entity) {
		subfeed := e.(*SubFeed)
		subfeed.Active = false
	}, &SubFeed{})
}

// DeleteSubFeed deletes a subfeed, or returns an error if the subfeed does not exist.
func DeleteSubFeed(ctx context.Context, store datastore.Store, ID, feedID int64) error {
	key := store.NameKey(typeSubFeed, fmt.Sprintf("%d.%d", feedID, ID))
	return store.Delete(ctx, key)
}
