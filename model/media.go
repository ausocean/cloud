/*
DESCRIPTION
  Datastore media type and functions.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2019 the Australian Ocean Lab (AusOcean).

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, eitherc version 3 of the License, or
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

	"github.com/ausocean/cloud/datastore"
)

// typeMedia is the name of our datastore type.
const typeMedia = "Media"

// Media represents a unique media source. Each media has a unique
// Media ID (MID) which serves as the datastore key. The description
// is optional.
type Media struct {
	MID         int64
	Description string
	Updated     time.Time
}

// Encode serializes a Media into tab-separated values.
func (m *Media) Encode() []byte {
	return []byte(fmt.Sprintf("%d\t%s\t%d", m.MID, m.Description, m.Updated.Unix()))
}

// Decode deserializes a Media from tab-separated values.
func (m *Media) Decode(b []byte) error {
	p := strings.Split(string(b), "\t")
	if len(p) != 3 {
		return datastore.ErrDecoding
	}
	var err error
	m.MID, err = strconv.ParseInt(p[0], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	m.Description = p[1]
	ts, err := strconv.ParseInt(p[2], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	m.Updated = time.Unix(ts, 0)
	return nil
}

// Copy is not currently implemented.
func (m *Media) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

// GetCache returns nil, indicating no caching.
func (m *Media) GetCache() datastore.Cache {
	return nil
}

// PutMedia creates or updates media.
func PutMedia(ctx context.Context, store datastore.Store, m *Media) error {
	key := store.IDKey(typeMedia, m.MID)
	m.Updated = time.Now()
	_, err := store.Put(ctx, key, m)
	return err
}

// GetMedia returns media by its Media ID.
func GetMedia(ctx context.Context, store datastore.Store, mid int64) (*Media, error) {
	key := store.IDKey(typeMedia, mid)
	var m Media
	err := store.Get(ctx, key, &m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}
