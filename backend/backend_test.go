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
	"encoding/gob"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

// Random ID used as the sessionID.
var sessionID = uuid.NewString()

// testService contains commonly used fields across handlers.
type testService struct {
	t        *testing.T
	netStore *sessions.CookieStore
}

// Oauth token used for testing.
var testTok = &oauth2.Token{
	AccessToken:  "example_access_token_12345",
	TokenType:    "Bearer",
	RefreshToken: "example_refresh_token_67890",
	Expiry:       time.Now().AddDate(0, 0, 7),
}

// TestFiberHandler tests the implementation of the FiberHandler using FiberSessions.
func TestFiberHandler(t *testing.T) {
	svc := &testService{t, nil}

	app := fiber.New()

	app.Get("/set", svc.fiberSetHandler)
	app.Get("/get", svc.fiberGetHandler)

	req1 := httptest.NewRequest(http.MethodGet, "/set", nil)
	resp1, err := app.Test(req1, -1)
	assert.Nil(t, err)
	assert.Len(t, resp1.Cookies(), 1, "expected 1 cookie to be set, got: %d", len(resp1.Cookies()))

	ck := resp1.Cookies()[0]

	req2 := httptest.NewRequest(http.MethodGet, "/get", nil)
	req2.AddCookie(ck)
	resp2, err := app.Test(req2, -1)
	assert.Nil(t, err)
	assert.Equal(t, fiber.StatusOK, resp2.StatusCode)
}

func (svc *testService) fiberSetHandler(c *fiber.Ctx) error {
	return svc.set(NewFiberHandler(c))
}

func (svc *testService) fiberGetHandler(c *fiber.Ctx) error {
	return svc.get(NewFiberHandler(c))
}

// TestNetHandler tests the NetHandler implementation using GorillaSessions.
func TestNetHandler(t *testing.T) {
	store := sessions.NewCookieStore(securecookie.GenerateRandomKey(64))
	gob.Register(&oauth2.Token{})

	svc := &testService{t, store}

	req1 := httptest.NewRequest(http.MethodGet, "/set", nil)
	w1 := httptest.NewRecorder()
	svc.netSetHandler(w1, req1)
	resp1 := w1.Result()
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	cookies := resp1.Cookies()
	assert.Equal(t, 1, len(cookies))

	ck := cookies[0]

	req2 := httptest.NewRequest(http.MethodGet, "/get", nil)
	req2.AddCookie(ck)
	w2 := httptest.NewRecorder()
	svc.netGetHandler(w2, req2)
	resp2 := w2.Result()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

func (svc *testService) netSetHandler(w http.ResponseWriter, r *http.Request) {
	err := svc.set(NewNetHandler(w, r, svc.netStore))
	if err != nil {
		svc.t.Errorf("unable to handle set: %v", err)
	}
}

func (svc *testService) netGetHandler(w http.ResponseWriter, r *http.Request) {
	err := svc.get(NewNetHandler(w, r, svc.netStore))
	if err != nil {
		svc.t.Errorf("unable to handle get: %v", err)
	}
}

func (svc *testService) set(h Handler) error {
	sess, err := h.LoadSession(sessionID)
	if err != nil {
		return fmt.Errorf("error getting session: %w", err)
	}

	err = sess.Set("oauth2_token", testTok)
	if err != nil {
		return fmt.Errorf("unable to set seesion value: %w", err)
	}

	return h.SaveSession(sess)
}

func (svc *testService) get(h Handler) error {
	sess, err := h.LoadSession(sessionID)
	if err != nil {
		svc.t.Errorf("unable to get Session with id %s: %v", sessionID, err)
		return fmt.Errorf("unable to get Session with id %s: %w", sessionID, err)
	}

	tok := &oauth2.Token{}
	err = sess.Get("oauth2_token", &tok)
	if err != nil {
		svc.t.Errorf("error getting session value: %v", err)
		return fmt.Errorf("error getting session value: %w", err)
	}
	assert.Equal(svc.t, testTok, tok)

	return nil
}
