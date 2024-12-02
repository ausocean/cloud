package backend

import (
	"fmt"
	"reflect"

	"github.com/gofiber/fiber/v2"
)

// SessionStore defines an interface which manages the handling of Session management.
type SessionStore interface {
	// Get returns a Session based on the given id.
	Get(string, ...SessionStoreOption) (Session, error)

	// Save saves the passed Session to the SessionStore storage type.
	Save(Session, ...SessionStoreOption) error
}

// SessionStoreOption is a function option type which can be passed to the methods of
// SessionStore to extend their functionality.
type SessionStoreOption func(SessionStore) error

// FiberSessionStore implements the SessionStore interface using the fiber http framework.
//
// NOTE: most methods implemented for the FiberSessionStore require the WithFiberCtx functional option
// to be passed, to allow the method to access the request ctx.
type FiberSessionStore struct {
	ctx *fiber.Ctx // Used to access the request, passed in using the WithFiberCtx functional option.
}

// WithFiberCtx is a SessionStoreOption which allows FiberSessionStore to access the request ctx by attaching
// the passed ctx to the FiberSessionStore fields.
func WithFiberCtx(c *fiber.Ctx) SessionStoreOption {
	return func(s SessionStore) error {
		store, ok := s.(*FiberSessionStore)
		if !ok {
			return fmt.Errorf("incorrect SessionStore type, expected *FiberSessionStore, got %s", reflect.TypeOf(s))
		}

		store.ctx = c
		return nil
	}
}

// NewFiberSessionStore returns an empty FiberSessionStore.
func NewFiberSessionStore() *FiberSessionStore {
	return &FiberSessionStore{}
}

// Get implements the SessionStore interface for the FiberSessionStore type.
//
// NOTE: The WithFiberCtx option must be used to provide the fiber context
// for the call to Get.
func (s *FiberSessionStore) Get(id string, opts ...SessionStoreOption) (Session, error) {
	for _, opt := range opts {
		opt(s)
	}

	// Check that the fiber.ctx exists.
	if s.ctx == nil {
		return nil, fmt.Errorf("cannot get session with nil fiber context")
	}

	return NewFiberSession(id, s.ctx.Cookies(id)), nil
}

// Save implements the SessionStore interface for the FiberSessionStore type.
//
// NOTE: The WithFiberCtx option must be used to provide the fiber context for the
// call to Save.
func (s *FiberSessionStore) Save(session Session, opts ...SessionStoreOption) error {
	// Check that the session is a fiber session.
	fs, ok := session.(*FiberSession)
	if !ok {
		return fmt.Errorf("incompatible session type, wanted FiberSession, got %v", reflect.TypeOf(fs))
	}

	for _, opt := range opts {
		opt(s)
	}

	// Check that the fiber.ctx exists.
	if s.ctx == nil {
		return fmt.Errorf("cannot save session with nil fiber context")
	}

	// Get the cookie from the FiberSession.
	s.ctx.Cookie(fs.getCookie())

	return nil
}
