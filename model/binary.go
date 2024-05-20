/*
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2022 the Australian Ocean Lab (AusOcean).

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

const typeBinaryData = "BinaryData"

// BinaryData a cloud type for storing binary data.
type BinaryData struct {
	Mac       int64
	Timestamp int64
	Fmt       string
	Data      []byte
	Pin       string
	Date      time.Time
}

// GetBinaryData retreives BinaryData entities for a given media ID, optionally
// filtered by timestamp(s). If ts is given with len(ts) == 1, then a single entity
// with the matching timestamp will be returned. If the ts is given with len(ts) == 2
// then multiple entities corresponding to the range of ts[0] to ts[1] will be
// given.
func GetBinaryData(ctx context.Context, store datastore.Store, mid int64, ts []int64) ([]BinaryData, error) {
	q, err := newBinaryDataQuery(store, mid, ts, false)
	if err != nil {
		return nil, fmt.Errorf("could not create new binary data query: %w", err)
	}
	var bds []BinaryData
	_, err = store.GetAll(ctx, q, &bds)
	return bds, err
}

func newBinaryDataQuery(store datastore.Store, mid int64, ts []int64, keysOnly bool) (datastore.Query, error) {
	q := store.NewQuery(typeBinaryData, false, "MID", "Timestamp")
	mac, _ := FromMID(mid)
	q.Filter("mac =", MacEncode(mac))

	if ts == nil {
		return q, nil
	}

	if len(ts) < 1 || len(ts) > 2 {
		return nil, fmt.Errorf("unexpected ts length: %d", len(ts))
	}

	if len(ts) == 1 {
		q.Filter("timestamp =", ts[0])
		return q, nil
	}

	q.Filter("timestamp >=", ts[0])
	if ts[1] < datastore.EpochEnd {
		q.Filter("timestamp <", ts[1])
	}
	q.Order("timestamp")

	return q, nil
}
