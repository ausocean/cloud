/*
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2022-2024 the Australian Ocean Lab (AusOcean).

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
	"encoding/json"
	"time"

	"github.com/ausocean/openfish/datastore"
)

const typeBinary = "Binary"

// Binary represents binary data.
//
// The Media ID (MID) can be any identifier that uniquely identifies
// the media source, but conventionally it is formed from the 48-bit
// MAC address followed by the 4-bit encoding of the pin of the
// source device. See ToMID.
type Binary struct {
	MID       int64          // Media ID.
	Timestamp int64          // Timestamp (in seconds).
	Type      string         // MIME type, if any.
	Data      []byte         `datastore:",noindex"` // Binary data.
	Date      time.Time      // Date/time last updated.
	Key       *datastore.Key `datastore:"__key__"` // Not persistent but populated upon reading from the datastore.
}

// Encode serializes binary data into JSON.
func (bin *Binary) Encode() []byte {
	bytes, _ := json.Marshal(bin)
	return bytes
}

// Decode deserializes binary data from JSON.
func (bin *Binary) Decode(b []byte) error {
	return json.Unmarshal(b, bin)
}

// Copy copies binary data bin to dst, or returns a copy of bin when dst is nil.
func (bin *Binary) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var b *Binary
	if dst == nil {
		b = new(Binary)
	} else {
		var ok bool
		b, ok = dst.(*Binary)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*b = *bin
	return b, nil
}

// GetCache returns the binary cache.
func (bin *Binary) GetCache() datastore.Cache {
	return nil
}

// KeyID returns the Binary key ID as an unsigned integer.
func (bin *Binary) KeyID() uint64 {
	return uint64(bin.Key.ID)
}

// PutBinary creates or updates binary data.
func PutBinary(ctx context.Context, store datastore.Store, bin *Binary) error {
	k := store.IDKey(typeBinary, datastore.IDKey(bin.MID, bin.Timestamp, 0))
	_, err := store.Put(ctx, k, bin)
	return err
}

// GetBinary gets binary data by MID and timestamp.
func GetBinary(ctx context.Context, store datastore.Store, mid, ts int64) (*Binary, error) {
	k := store.IDKey(typeBinary, datastore.IDKey(mid, ts, 0))
	var bin Binary
	err := store.Get(ctx, k, &bin)
	if err != nil {
		return nil, err
	}
	return &bin, err
}
