/*
DESCRIPTION
  Site type and functions.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2019-2023 the Australian Ocean Lab (AusOcean).

  This file is free software: you can redistribute it and/or modify it
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
	"encoding/json"
	"fmt"
	"time"

	"github.com/ausocean/openfish/datastore"
)

// typeSite is the name of the datastore site type.
const typeSite = "Site"

// Site represents a cloud site.
type Site struct {
	Skey         int64
	Name         string
	OwnerEmail   string
	Latitude     float64
	Longitude    float64
	Timezone     float64
	NotifyPeriod int64
	Enabled      bool
	Confirmed    bool
	Premium      bool
	Public       bool
	Created      time.Time
}

// Encode serializes a Site into JSON.
func (site *Site) Encode() []byte {
	bytes, _ := json.Marshal(site)
	return bytes
}

// Decode deserializes a Site from JSON.
func (site *Site) Decode(b []byte) error {
	return json.Unmarshal(b, site)
}

// Copy copies a site to dst, or returns a copy of the site when dst is nil.
func (site *Site) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var s *Site
	if dst == nil {
		s = new(Site)
	} else {
		var ok bool
		s, ok = dst.(*Site)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*s = *site
	return s, nil
}

var siteCache datastore.Cache = datastore.NewEntityCache()

// GetCache returns the site cache.
func (site *Site) GetCache() datastore.Cache {
	return siteCache
}

// PutSite creates or updates a site.
func PutSite(ctx context.Context, store datastore.Store, site *Site) error {
	key := store.IDKey(typeSite, site.Skey)
	_, err := store.Put(ctx, key, site)
	return err
}

// CreateSite creates a site, or returns an error if a site with the given key exists.
func CreateSite(ctx context.Context, store datastore.Store, site *Site) error {
	key := store.IDKey(typeSite, site.Skey)
	return store.Create(ctx, key, site)
}

// GetSite returns a site by its site key.
func GetSite(ctx context.Context, store datastore.Store, skey int64) (*Site, error) {
	key := store.IDKey(typeSite, skey)
	var site Site
	err := store.Get(ctx, key, &site)
	if err != nil {
		return nil, err
	}

	return &site, err
}

// GetAllSites returns all sites.
func GetAllSites(ctx context.Context, store datastore.Store) ([]Site, error) {
	q := store.NewQuery(typeSite, false)
	var sites []Site
	_, err := store.GetAll(ctx, q, &sites)
	return sites, err
}

// GetPublicSites returns all public sites.
func GetPublicSites(ctx context.Context, store datastore.Store) ([]Site, error) {
	q := store.NewQuery(typeSite, false)
	q.Filter("Public=", true)
	var sites []Site
	_, err := store.GetAll(ctx, q, &sites)
	return sites, err
}

// DeleteSite deletes a site.
func DeleteSite(ctx context.Context, store datastore.Store, skey int64) error {
	key := store.IDKey(typeSite, skey)
	return store.DeleteMulti(ctx, []*datastore.Key{key})
}

// GetSiteName is a helper function that returns the site name string given the site key.
func GetSiteName(ctx context.Context, store datastore.Store, skey int64) (string, error) {
	s, err := GetSite(ctx, store, skey)
	if err != nil {
		return "", fmt.Errorf("could not get site for site key %v: %w", skey, err)
	}
	return s.Name, nil
}
