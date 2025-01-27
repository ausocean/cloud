package main

import (
	"context"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/ausocean/openfish/datastore"
)

// ausOceanCompositeStore returns a composite store that delegates to the
// appropriate store based on the kind of the entity. This tries to fix the
// awkwardness of selecting the right store based on the kind of the entity
// you're dealing with.
func ausOceanCompositeStore(settingsStore, mediaStore datastore.Store) *CompositeStore {
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

	return NewCompositeStore(
		map[string]datastore.Store{
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
		},
		getKindFromQuery,
	)
}

// CompositeStore is a datastore "facade" that delegates to the appropriate
// store based on the kind of the entity. This is useful when you have multiple
// stores that you want to treat as a single store.
// CompositeStore implements the datastore.Store interface and can therefore
// substitute for any particular store instance.
type CompositeStore struct {
	stores        map[string]datastore.Store
	kindFromQuery KindFromQuery
}

// KindFromQuery is a function that returns the kind of the entity from the
// given query. Given that the datastore.Query interface does not expose the
// kind via a method this function mast assert a particular query type and
// extract the kind from it in some manner. This will probably look like an
// assertion switch if there are multiple query types to be handled.
type KindFromQuery func(datastore.Query) string

// NewCompositeStore returns a new CompositeStore with the given stores.
// The stores map should be keyed by the kind of the entity.
func NewCompositeStore(stores map[string]datastore.Store, kindFromQuery KindFromQuery) *CompositeStore {
	return &CompositeStore{stores, kindFromQuery}
}

// IDKey implements the Store.IDKey by calling IDKey on the appropriate
// store based on the kind.
func (s *CompositeStore) IDKey(kind string, id int64) *Key {
	return s.stores[kind].IDKey(kind, id)
}

// NameKey implements the Store.NameKey by calling NameKey on the appropriate
// store based on the kind.
func (s *CompositeStore) NameKey(kind, name string) *Key {
	return s.stores[kind].NameKey(kind, name)
}

// IncompleteKey implements the Store.IncompleteKey by calling IncompleteKey
// on the appropriate store based on the kind.
func (s *CompositeStore) IncompleteKey(kind string) *Key {
	return s.stores[kind].IncompleteKey(kind)
}

// NewQuery implements the Store.NewQuery by calling NewQuery on the appropriate
// store based on the kind.
func (s *CompositeStore) NewQuery(kind string, keysOnly bool, keyParts ...string) datastore.Query {
	return s.stores[kind].NewQuery(kind, keysOnly, keyParts...)
}

// Get implements the Store.Get by calling Get on the appropriate store based
// on the kind.
func (s *CompositeStore) Get(ctx context.Context, key *Key, dst datastore.Entity) error {
	return s.stores[key.Kind].Get(ctx, key, dst)
}

// GetAll implements the Store.GetAll by calling GetAll on the appropriate store.
// We find the appropriate store through trial and error given that the query
// does not contain the kind. We look at possible stores and try to find the matching
// one.
func (s *CompositeStore) GetAll(ctx context.Context, query datastore.Query, dst interface{}) ([]*Key, error) {
	return s.stores[s.kindFromQuery(query)].GetAll(ctx, query, dst)
}

// Create implements the Store.Create by calling Create on the appropriate store
// based on the kind.
func (s *CompositeStore) Create(ctx context.Context, key *Key, src datastore.Entity) error {
	return s.stores[key.Kind].Create(ctx, key, src)
}

// Put implements the Store.Put by calling Put on the appropriate store
// based on the kind.
func (s *CompositeStore) Put(ctx context.Context, key *Key, src datastore.Entity) (*Key, error) {
	return s.stores[key.Kind].Put(ctx, key, src)
}

// Update implements the Store.Update by calling Update on the appropriate store
// based on the kind.
func (s *CompositeStore) Update(ctx context.Context, key *Key, fn func(datastore.Entity), dst datastore.Entity) error {
	return s.stores[key.Kind].Update(ctx, key, fn, dst)
}

// DeleteMulti implements the Store.DeleteMulti by calling DeleteMulti on the
// appropriate store based on the kind.
func (s *CompositeStore) DeleteMulti(ctx context.Context, keys []*Key) error {
	return s.stores[keys[0].Kind].DeleteMulti(ctx, keys)
}

// Delete implements the Store.Delete by calling Delete on the appropriate store
// based on the kind.
func (s *CompositeStore) Delete(ctx context.Context, key *Key) error {
	return s.stores[key.Kind].Delete(ctx, key)
}
