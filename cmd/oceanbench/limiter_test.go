/*
DESCRIPTION
	limiter_test.go provides testing for the OceanTokenBucketLimiter.

AUTHORS
	Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
	Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

	This file is part of Ocean Bench. Ocean Bench is free software: you can
	redistribute it and/or modify it under the terms of the GNU
	General Public License as published by the Free Software
	Foundation, either version 3 of the License, or (at your option)
	any later version.

	Ocean Bench is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	in gpl.txt.  If not, see
	<http://www.gnu.org/licenses/>.
*/

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"bitbucket.org/ausocean/iotsvc/iotds"
	"golang.org/x/net/context"
)

type mockStore struct {
	id  string
	dir string
}

func newMockStore(ctx context.Context, id, dir string) (*mockStore, error) {
	if dir == "" {
		dir = "."
	}
	store := mockStore{id: id, dir: dir}
	dir = filepath.Join(dir, id)
	err := os.MkdirAll(dir, 0766)
	if err != nil {
		return &store, err
	}
	return &store, nil
}

func (s *mockStore) NameKey(kind, name string) *Key {
	dir := filepath.Join(s.dir, s.id, kind)
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		os.MkdirAll(dir, 0766)
	}
	return &Key{Kind: kind, Name: name}
}

func (s *mockStore) Get(ctx Ctx, key *Key, dst Ety) error {
	bytes, err := os.ReadFile(filepath.Join(s.dir, s.id, key.Kind, key.Name))
	if err != nil {
		if os.IsNotExist(err) {
			return iotds.ErrNoSuchEntity
		}
		return err
	}
	dst.Decode(bytes)
	return nil
}

func (s *mockStore) Put(ctx Ctx, key *Key, src Ety) (*Key, error) {
	bytes := src.Encode()
	err := os.WriteFile(filepath.Join(s.dir, s.id, key.Kind, key.Name), bytes, 0777)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (s *mockStore) CleanUp() error {
	return os.RemoveAll(filepath.Join(s.dir, s.id))
}

func (s *mockStore) IDKey(kind string, id int64) *Key                                    { return nil }
func (s *mockStore) IncompleteKey(kind string) *Key                                      { return nil }
func (s *mockStore) DeleteMulti(ctx Ctx, keys []*Key) error                              { return nil }
func (s *mockStore) NewQuery(kind string, keysOnly bool, keyParts ...string) iotds.Query { return nil }
func (s *mockStore) GetAll(ctx Ctx, q iotds.Query, dst interface{}) ([]*Key, error)      { return nil, nil }
func (s *mockStore) Create(ctx Ctx, key *Key, src Ety) error                             { return nil }
func (s *mockStore) Update(ctx Ctx, key *Key, fn func(Ety), dst Ety) error               { return nil }
func (s *mockStore) Delete(ctx Ctx, key *Key) error                                      { return nil }

func assertEqual[T comparable](t *testing.T, got, want T, msg string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %v, want %v", msg, got, want)
	}
}

func TestTokenBucketLimiter(t *testing.T) {
	const (
		maxTokens  = 10
		refillRate = 1 // 1 per hour
		id         = "test_bucket"
		storeID    = "test"
		storeDir   = ""
	)

	assertTimesEqual := func(got, want time.Time, msg string) {
		t.Helper()
		if !got.Equal(want) {
			t.Errorf("%s: got %v, want %v", msg, got, want)
		}
	}

	store, err := newMockStore(context.Background(), storeID, storeDir)
	if err != nil {
		t.Fatalf("newMockStore() error: %v", err)
	}
	defer store.CleanUp()

	bucket, err := GetOceanTokenBucketLimiter(maxTokens, refillRate, id, store)
	if err != nil {
		t.Fatalf("GetOceanTokenBucketLimiter() error: %v", err)
	}

	// Make sure everything has been initialised correctly.
	assertEqual(t, bucket.Tokens, maxTokens, "Tokens initial")
	assertEqual(t, bucket.MaxTokens, maxTokens, "MaxTokens")
	assertEqual(t, bucket.RefillRate, refillRate, "RefillRate")
	assertEqual(t, bucket.ID, id, "ID")
	assertEqual(t, bucket.RequestOK(), true, "RequestOK()")
	assertEqual(t, bucket.Tokens, maxTokens-1, "Tokens after request")

	// Let's see if everything is stored correctly after first request.
	// We'll need to get the last refill time.
	lastRefillTime := bucket.LastRefillTime
	bucket2, err := GetOceanTokenBucketLimiter(maxTokens, refillRate, id, store)
	if err != nil {
		t.Fatalf("GetOceanTokenBucketLimiter() error: %v", err)
	}
	assertEqual(t, bucket2.Tokens, maxTokens-1, "Tokens bucket2")
	assertEqual(t, bucket2.MaxTokens, maxTokens, "MaxTokens bucket2")
	assertEqual(t, bucket2.RefillRate, refillRate, "RefillRate bucket2")
	assertEqual(t, bucket2.ID, id, "ID bucket2")
	assertTimesEqual(bucket2.LastRefillTime, lastRefillTime, "LastRefillTime bucket2")

	// Let's try abusing it until we run out of tokens.
	for i := 0; i < maxTokens-1; i++ {
		assertEqual(t, bucket.RequestOK(), true, "RequestOK()")
	}
	assertEqual(t, bucket.RequestOK(), false, "RequestOK()")

	// Let's subtract time by 2 hour so that it gets refilled by 2 tokens to test refill,
	// and then use tokens until it runs out.
	bucket.LastRefillTime = bucket.LastRefillTime.Add(-2 * time.Hour)
	assertEqual(t, bucket.RequestOK(), true, "RequestOK()")
	assertEqual(t, bucket.RequestOK(), true, "RequestOK()")
	assertEqual(t, bucket.RequestOK(), false, "RequestOK()")
}
