/*
DESCRIPTION
  Cron datastore type and functions.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2021 the Australian Ocean Lab (AusOcean).

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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/datastore"
)

const (
	typeCron  = "Cron" // Cron datastore type.
	minsInDay = 1440   // Minutes in the day.
)

var ErrInvalidTime = errors.New("invalid time")

// Cron represents a cloud cron which perform actions at
// specified times (to the nearest minute).
type Cron struct {
	Skey    int64     // Site key.
	ID      string    // Cron ID.
	Time    time.Time // Cron time.
	TOD     string    // Symbolic time of day, e.g., "Sunset", or repeating time "*30".
	Repeat  bool      // True if repeating time.
	Minutes int64     // Minutes since start of UTC day or repeat minutes.
	Action  string    // Action to be performed
	Var     string    // Action variable (if any).
	Data    string    `datastore:",noindex"` // Action data (if any).
	Enabled bool      // True if enabled, false otherwise.
}

// Encode serializes a Cron into tab-separated values.
func (c *Cron) Encode() []byte {
	return []byte(fmt.Sprintf("%d\t%s\t%d\t%s\t%t\t%d\t%s\t%s\t%s\t%t",
		c.Skey, c.ID, c.Time.Unix(), c.TOD, c.Repeat, c.Minutes, c.Action, c.Var, c.Data, c.Enabled))
}

// Decode deserializes a Cron from tab-separated values.
func (c *Cron) Decode(b []byte) error {
	p := strings.Split(string(b), "\t")
	if len(p) != 10 {
		return datastore.ErrDecoding
	}
	var err error
	c.Skey, err = strconv.ParseInt(p[0], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	c.ID = p[1]
	ts, err := strconv.ParseInt(p[2], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	c.Time = time.Unix(ts, 0)
	c.TOD = p[3]
	c.Repeat, err = strconv.ParseBool(p[4])
	if err != nil {
		return datastore.ErrDecoding
	}
	m, err := strconv.ParseInt(p[5], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	c.Minutes = m
	c.Action = p[6]
	c.Var = p[7]
	c.Data = p[8]
	c.Enabled, err = strconv.ParseBool(p[9])
	if err != nil {
		return datastore.ErrDecoding
	}
	return nil
}

// Copy is not currently implemented.
func (c *Cron) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

// GetCache returns nil, indicating no caching.
func (c *Cron) GetCache() datastore.Cache {
	return nil
}

// PutCron creates or updates a cron.
func PutCron(ctx context.Context, store datastore.Store, c *Cron) error {
	key := store.NameKey(typeCron, strconv.FormatInt(c.Skey, 10)+"."+c.ID)
	_, err := store.Put(ctx, key, c)
	return err
}

// GetCron gets a cron.
func GetCron(ctx context.Context, store datastore.Store, skey int64, id string) (*Cron, error) {
	key := store.NameKey(typeCron, strconv.FormatInt(skey, 10)+"."+id)
	c := new(Cron)
	err := store.Get(ctx, key, c)
	if err != nil {
		return nil, err
	}
	return c, err
}

// GetCronsBySite returns all the crons for a given site.
func GetCronsBySite(ctx context.Context, store datastore.Store, skey int64) ([]Cron, error) {
	q := store.NewQuery(typeCron, false, "Skey", "ID")
	q.Filter("Skey =", skey)
	q.Order("ID")
	var crons []Cron
	_, err := store.GetAll(ctx, q, &crons)
	return crons, err
}

// DeleteCron deletes a cron.
func DeleteCron(ctx context.Context, store datastore.Store, skey int64, id string) error {
	key := store.NameKey(typeCron, strconv.FormatInt(skey, 10)+"."+id)
	return store.DeleteMulti(ctx, []*datastore.Key{key})
}

// Helper functions.

// ParseTime parses a string representing a 24-hour time, i.e., hh:mm
// or hhmm, or a symbolic time of day, e.g., Sunrise or Sunset, and
// sets the cron time properties accordingly.
func (c *Cron) ParseTime(s string, tz float64) error {
	c.TOD = s
	split := strings.Split(s, ":")
	if len(split) == 2 {
		h := strings.TrimPrefix(split[0], "0")
		m := strings.TrimPrefix(split[1], "0")
		c.TOD = fmt.Sprintf("%s %s * * *", m, h)
	}
	return nil
}

// FormatTime formats the cron time either as hh:mm or the time of day.
func (c *Cron) FormatTime(tz float64) string {
	if c.TOD != "" {
		return c.TOD
	}
	mins := (c.Minutes + int64(tz*60)) % minsInDay
	h := mins / 60
	m := mins % 60
	return fmt.Sprintf("%02d:%02d", h, m)
}
