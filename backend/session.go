/*
AUTHORS
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean).

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

package backend

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"

	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/sessions"
)

// Session defines an interface for a session to keep track of user
// authenticated sessions.
type Session interface {
	// SetMaxAge sets the Max Age of the session in seconds, after which the session is
	// no longer valid.
	SetMaxAge(age int) error

	// Set sets a key value store in the session.
	Set(key string, value any) error

	// Get retrieves the value for a given key in the session and stores it in the destination.
	Get(key string, dst any) error

	// Invalidate immediately invalidates the session and marks it as no
	// longer valid.
	Invalidate() error
}

// FiberSession implements the Session interface using a Fiber Cookie based
// storage method.
type FiberSession struct {
	cookie *fiber.Cookie              // Cookie used to store the session.
	values map[string]json.RawMessage // Map of the key value pairs to be encoded into the session.
}

// NewFiberSession creates a new empty FiberSession with the given id, and value.
// NOTE: The passed value should be the stored value of the cookie, which may be empty.
func NewFiberSession(id, value string) (*FiberSession, error) {
	s := &FiberSession{cookie: &fiber.Cookie{Name: id}, values: make(map[string]json.RawMessage)}

	if value == "" {
		return s, nil
	}

	// Parse the value into the session value map.
	ckValue, err := url.QueryUnescape(value)
	if err != nil {
		return nil, fmt.Errorf("unable to unescape cookie value: %w", err)
	}
	err = json.Unmarshal([]byte(ckValue), &s.values)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal value: %w", err)
	}

	return s, nil
}

// SetMaxAge implements the SetMaxAge method of the Session interface by setting
// the maximum age of the cookie in seconds.
func (s *FiberSession) SetMaxAge(age int) error {
	s.cookie.MaxAge = age
	return nil
}

// Set implements the Set method of the Session interface by encoding a query escaped
// map in JSON format to the cookie value.
func (s *FiberSession) Set(key string, value any) error {
	v, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("unable to marshal value to json: %w:", err)
	}
	s.values[key] = json.RawMessage(v)
	bytes, err := json.Marshal(s.values)
	if err != nil {
		return fmt.Errorf("unable to marshal cookie value: %w", err)
	}

	s.cookie.Value = url.QueryEscape(string(bytes))
	return nil
}

// Get implements the Get method of the Session interface by getting the for the given key
// of a key value pair stored in the session.
func (s *FiberSession) Get(key string, dst any) error {
	return json.Unmarshal(s.values[key], dst)
}

// Invalidate implements the Invalidate method of the Session interface by setting
// the Max Age of the cookie to -1.
func (s *FiberSession) Invalidate() error {
	s.cookie.MaxAge = -1
	return nil
}

// GorillaSession implements the Session interface using Gorilla Sessions.
type GorillaSession struct {
	session *sessions.Session
}

func NewGorillaSession(session *sessions.Session) *GorillaSession {
	return &GorillaSession{session: session}
}

// SetMaxAge implements the SetMaxAge method of the Session interface by setting
// the maximum age of the cookie.
func (s *GorillaSession) SetMaxAge(maxAge int) error {
	s.session.Options.MaxAge = maxAge
	return nil
}

// Set implements the Set method of the Session interface by adding the key, value
// pair to the gorilla session's Values map.
func (s *GorillaSession) Set(key string, value interface{}) error {
	s.session.Values[key] = value
	return nil
}

// Get implements the Get method of the Session interface by getting the for the given key
// of a key value pair stored in the session.
func (s *GorillaSession) Get(key string, dst any) error {
	v := s.session.Values[key]

	if v == nil {
		return fmt.Errorf("session %s has no value for key %s", s.session.ID, key)
	}

	// Use reflection to set the value
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer {
		return fmt.Errorf("dst must be a pointer")
	}

	rv = rv.Elem()
	rv.Set(reflect.ValueOf(v))

	return nil
}

// Invalidate implements the Invalidate method of the Session interface by setting
// the Max Age of the cookie to -1.
func (s *GorillaSession) Invalidate() error {
	s.session.Options.MaxAge = -1
	return nil
}
