/*
DESCRIPTION
  Text datastore type and functions.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2019 the Australian Ocean Lab (AusOcean).

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
  or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
  License for more details.

  You should have received a copy of the GNU General Public License in
  gpl.txt. If not, see http://www.gnu.org/licenses/.
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

const (
	typeText = "Text" // Text datastore type.
)

// Text represents text data.
//
// The Media ID (MID) can be any identifier that uniquely identifies
// the media source, but conventionally it is formed from the 48-bit
// MAC address followed by the 4-bit encoding of the pin of the
// source device. See ToMID.
type Text struct {
	MID       int64          // Media ID.
	Timestamp int64          // Timestamp (in seconds).
	Type      string         // Text type.
	Data      string         `datastore:",noindex"` // Text data.
	Date      time.Time      // Date/time last updated.
	Key       *datastore.Key `datastore:"__key__"` // Not persistent but populated upon reading from the datastore.
}

// Encode serializes a Text entity into tab-separated values.
func (t *Text) Encode() []byte {
	return []byte(fmt.Sprintf("%d\t%d\t%s\t%s\t%d", t.MID, t.Timestamp, t.Type, t.Data, t.Date.Unix()))
}

// Decode deserializes a Text entity from tab-separated values.
func (t *Text) Decode(b []byte) error {
	p := strings.Split(string(b), "\t")
	if len(p) != 5 {
		return datastore.ErrDecoding
	}
	var err error
	t.MID, err = strconv.ParseInt(p[0], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	t.Timestamp, err = strconv.ParseInt(p[1], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	t.Type = p[2]
	t.Data = p[3]
	ts, err := strconv.ParseInt(p[4], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	t.Date = time.Unix(ts, 0)
	return nil
}

// Copy is not currently implemented.
func (t *Text) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

// GetCache returns nil, indicating no caching.
func (t *Text) GetCache() datastore.Cache {
	return nil
}

// KeyID returns the Text key ID as an unsigned integer.
func (t *Text) KeyID() uint64 {
	return uint64(t.Key.ID)
}

// WriteText writes text to the datastore.
func WriteText(ctx context.Context, store datastore.Store, t *Text) error {
	if len(t.Data) == 0 {
		return nil
	}
	key := store.IDKey(typeText, datastore.IDKey(t.MID, t.Timestamp, 0))
	t.Date = time.Now()
	_, err := store.Put(ctx, key, t)
	if err != nil {
		return err
	}
	return nil
}

// GetText retrieves text for a given Media ID, optionally filtered by
// timestamp(s).
func GetText(ctx context.Context, store datastore.Store, mid int64, ts []int64) ([]Text, error) {
	q, err := newTextQuery(store, mid, ts, false)
	if err != nil {
		return nil, err
	}
	var texts []Text
	_, err = store.GetAll(ctx, q, &texts)
	return texts, err
}

// GetTextKeys retrieves text keys for a given Media ID, optionally
// filtered by timestamp(s).
func GetTextKeys(ctx context.Context, store datastore.Store, mid int64, ts []int64) ([]*datastore.Key, error) {
	q, err := newTextQuery(store, mid, ts, true)
	if err != nil {
		return nil, err
	}
	return store.GetAll(ctx, q, nil)
}

// DeleteText deletes all text for a given Media ID.
func DeleteText(ctx context.Context, store datastore.Store, mid int64) error {
	keys, err := GetTextKeys(ctx, store, mid, nil)
	if err != nil {
		return err
	}
	return store.DeleteMulti(ctx, keys)
}

// newTextQuery constructs a text query.
func newTextQuery(store datastore.Store, mid int64, ts []int64, keysOnly bool) (datastore.Query, error) {
	q := store.NewQuery(typeText, keysOnly, "MID", "Timestamp")
	q.Filter("MID =", mid)

	if ts != nil {
		if len(ts) > 1 {
			q.Filter("Timestamp >=", ts[0])
			if ts[1] < datastore.EpochEnd {
				q.Filter("Timestamp <", ts[1])
			}
			q.Order("Timestamp")
		} else if len(ts) > 0 {
			q.Filter("Timestamp =", ts[0])
		}
	}
	return q, nil
}
