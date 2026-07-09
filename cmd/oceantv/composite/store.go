/*
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package composite

import (
	"context"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/ausocean/cloud/datastore"
)

type (
	ctx   = context.Context
	store = datastore.Store
	key   = datastore.Key
	ety   = datastore.Entity
)

// AusOceanStore returns a composite store that delegates to the
// appropriate store based on the kind of the entity. This tries to fix the
// awkwardness of selecting the right store based on the kind of the entity
// you're dealing with.
func AusOceanStore(settingsStore, mediaStore store) *Store {
	getKindFromQuery := func(query datastore.Query) string {
		getKindField := func(v reflect.Value) string {
			const kindField = "kind"
			field := v.Elem().FieldByName(kindField)
			if !field.IsValid() {
				panic("kind field not found")
			}
			return *(*string)(unsafe.Pointer(field.UnsafeAddr()))
		}

		switch q := query.(type) {
		case *datastore.CloudQuery:
			// CloudQuery wraps a google.Query, so we have to get this
			// first.
			const googleQueryField = "query"
			queryField := reflect.ValueOf(q).Elem().FieldByName(googleQueryField)
			if !queryField.IsValid() || queryField.IsNil() {
				panic("query field not found or nil")
			}
			return getKindField(queryField)
		case *datastore.FileQuery:
			return getKindField(reflect.ValueOf(q))
		default:
			panic(fmt.Sprintf("unsupported query type: %T", query))
		}
	}

	return NewStore(
		map[string]store{
			"Scalar":     mediaStore,
			"Text":       mediaStore,
			"MtsMedia":   mediaStore,
			"Device":     settingsStore,
			"Site":       settingsStore,
			"Signal":     settingsStore,
			"Notice":     settingsStore,
			"Trigger":    settingsStore,
			"Cron":       settingsStore,
			"Request":    settingsStore,
			"Variable":   settingsStore,
			"BinaryData": settingsStore,
			"User":       settingsStore,
			"Sensor":     settingsStore,
			"SensorV2":   settingsStore,
			"Actuator":   settingsStore,
			"ActuatorV2": settingsStore,
		},
		getKindFromQuery,
	)
}

// Store is a datastore "facade" that delegates to the appropriate
// store based on the kind of the entity. This is useful when you have multiple
// stores that you want to treat as a single store.
// Store implements the Store interface and can therefore
// substitute for any particular store instance.
type Store struct {
	stores        map[string]store
	kindFromQuery KindFromQuery
}

// KindFromQuery is a function that returns the kind of the entity from the
// given query. Given that the datastore.Query interface does not expose the
// kind via a method this function mast assert a particular query type and
// extract the kind from it in some manner. This will probably look like an
// assertion switch if there are multiple query types to be handled.
type KindFromQuery func(datastore.Query) string

// NewStore returns a new CompositeStore with the given stores.
// The stores map should be keyed by the kind of the entity.
func NewStore(stores map[string]store, kindFromQuery KindFromQuery) *Store {
	return &Store{stores, kindFromQuery}
}

// IDKey implements the Store.IDKey by calling IDKey on the appropriate
// store based on the kind.
func (s *Store) IDKey(kind string, id int64) *key {
	return s.getStore(kind).IDKey(kind, id)
}

// NameKey implements the Store.NameKey by calling NameKey on the appropriate
// store based on the kind.
func (s *Store) NameKey(kind, name string) *key {
	return s.getStore(kind).NameKey(kind, name)
}

// IncompleteKey implements the Store.IncompleteKey by calling IncompleteKey
// on the appropriate store based on the kind.
func (s *Store) IncompleteKey(kind string) *key {
	return s.getStore(kind).IncompleteKey(kind)
}

// NewQuery implements the Store.NewQuery by calling NewQuery on the appropriate
// store based on the kind.
func (s *Store) NewQuery(kind string, keysOnly bool, keyParts ...string) datastore.Query {
	return s.getStore(kind).NewQuery(kind, keysOnly, keyParts...)
}

// Get implements the Store.Get by calling Get on the appropriate store based
// on the kind.
func (s *Store) Get(ctx ctx, key *key, dst ety) error {
	return s.getStore(key.Kind).Get(ctx, key, dst)
}

// GetAll implements the Store.GetAll by calling GetAll on the appropriate store.
// We find the appropriate store through trial and error given that the query
// does not contain the kind. We look at possible stores and try to find the matching
// one.
func (s *Store) GetAll(ctx ctx, query datastore.Query, dst interface{}) ([]*key, error) {
	return s.getStore(s.kindFromQuery(query)).GetAll(ctx, query, dst)
}

// Create implements the Store.Create by calling Create on the appropriate store
// based on the kind.
func (s *Store) Create(ctx ctx, key *key, src ety) error {
	return s.getStore(key.Kind).Create(ctx, key, src)
}

// Put implements the Store.Put by calling Put on the appropriate store
// based on the kind.
func (s *Store) Put(ctx ctx, key *key, src ety) (*key, error) {
	return s.getStore(key.Kind).Put(ctx, key, src)
}

// Update implements the Store.Update by calling Update on the appropriate store
// based on the kind.
func (s *Store) Update(ctx ctx, key *key, fn func(ety), dst ety) error {
	return s.getStore(key.Kind).Update(ctx, key, fn, dst)
}

// DeleteMulti implements the Store.DeleteMulti by calling DeleteMulti on the
// appropriate store based on the kind.
func (s *Store) DeleteMulti(ctx ctx, keys []*key) error {
	return s.getStore(keys[0].Kind).DeleteMulti(ctx, keys)
}

// Delete implements the Store.Delete by calling Delete on the appropriate store
// based on the kind.
func (s *Store) Delete(ctx ctx, key *key) error {
	return s.getStore(key.Kind).Delete(ctx, key)
}

func (s *Store) getStore(kind string) store {
	store, ok := s.stores[kind]
	if !ok {
		panic(fmt.Sprintf("store not found for kind: %q, ensure this kind is mapped to a store", kind))
	}
	if store == nil {
		panic(fmt.Sprintf("store for kind: %q is nil", kind))
	}
	return store
}
