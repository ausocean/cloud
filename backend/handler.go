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

// Package backend provides a unified interface for handling HTTP requests and managing user sessions.
//
// The `Handler` interface abstracts common HTTP operations, such as accessing form values,
// redirecting requests, and retrieving context. Concrete implementations are provided for
// Fiber and net/http frameworks.
//
// The `Session` interface defines a standardized way to manage user sessions, including setting
// expiration times, storing and retrieving data, and invalidating sessions.
// Implementations for Fiber-based and Gorilla sessions are included.
package backend

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/sessions"
)

// Handler is an interface used to abstract the functionality of different HTTP frameworks.
type Handler interface {
	// FormValue returns the value for the given field in a http form if it exists.
	FormValue(string) string

	// Redirect creates a redirect to the specified location, with the given status code.
	Redirect(string, int) error

	// Context returns a context value which implements the context.Context interface.
	Context() context.Context

	// LoadSession returns a Session based on the given id.
	LoadSession(string) (Session, error)

	// Save saves the passed Session to the session store.
	SaveSession(Session) error
}

// FiberHandler is a fiber based implementation of the Handler interface.
//
// NOTE: FiberHandler uses FiberSessions and stores them in client side cookies
// which should be encrypted.
type FiberHandler struct {
	Ctx *fiber.Ctx
}

// NewFiberHandler creates a new FiberHandler.
func NewFiberHandler(c *fiber.Ctx) Handler {
	return &FiberHandler{c}
}

// FormValue implements the Handler FormValue method by calling the FormValue method
// of the attached *fiber.Ctx.
func (h *FiberHandler) FormValue(key string) string {
	return h.Ctx.FormValue(key)
}

// Redirect implements the Handler Redirect method by calling the Redirect method
// of the attached *fiber.Ctx.
func (h *FiberHandler) Redirect(location string, status int) error {
	return h.Ctx.Redirect(location, status)
}

// Context implements the Handler Context method by calling the *fiber.Ctx.Context
// method.
func (h *FiberHandler) Context() context.Context {
	return h.Ctx.Context()
}

// Load implements the SessionStore interface for the FiberSessionStore type.
func (h *FiberHandler) LoadSession(id string) (Session, error) {
	return NewFiberSession(id, h.Ctx.Cookies(id))
}

// Save implements the SessionStore interface for the FiberSessionStore type.
func (h *FiberHandler) SaveSession(session Session) error {
	// Check that the session is a fiber session.
	fs, ok := session.(*FiberSession)
	if !ok {
		return fmt.Errorf("incompatible session type, wanted FiberSession, got %v", reflect.TypeOf(fs))
	}

	// Get the cookie from the FiberSession.
	h.Ctx.Cookie(fs.cookie)

	return nil
}

// NetHandler is a net/http based implementation of the Handler interface.
//
// NOTE: NetHandler uses GorillaSessions.
type NetHandler struct {
	w     http.ResponseWriter
	r     *http.Request
	store *sessions.CookieStore
}

// NewNetHandler creates a new NetHandler.
func NewNetHandler(w http.ResponseWriter, r *http.Request, store *sessions.CookieStore) Handler {
	return &NetHandler{w, r, store}
}

// Redirect implements the Handler Redirect method by calling the Redirect method
// of the attached *http.Request.
func (h *NetHandler) Redirect(location string, status int) error {
	http.Redirect(h.w, h.r, location, status)
	return nil
}

// FormValue implements the Handler FormValue method by calling the FormValue method
// of the attached *http.Request
func (h *NetHandler) FormValue(key string) string {
	return h.r.FormValue(key)
}

// Context implements the Handler Context method by calling the *http.Request.Context
// method.
func (h *NetHandler) Context() context.Context {
	return h.r.Context()
}

// Get implements the SessionStore interface for the GorillaSessionStore type.
func (h *NetHandler) LoadSession(id string) (Session, error) {
	sess, err := h.store.Get(h.r, id)
	if err != nil {
		return nil, fmt.Errorf("unable to get session with ID: %s: %w", id, err)
	}

	return NewGorillaSession(sess), nil
}

// Save implements the Save method of the SessionStore interface using GorillaSessions.
func (h *NetHandler) SaveSession(session Session) error {
	// Check that the session is a gorilla session.
	gs, ok := session.(*GorillaSession)
	if !ok {
		return fmt.Errorf("incompatible session type, wanted GorillaSession, got %v", reflect.TypeOf(gs))
	}

	return h.store.Save(h.r, h.w, gs.session)
}
