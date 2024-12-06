package backend

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

var sessionID = uuid.NewString()

type testService struct {
	t *testing.T
}

func TestFiberHandler(t *testing.T) {
	svc := &testService{t}

	// Create a fiber app.
	app := fiber.New()

	// Add endpoints.
	app.Get("/set", svc.setHandler)
	app.Get("/get", svc.getHandler)

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

	t.Log(resp2)

	// Create a new Session.
	sess, _ := NewFiberSession(sessionID, "")

	// Add a key/value pair to the session.
	sess.Set("session_token", &oauth2.Token{})
}

func (svc *testService) setHandler(c *fiber.Ctx) error {
	svc.t.Log("setting")

	// Create a new FiberHandler.
	h := NewFiberHandler(c)

	// Get a session.
	sess, err := h.LoadSession(sessionID)
	if err != nil {
		return fmt.Errorf("error getting session: %w", err)
	}

	// Set a session value.
	err = sess.Set("oauth2_token", &oauth2.Token{})
	if err != nil {
		return fmt.Errorf("unable to set seesion value: %w", err)
	}

	// Save the session.
	return h.SaveSession(sess)
}

func (svc *testService) getHandler(c *fiber.Ctx) error {
	svc.t.Log("getting")

	// Make a new Handler.
	h := NewFiberHandler(c)

	// Get the session.
	sess, err := h.LoadSession(sessionID)
	if err != nil {
		svc.t.Errorf("unable to get Session with id %s: %v", sessionID, err)
		return fmt.Errorf("unable to get Session with id %s: %w", sessionID, err)
	}

	// Get the value from the session.
	tok := &oauth2.Token{}
	err = sess.Get("oauth2_token", tok)
	if err != nil {
		svc.t.Errorf("error getting session value: %v", err)
		return fmt.Errorf("error getting session value: %w", err)
	}

	log.Printf("%+v", tok)

	return nil
}
