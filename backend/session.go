package backend

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/sessions"
)

// Session defines an interface for a session to keep track of user
// authenticated sessions.
type Session interface {
	// SetMaxAge sets the Max Age of the session, after which the session is
	// no longer valid.
	SetMaxAge(age time.Duration) error

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

// NewFiberSession creates a new empty FiberSession with the given id.
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
// the maximum age of the cookie.
func (s *FiberSession) SetMaxAge(age time.Duration) error {
	s.cookie.MaxAge = int(age.Seconds())
	return nil
}

// Set implements the Set method of the Session interface by encoding a query escaped
// map in JSON format to the cookie value.
func (s *FiberSession) Set(key string, value any) error {
	// Convert the value into a json RawMessage.
	v, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("unable to marshal value to json: %w:", err)
	}
	s.values[key] = json.RawMessage(v)
	bytes, err := json.Marshal(s.values)
	s.cookie.Value = url.QueryEscape(string(bytes))
	return err
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

// getCookie is a helper function which returns the fiber Cookie used to store the Fiber Session.
func (s *FiberSession) getCookie() *fiber.Cookie {
	return s.cookie
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
func (s *GorillaSession) SetMaxAge(maxAge time.Duration) error {
	s.session.Options.MaxAge = int(maxAge.Seconds())
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
	dst = s.session.Values[key]
	return nil
}

// Invalidate implements the Invalidate method of the Session interface by setting
// the Max Age of the cookie to -1.
func (s *GorillaSession) Invalidate() error {
	s.session.Options.MaxAge = -1
	return nil
}
