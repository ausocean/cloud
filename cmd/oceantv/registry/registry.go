/*
AUTHORS
	Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
	Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

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

// Package registry provides a thread-safe registry for storing and retrieving
// objects that implement the Nameable interface. It ensures that each object
// is registered only once and provides error handling for duplicate registrations.
package registry

import (
	"fmt"
	"sync"
)

// Nameable is an interface that provides a method to return the name of a type.
type Nameable interface {
	Name() string
}

// SafeMap is a thread-safe map that stores any type of value.
type SafeMap struct {
	mu sync.RWMutex
	m  map[string]any
}

// NewSafeMap creates a new SafeMap.
func NewSafeMap() *SafeMap {
	return &SafeMap{
		m: make(map[string]any),
	}
}

// Get retrieves a value from the map.
func (s *SafeMap) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.m[key]
	return val, ok
}

// Set stores a value in the map.
func (s *SafeMap) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

type registry struct {
	m *SafeMap
}

var (
	instantiated *registry
	once         sync.Once
)

// ErrObjectAlreadyRegistered is an error that is returned when an object is
// registered more than once.
type ErrObjectAlreadyRegistered struct{ Name string }

// Error returns the error message.
func (e ErrObjectAlreadyRegistered) Error() string {
	return fmt.Sprintf("object %s already registered", e.Name)
}

// Is returns true if the error is of type ErrObjectAlreadyRegistered.
func (e ErrObjectAlreadyRegistered) Is(err error) bool {
	_, ok := err.(ErrObjectAlreadyRegistered)
	return ok
}

// Register stores an object in the registry. It returns an error if the object
// is already registered, or if the object does not implement the Nameable
// interface and cannot be registered.
func Register(a any) error {
	r := get()
	if n, ok := a.(Nameable); ok {
		if _, ok := Get(n.Name()); ok {
			return ErrObjectAlreadyRegistered{Name: n.Name()}
		}
		r.m.Set(n.Name(), a)
		return nil
	}

	// This is not the best given that you could register
	if n, ok := a.(fmt.Stringer); ok {
		if _, ok := Get(n.String()); ok {
			return ErrObjectAlreadyRegistered{Name: n.String()}
		}
		r.m.Set(n.String(), a)
		return nil
	}
	return fmt.Errorf("object does not implement Nameable")
}

// Get retrieves an object from the registry.
func Get(name string) (any, bool) {
	r := get()
	return r.m.Get(name)
}

func get() *registry {
	once.Do(func() {
		instantiated = &registry{m: NewSafeMap()}
	})
	return instantiated
}
