/*
DESCRIPTION
  Datastore log type and functions.

AUTHORS
  Deborah Baker <deborah@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean).

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
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/google/uuid"
)

// typeLog is the name of the datastore log type.
const typeLog = "Log"

// Log represents a logged note about a device or a site to help keep track of
// where and when particular devices have been used and how.
type Log struct {
	UUID      string    // Log ID.
	Skey      int64     // Site key.
	Dkey      int64     // Device key.
	DeviceMAC int64     // Encoded MAC address of a device.
	Note      string    // Notes made about device or site.
	Created   time.Time // Time the log was written.
	Level     string    // Log level of importance.
}

// Copy copies a log to dst, or returns a copy of the log when dst is nil.
func (log *Log) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var d *Log
	if dst == nil {
		d = new(Log)
	} else {
		var ok bool
		d, ok = dst.(*Log)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*d = *log
	return d, nil
}

// GetCache returns the log cache.
func (log *Log) GetCache() datastore.Cache {
	return nil
}

// PutLog puts the passed log into the datastore. The Created field will be filled with the current time,
// and a unique ID is generated to fill the UUID field.
func PutLog(ctx context.Context, store datastore.Store, log *Log) error {
	log.Created = time.Now()
	log.UUID = uuid.New().String()
	key := store.NameKey(typeLog, log.UUID)
	_, err := store.Put(ctx, key, log)
	return err
}

// GetLogsByDevice returns all logs for a device with the given MAC address.
func GetLogsByDevice(ctx context.Context, store datastore.Store, DeviceMAC int64) ([]Log, error) {
	q := store.NewQuery(typeLog, false, "UUID")
	q.Filter("DeviceMAC =", DeviceMAC)
	var logs []Log
	_, err := store.GetAll(ctx, q, &logs)
	return logs, err
}

// GetLogsBySite returns all logs for a site with the given Skey.
func GetLogsBySite(ctx context.Context, store datastore.Store, Skey int64) ([]Log, error) {
	q := store.NewQuery(typeLog, false, "UUID")
	q.Filter("Skey =", Skey)
	var logs []Log
	_, err := store.GetAll(ctx, q, &logs)
	return logs, err
}

// DeleteLog deletes a Log with the given UUID.
func DeleteLog(ctx context.Context, store datastore.Store, UUID string) error {
	key := store.NameKey(typeLog, UUID)
	return store.DeleteMulti(ctx, []*datastore.Key{key})
}
