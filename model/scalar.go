/*
DESCRIPTION
  Scalar datastore type and functions.

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

	"github.com/ausocean/openfish/datastore"
)

const typeScalar = "Scalar" // Scalar datastore type.

// Scalar represents scalar data, such as a single analog (A) or
// digital (D) value.
//
// The ID can be any identifier that uniquely identifies the data
// source, but conventionally it is formed from the 48-bit MAC address
// followed by the 8-bit encoding of the pin of the source device. See
// ToSID.
type Scalar struct {
	ID        int64
	Timestamp int64
	Value     float64
	Key       *datastore.Key `datastore:"__key__" json:"-"` // Not persistent but populated upon reading from the datastore.
}

// Encode serializes a Scalar entity into tab-separated values.
func (s *Scalar) Encode() []byte {
	return []byte(fmt.Sprintf("%d\t%d\t%s", s.ID, s.Timestamp, s.FormatValue(3)))
}

// Decode deserializes a Scalar entity from tab-separated values.
func (s *Scalar) Decode(b []byte) error {
	p := strings.Split(string(b), "\t")
	if len(p) != 3 {
		return datastore.ErrDecoding
	}
	var err error
	s.ID, err = strconv.ParseInt(p[0], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	s.Timestamp, err = strconv.ParseInt(p[1], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	s.Value, err = strconv.ParseFloat(p[2], 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	return nil
}

// Copy is not currently implemented.
func (s *Scalar) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

// GetCache returns nil, indicating no caching.
func (s *Scalar) GetCache() datastore.Cache {
	return nil
}

// KeyID returns the scalar's key ID as an unsigned integer.
func (s *Scalar) KeyID() uint64 {
	return uint64(s.Key.ID)
}

// FormatValue formats a scalar's value as a string to the specified precision.
func (s *Scalar) FormatValue(prec int) string {
	if prec < 0 {
		panic(fmt.Sprintf("Scalar.FormatValue: negative precision: %d", prec))
	}
	if s.Value == float64(int64(s.Value)) {
		return strconv.FormatInt(int64(s.Value), 10)
	} else {
		return strconv.FormatFloat(s.Value, 'f', prec, 64)
	}
}

// PutScalar writes a scalar.
func PutScalar(ctx context.Context, store datastore.Store, s *Scalar) error {
	key := store.IDKey(typeScalar, datastore.IDKey(s.ID, s.Timestamp, 0))
	_, err := store.Put(ctx, key, s)
	if err != nil {
		return err
	}
	return nil
}

// GetScalar gets a single scalar by ID and timestamp.
func GetScalar(ctx context.Context, store datastore.Store, id int64, ts int64) (*Scalar, error) {
	key := store.IDKey(typeScalar, datastore.IDKey(id, ts, 0))
	s := new(Scalar)
	err := store.Get(ctx, key, s)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// GetScalars returns scalar data.
// When ts is a non-identical pair, it represents a time range.
// A value of -1 for the second value indicates no upper bound to the time range.
// When ts is a singleton or identical pair, it represents an exact time.
func GetScalars(ctx context.Context, store datastore.Store, id int64, ts []int64) ([]Scalar, error) {
	q := store.NewQuery(typeScalar, false, "ID", "Timestamp")
	q.Filter("ID =", id)
	filterByTime(q, ts)

	var data []Scalar
	_, err := store.GetAll(ctx, q, &data)
	return data, err
}

// GetScalarKeys is similiar to GetScalars, but returns the keys rather than the entities.
func GetScalarKeys(ctx context.Context, store datastore.Store, id int64, ts []int64) ([]*datastore.Key, error) {
	q := store.NewQuery(typeScalar, true, "ID", "Timestamp")
	q.Filter("ID =", id)
	filterByTime(q, ts)

	return store.GetAll(ctx, q, nil)
}

// filterByTime optionally adds timestamp filters to a Scalar query.
func filterByTime(q datastore.Query, ts []int64) {
	if ts == nil {
		return
	}
	if len(ts) > 1 && ts[1] != ts[0] {
		q.Filter("Timestamp >=", ts[0])
		if ts[1] > 0 && ts[1] < datastore.EpochEnd {
			q.Filter("Timestamp <", ts[1])
		}
		q.Order("Timestamp")
	} else if len(ts) > 0 {
		q.Filter("Timestamp =", ts[0])
	} else {
		panic(fmt.Sprintf("filterByTime: unexpected ts length: %d", len(ts)))
	}
}

// DeleteScalar deletes all scalars for a given ID.
func DeleteScalars(ctx context.Context, store datastore.Store, id int64) error {
	q := store.NewQuery(typeScalar, true, "ID")
	q.Filter("ID =", id)
	keys, err := store.GetAll(ctx, q, nil)
	if err != nil {
		return err
	}

	return store.DeleteMulti(ctx, keys)
}

// ToSID produces a Scalar ID from a MAC address and pin.  Unlike
// Media IDs, the pin is represented by 8 bits in order to accommodate
// 2-digit pin numbers.
func ToSID(mac, pin string) int64 {
	return MacEncode(mac)<<8 | int64(putScalarPin(pin))
}

// FromSID returns a MAC address and pin given a Scalar ID.
func FromSID(id int64) (string, string) {
	return MacDecode(id >> 8), getScalarPin(byte(id & 0xff))
}

// putScalarPin encodes a pin string, such as "A0", "D10" or "X22",
// into a byte. The most signifcant bit encodes the pin type, namely 0
// for A or D, or 1 for X. The remaining bits encode the 2-digit pin
// number, with numbers between 1 and 99 (inclusive) representing
// digital pins, e.g, 1 for D0, 2 for D1 and 100 and 127 (inclusive)
// representing analog pins, e.g., 100 for A0, 101 for A1, etc.
func putScalarPin(pin string) byte {
	if pin == "" {
		return 0
	}
	pn, _ := strconv.Atoi(pin[1:])
	switch pin[0] {
	case 'A':
		return byte(pn + 100)
	case 'D':
		return byte(pn + 1)
	case 'X':
		return 0x80 | byte(pn)
	}
	return 0
}

// getScalarPin decodes a byte into a pin string.
func getScalarPin(b byte) string {
	pn := int(b & 0x7f)
	if b&0x80 == 0 {
		if pn >= 100 {
			return "A" + strconv.Itoa(pn-100)
		} else if pn >= 1 {
			return "D" + strconv.Itoa(pn-1)
		} else {
			return ""
		}

	} else {
		return "X" + strconv.Itoa(pn)
	}
}

// GetLatestScalar finds the most recent scalar.
func GetLatestScalar(ctx context.Context, store datastore.Store, id int64) (*Scalar, error) {
	const countPeriod = 60 * time.Minute
	start := time.Now().Add(-countPeriod).Unix()
	keys, err := GetScalarKeys(ctx, store, id, []int64{start, -1})
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, datastore.ErrNoSuchEntity
	}
	_, ts, _ := datastore.SplitIDKey(keys[len(keys)-1].ID)
	return GetScalar(ctx, store, id, ts)
}
