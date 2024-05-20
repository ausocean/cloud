/*
DESCRIPTION
  Datastore credential type and functions.

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

	"github.com/ausocean/openfish/datastore"
)

// typeCredential is the name of our datastore type.
const typeCredential = "Credential"

// Credential represents a user credential to access media. One
// credential exists for each media item for which a user, group or
// domain is permitted access. The same user/group/domain may have
// different credentials for different media, for example, have write
// access for one media item but only have read access for another.
type Credential struct {
	MID     int64     // Media ID.
	Name    string    // User name, email address, group or domain.
	Perm    int64     // Permissions.
	Created time.Time // Date/time created.
}

const (
	ReadPermission  = 0x0001 // Required to find or play media.
	WritePermission = 0x0002 // Required to update media.
	AdminPermission = 0x0004 // Required to create or delete media.
	SuperPermission = 0x0100 // Required to modify other's credentials.
)

// IsReadable returns true for a credential with read permissions.
func (c *Credential) IsReadable() bool {
	return c.Perm&ReadPermission != 0
}

// IsWritable returns true for a credential with write permissions.
func (c *Credential) IsWritable() bool {
	return c.Perm&WritePermission != 0
}

// IsAdmin returns true for a credential with admin permissions.
func (c *Credential) IsAdmin() bool {
	return c.Perm&AdminPermission != 0
}

// IsSuper returns true for a credential with super permissions.
func (c *Credential) IsSuper() bool {
	return c.Perm&SuperPermission != 0
}

// Encode serializes a Credential into tab-separated values.
func (c *Credential) Encode() []byte {
	return []byte(fmt.Sprintf("%d\t%s\t%d\t%d", c.MID, c.Name, c.Perm, c.Created.Unix()))
}

// Decode deserializes a Credential from tab-separated values.
func (c *Credential) Decode(b []byte) error {
	p := strings.Split(string(b), "\t")
	if len(p) != 4 {
		return datastore.ErrDecoding
	}
	var err error
	c.MID, err = strconv.ParseInt(p[0], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	c.Name = p[1]
	c.Perm, err = strconv.ParseInt(p[2], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	ts, err := strconv.ParseInt(p[3], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	c.Created = time.Unix(ts, 0)
	return nil
}

// Copy is not currently implemented.
func (c *Credential) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

// GetCache returns nil, indicating no caching.
func (c *Credential) GetCache() datastore.Cache {
	return nil
}

// PutCredential creates or updates a Credential using MID.Name as the key.
func PutCredential(ctx context.Context, store datastore.Store, c *Credential) error {
	key := store.NameKey(typeCredential, strconv.FormatInt(c.MID, 10)+"."+c.Name)
	_, err := store.Put(ctx, key, c)
	return err
}

// GetCredential returns a Credential for given a MID and name.
func GetCredential(ctx context.Context, store datastore.Store, mid int64, name string) (*Credential, error) {
	key := store.NameKey(typeCredential, strconv.FormatInt(mid, 10)+"."+name)
	var c Credential
	err := store.Get(ctx, key, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

// GetCredentialsByMID returns all credentials for a given MID.
func GetCredentialsByMID(ctx context.Context, store datastore.Store, mid int64) ([]Credential, error) {
	q := store.NewQuery(typeCredential, false, "MID", "Name")
	q.Filter("MID =", mid)
	q.Order("Name")

	var creds []Credential
	_, err := store.GetAll(ctx, q, &creds)
	if err != nil {
		return nil, err
	}
	return creds, err
}

// GetCredentialsByName returns all credentials for a given name.
func GetCredentialsByName(ctx context.Context, store datastore.Store, name string) ([]Credential, error) {
	q := store.NewQuery(typeCredential, false, "MID", "Name")
	q.Filter("Name =", name)
	q.Order("MID")

	var creds []Credential
	_, err := store.GetAll(ctx, q, &creds)
	if err != nil {
		return nil, err
	}
	return creds, err
}

// DeleteCredential deletes a single credential.
func DeleteCredential(ctx context.Context, store datastore.Store, mid int64, name string) error {
	key := store.NameKey(typeCredential, strconv.FormatInt(mid, 10)+"."+name)
	return store.DeleteMulti(ctx, []*datastore.Key{key})
}

// DeleteCredentials deletes all credential for a given name.
func DeleteCredentials(ctx context.Context, store datastore.Store, name string) error {
	q := store.NewQuery(typeCredential, true, "MID", "Name")
	q.Filter("Name =", name)

	keys, err := store.GetAll(ctx, q, nil)
	if err != nil {
		return err
	}

	return store.DeleteMulti(ctx, keys)
}

// HasPermission returns true if the user's email address or domain
// has the requested permission for the given MID. Note that the
// permission for a user (i.e., one associated with an email address)
// takes priority over the permission for a domain. This means that a
// particular user may have write or admin permission whereas other
// users in the domain are limited to read permissions. The
// single-character name "@" represents any domain, effectively
// granting public access.
func HasPermission(ctx context.Context, store datastore.Store, mid int64, email string, perm int64) bool {
	return hasPermission(ctx, store, mid, email, perm) || hasPermission(ctx, store, 0, email, perm)
}

func hasPermission(ctx context.Context, store datastore.Store, mid int64, email string, perm int64) bool {
	c, err := GetCredential(ctx, store, mid, email)
	if err == nil && c.Perm&perm != 0 {
		return true
	}
	i := strings.Index(email, "@")
	if i == -1 || len(email[i:]) == 1 {
		return false // Not an email address.
	}
	c, err = GetCredential(ctx, store, mid, email[i:])
	if err == nil && c.Perm&perm != 0 {
		return true
	}
	c, err = GetCredential(ctx, store, mid, "@")
	if err == nil && c.Perm&perm != 0 {
		return true
	}
	return false
}
