/*
AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  It is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package datastore

import (
	"fmt"
	"sync"
)

// Cache defines the (optional) caching interface used by Entity.
type Cache interface {
	Set(key *Key, src Entity) error // Set adds or updates a value to the cache.
	Get(key *Key, dst Entity) error // Get retrieves a value from the cache, or returns ErrCacheMiss.
	Delete(key *Key)                // Delete removes a value from the cache.
	Reset()                         // Reset resets (clears) the cache.
}

// EntityCache, which implements Cache, represents a cache for holding
// datastore entities indexed by key.
type EntityCache struct {
	data  map[Key]Entity
	mutex sync.RWMutex
}

// ErrCacheMiss is the type of error returned when a key is not found in the cache.
type ErrCacheMiss struct {
	key Key
}

// Error returns an error string for errors of type ErrCacheMiss.
func (e ErrCacheMiss) Error() string {
	return fmt.Sprintf("cache miss for key: %v", e.key)
}

// NewEntityCache returns a new EntityCache.
func NewEntityCache() *EntityCache {
	return &EntityCache{data: make(map[Key]Entity)}
}

// Set adds or updates a value to the cache.
func (c *EntityCache) Set(key *Key, src Entity) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	v, err := src.Copy(nil)
	if err != nil {
		return err
	}
	c.data[*key] = v
	return nil
}

// Get retrieves a value from the cache, or returns ErrcacheMiss.
func (c *EntityCache) Get(key *Key, dst Entity) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	v, ok := c.data[*key]
	if !ok {
		return ErrCacheMiss{*key}
	}
	_, err := v.Copy(dst)
	return err
}

// Delete removes a value from the cache.
func (c *EntityCache) Delete(key *Key) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.data, *key)
}

// Reset resets (clears) the cache.
func (c *EntityCache) Reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.data = map[Key]Entity{}
}

// NilCache returns a nil Cache, which denotes no caching.
func NilCache() Cache {
	return nil
}

// NoCache is a helper struct to reduce boilerplate code when implementing the Entity interface for entities that do not require caching.
type NoCache struct{}

// GetCache returns nil, indicating no caching.
func (NoCache) GetCache() Cache {
	return nil
}
