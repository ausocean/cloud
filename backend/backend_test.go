package backend_test

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/ausocean/cloud/backend"
	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/securecookie"
	"github.com/stretchr/testify/assert"
)

const (
	testCookieID    = "a9ff1695-60d8-49e2-aa2d-3b4c5200da70"
	testCookieKey   = "cookie-name"
	testCookieValue = "cookie-value"
)

// testService is used to pass global scope variables to handlers.
type testService struct {
	store backend.SessionStore
	t     *testing.T
}

// TestFiberSessionStore tests the interface methods of the FiberSessionStore.
func TestFiberSessionStore(t *testing.T) {
	// Create a new Fiber Session Store.
	store := backend.NewFiberSessionStore()

	// Create a new fiber app.
	app := fiber.New()

	// Create a testService with the new store, and testing type.
	svc := &testService{store: store, t: t}

	// Register the test endpoints.
	app.Get("/set-session", svc.setHandler) // Set session creates a new session.
	app.Get("/get-session", svc.getHandler) // Get session checks the created session.

	// Make a request to create a new session.
	req := httptest.NewRequest(http.MethodGet, "/set-session", nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Get the cookies from the response.
	cookies := resp.Cookies()

	// Check that the cookies are there, and are set correctly.
	assert.Len(t, cookies, 1, "Expected 1 cookie to be set")
	assert.Equal(t, testCookieID, cookies[0].Name)

	// Since the cookie is URL escaped, it must be decoded first.
	v, err := url.QueryUnescape(cookies[0].Value)
	assert.NoError(t, err)

	// Unmarshal the JSON to get the value.
	var actualMap map[string]string
	err = json.Unmarshal([]byte(v), &actualMap)
	assert.NoError(t, err)

	// Compare to the expected cookie values.
	expectedMap := map[string]string{
		testCookieKey: testCookieValue,
	}
	assert.Equal(t, expectedMap, actualMap, "Cookie value does not match")

	// Make a new request to the get-session endpoint.
	req2 := httptest.NewRequest(http.MethodGet, "/get-session", nil)

	// Add the newly obtained cookie.
	req2.AddCookie(cookies[0])
	resp2, err := app.Test(req2)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp2.StatusCode)

}

func (svc *testService) setHandler(c *fiber.Ctx) error {
	sess, err := svc.store.Get(testCookieID, backend.WithFiberCtx(c))
	if err != nil {
		svc.t.Errorf("error getting session: %v", err)
	}

	// Create and Set some values.
	sess.Set(testCookieKey, testCookieValue)
	sess.SetMaxAge(7 * 24 * time.Hour)
	return svc.store.Save(sess, backend.WithFiberCtx(c))
}

func (svc *testService) getHandler(c *fiber.Ctx) error {
	log.Println("get handler:")
	sess, err := svc.store.Get(testCookieID, backend.WithFiberCtx(c))
	if err != nil {
		svc.t.Errorf("error getting session: %v", err)
	}

	// Create a Set some values.
	v, err := sess.Get(testCookieKey)
	assert.NoError(svc.t, err)
	c.Writef("Got Value: %s", v)
	if v != testCookieValue {
		svc.t.Errorf("mismatch in set value and gotten value, got: %s, wanted: %s", v, testCookieValue)
	}

	return nil
}

// TestGorillaSessionStore tests the interface methods of the GorillaSessionStore.
func TestGorillaSessionStore(t *testing.T) {
	const secretKey = "abc123"

	// Create a new Fiber Session Store.
	store := backend.NewGorillaSessionStore(secretKey)

	// Create a testService with the new store, and testing type.
	svc := &testService{store: store, t: t}

	// Make a request to create a new session.
	req := httptest.NewRequest(http.MethodGet, "/set-session", nil)
	w := httptest.NewRecorder()
	svc.setHandlerHttp(w, req)
	resp := w.Result()
	assert.Equal(t, 200, resp.StatusCode)

	// Get the cookies from the response.
	cookies := resp.Cookies()

	// Check that the cookies are there, and are set correctly.
	assert.Len(t, cookies, 1, "Expected 1 cookie to be set")
	assert.Equal(t, testCookieID, cookies[0].Name)

	// Decode the cookie.
	s := securecookie.New([]byte(secretKey), nil)
	value := make(map[any]any)
	err := s.Decode(cookies[0].Name, cookies[0].Value, &value)
	if err != nil {
		t.Errorf("unable to decode cookie: %v", err)
	}

	assert.Equal(t, testCookieValue, value[testCookieKey], "mismatch between set and received session values")

	// Make a new request to the get-session endpoint.
	req2 := httptest.NewRequest(http.MethodGet, "/get-session", nil)

	// Add the newly obtained cookie.
	req2.AddCookie(cookies[0])
	w2 := httptest.NewRecorder()
	svc.setHandlerHttp(w, req)
	resp2 := w2.Result()
	assert.Equal(t, 200, resp2.StatusCode)

}

func (svc *testService) setHandlerHttp(w http.ResponseWriter, r *http.Request) {
	sess, err := svc.store.Get(testCookieID, backend.WithNetHttpHandler(w, r))
	if err != nil {
		svc.t.Errorf("error getting session: %v", err)
	}

	// Create and Set some values.
	sess.Set(testCookieKey, testCookieValue)
	sess.SetMaxAge(7 * 24 * time.Hour)
	err = svc.store.Save(sess, backend.WithNetHttpHandler(w, r))
	if err != nil {
		svc.t.Errorf("error saving session: %v", err)
	}
}

func (svc *testService) getHandlerHttp(w http.ResponseWriter, r *http.Request) {
	log.Println("get handler:")
	sess, err := svc.store.Get(testCookieID, backend.WithNetHttpHandler(w, r))
	if err != nil {
		svc.t.Errorf("error getting session: %v", err)
	}

	// Create a Set some values.
	v, err := sess.Get(testCookieKey)
	assert.NoError(svc.t, err)
	if v != testCookieValue {
		svc.t.Errorf("mismatch in set value and gotten value, got: %s, wanted: %s", v, testCookieValue)
	}
}
