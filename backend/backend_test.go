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

var sessionID = uuid.NewString()

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

func TestFiberHandler(t *testing.T) {
	svc := &testService{t, nil}

	// Create a fiber app.
	app := fiber.New()

	// Add endpoints.
	app.Get("/set", svc.fiberSetHandler)
	app.Get("/get", svc.fiberGetHandler)

	// Make a request to /set.
	req1 := httptest.NewRequest(http.MethodGet, "/set", nil)
	resp1, err := app.Test(req1, -1)
	assert.Nil(t, err)
	assert.Len(t, resp1.Cookies(), 1, "expected 1 cookie to be set, got: %d", len(resp1.Cookies()))

	// Get the cookie.
	ck := resp1.Cookies()[0]
	// Make a request to /get.
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

func TestNetHandler(t *testing.T) {
	// Create a new cookie store.
	store := sessions.NewCookieStore(securecookie.GenerateRandomKey(64))
	gob.Register(&oauth2.Token{})

	svc := &testService{t, store}

	// Make a request to /set.
	req1 := httptest.NewRequest(http.MethodGet, "/set", nil)
	w1 := httptest.NewRecorder()
	svc.netSetHandler(w1, req1)
	resp1 := w1.Result()
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	cookies := resp1.Cookies()
	assert.Equal(t, 1, len(cookies))

	// Get the cookie.
	ck := cookies[0]
	// Make a request to /get.
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
	// Get a session.
	sess, err := h.LoadSession(sessionID)
	if err != nil {
		return fmt.Errorf("error getting session: %w", err)
	}

	// Set a session value.
	err = sess.Set("oauth2_token", testTok)
	if err != nil {
		return fmt.Errorf("unable to set seesion value: %w", err)
	}

	// Save the session.
	return h.SaveSession(sess)
}

func (svc *testService) get(h Handler) error {
	// Get the session.
	sess, err := h.LoadSession(sessionID)
	if err != nil {
		svc.t.Errorf("unable to get Session with id %s: %v", sessionID, err)
		return fmt.Errorf("unable to get Session with id %s: %w", sessionID, err)
	}

	// Get the value from the session.
	tok := &oauth2.Token{}
	err = sess.Get("oauth2_token", &tok)
	if err != nil {
		svc.t.Errorf("error getting session value: %v", err)
		return fmt.Errorf("error getting session value: %w", err)
	}
	assert.Equal(svc.t, testTok, tok)

	return nil
}