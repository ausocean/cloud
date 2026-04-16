/*
DESCRIPTION
  Datastore user type and functions.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2019-2023 the Australian Ocean Lab (AusOcean).

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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/datastore"
)

// typeUser is the name of the datastore user type.
const typeUser = "User"

// User represents a cloud user. One user exists for
// each site that a Google account is associated with. The same Google
// account may have different permissions for different sites, or none
// at all.
type User struct {
	Skey    int64     // Site key.
	Email   string    // User email address.
	UserID  string    // Google account ID. Not currently used.
	Perm    int64     // User's site permissions.
	Created time.Time // Date/time created.
}

// Encode serializes a User into tab-separated values.
func (user *User) Encode() []byte {
	return []byte(fmt.Sprintf("%d\t%s\t%s\t%d\t%d", user.Skey, user.Email, user.UserID, user.Perm, user.Created.Unix()))
}

// Decode deserializes a User from tab-separated values.
func (user *User) Decode(b []byte) error {
	p := strings.Split(string(b), "\t")
	if len(p) != 5 {
		return datastore.ErrDecoding
	}
	var err error
	user.Skey, err = strconv.ParseInt(p[0], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	user.Email = p[1]
	user.UserID = p[2]
	user.Perm, err = strconv.ParseInt(p[3], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	ts, err := strconv.ParseInt(p[4], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	user.Created = time.Unix(ts, 0)
	return nil
}

// Copy copies a user to dst, or returns a copy of the user when dst is nil.
func (user *User) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var u *User
	if dst == nil {
		u = new(User)
	} else {
		var ok bool
		u, ok = dst.(*User)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*u = *user
	return u, nil
}

var userCache datastore.Cache = datastore.NewEntityCache()

// GetCache returns the user cache.
func (user *User) GetCache() datastore.Cache {
	return userCache
}

// PermissionText returns text describing the user's permissions.
func (user *User) PermissionText() string {
	switch user.Perm {
	case 0:
		return "No Permission"
	case ReadPermission:
		return "Read Only"
	case ReadPermission | WritePermission:
		return "Read Write"
	case ReadPermission | WritePermission | AdminPermission:
		return "Admin"
	default:
		return "Unknown Permission"
	}
}

// PutUser creates or updates a user.
func PutUser(ctx context.Context, store datastore.Store, user *User) error {
	key := store.NameKey(typeUser, strconv.FormatInt(user.Skey, 10)+"."+user.Email)
	_, err := store.Put(ctx, key, user)
	return err
}

// GetUser returns a user by its key, which is the concatenated
// Skey.Email.
func GetUser(ctx context.Context, store datastore.Store, skey int64, email string) (*User, error) {
	key := store.NameKey(typeUser, strconv.FormatInt(skey, 10)+"."+email)
	var user User
	err := store.Get(ctx, key, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetUsers returns all users corresponding to a given email or domain, sorted
// by site key.
func GetUsers(ctx context.Context, store datastore.Store, email string) ([]User, error) {
	var users []User
	for _, v := range []string{email, email[strings.Index(email, "@"):]} {
		q := store.NewQuery(typeUser, false, "Skey", "Email")
		q.Filter("Email =", v)
		_, err := store.GetAll(ctx, q, &users)
		if err != nil {
			return nil, fmt.Errorf("could not find users for %s, %w", v, err)
		}
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Skey < users[j].Skey })
	return users, nil
}

// GetUsersBySite returns all of the users for a given site.
func GetUsersBySite(ctx context.Context, store datastore.Store, skey int64) ([]User, error) {
	var users []User
	q := store.NewQuery(typeUser, false, "Skey", "Email")
	q.Filter("Skey =", skey)
	_, err := store.GetAll(ctx, q, &users)
	return users, err
}

// DeleteUser deletes a user.
func DeleteUser(ctx context.Context, store datastore.Store, skey int64, email string) error {
	key := store.NameKey(typeUser, strconv.FormatInt(skey, 10)+"."+email)
	return store.DeleteMulti(ctx, []*datastore.Key{key})
}
