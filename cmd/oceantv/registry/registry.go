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
// values of types via a string key.
// It ensures that each type is registered only once and provides error
// handling for duplicate registrations.
package registry

import (
	"errors"
	"fmt"
	"sync"
)

// Named is an interface that provides a method to return the name of a type.
type Named interface {
	Name() string
}

// Newable is an interface that provides a method to create a fresh value of a type.
type Newable interface {
	New(...interface{}) (any, error)
}

type registry struct {
	m *SafeMap
}

var (
	instantiated *registry
	once         sync.Once
)

// ErrObjectAlreadyRegistered is an error that is returned when a type is
// registered more than once.
type ErrTypeAlreadyRegistered struct{ Name string }

// Error returns the error message.
func (e ErrTypeAlreadyRegistered) Error() string {
	return fmt.Sprintf("type %s already registered", e.Name)
}

// Is returns true if the error is of type ErrTypeAlreadyRegistered.
func (e ErrTypeAlreadyRegistered) Is(err error) bool {
	_, ok := err.(ErrTypeAlreadyRegistered)
	return ok
}

// ErrTypeDoesNotExist is an error that is returned when a type is not found
// in the registry.
type ErrTypeNotRegistered struct{ Name string }

// Error returns the error message.
func (e ErrTypeNotRegistered) Error() string {
	return fmt.Sprintf("type %s is not registered", e.Name)
}

// Is returns true if the error is of type ErrTypeNotRegistered.
func (e ErrTypeNotRegistered) Is(err error) bool {
	_, ok := err.(ErrTypeNotRegistered)
	return ok
}

// Register stores a type in the registry. It returns an error if the type is
// already registered, or if the type does not implement the Named interface
// and cannot be registered.
//
// To register, a value of a type is provided. Implementing the Newable interface
// allows the registry to create and initialize a new value of the type for you
// based on the provided args, otherwise any initialization must be done
// manually.
func Register(a any) error {
	r := get()
	if n, ok := a.(Named); ok {
		_, err := Get(n.Name())
		if err == nil {
			return ErrTypeAlreadyRegistered{Name: n.Name()}
		}
		if !errors.Is(err, ErrTypeNotRegistered{Name: n.Name()}) {
			return fmt.Errorf("error checking for type %s: %w", n.Name(), err)
		}
		r.m.Set(n.Name(), a)
		return nil
	}

	// This is here only for legacy support of things that incorrectly implement
	// Stringer instead of Named.
	if n, ok := a.(fmt.Stringer); ok {
		_, err := Get(n.String())
		if err == nil {
			return ErrTypeAlreadyRegistered{Name: n.String()}
		}
		if !errors.Is(err, ErrTypeNotRegistered{Name: n.String()}) {
			return fmt.Errorf("error checking for type %s: %w", n.String(), err)
		}
		r.m.Set(n.String(), a)
		return nil
	}
	return errors.New("type does not implement Named or Stringer")
}

// Get retrieves value with provide type name from the registry.
func Get(name string, args ...interface{}) (any, error) {
	r := get()
	obj, ok := r.m.Get(name)
	if !ok {
		return nil, ErrTypeNotRegistered{Name: name}
	}
	if n, ok := obj.(Newable); ok {
		var err error
		obj, err = n.New(args...)
		if err != nil {
			return nil, fmt.Errorf("error call New for type %s: %w", name, err)
		}
	}
	return obj, nil
}

func get() *registry {
	once.Do(func() {
		instantiated = &registry{m: NewSafeMap()}
	})
	return instantiated
}
