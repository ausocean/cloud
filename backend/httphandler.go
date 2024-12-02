package backend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/gofiber/fiber/v2"
)

const (
	Fiber = iota
	NetHTTP
)

// Error types.
var ErrInvalidHandlerType = errors.New("invalid handler type")

// HTTPHandlerOption is a functional option which can be used to pass additional values into
// a new HTTPHandler.
type HTTPHandlerOption func(HTTPHandler) error

// WithHTTPWriterAndRequest is a HTTPHandlerOption which is used to attach a http.ResponseWriter and Request
// to a NetHTTPHandler.
func WithHTTPWriterAndRequest(w http.ResponseWriter, r *http.Request) HTTPHandlerOption {
	return func(h HTTPHandler) error {
		nh, ok := h.(*NetHTTPHandler)
		if !ok {
			return fmt.Errorf("expected NetHTTPHandler type, got %v", reflect.TypeOf(h))
		}

		nh.w = w
		nh.r = r
		return nil
	}
}

// WithFiberHandlerCtx is a SessionStoreOption which allows FiberHandler to access the request ctx by attaching
// the passed ctx to the FiberHandler fields.
func WithFiberHandlerCtx(c *fiber.Ctx) HTTPHandlerOption {
	return func(h HTTPHandler) error {
		fh, ok := h.(*FiberHandler)
		if !ok {
			return fmt.Errorf("incorrect SessionStore type, expected *FiberSessionStore, got %s", reflect.TypeOf(h))
		}

		fh.ctx = c
		return nil
	}
}

// NetHTTPHandler creates a new HTTPHandler of the specified type, applying the passed options
// to the newly created handler.
func NewHTTPHandler(handlerType int, opts ...HTTPHandlerOption) (HTTPHandler, error) {
	switch handlerType {
	case Fiber:
		return NewFiberHandler(opts...)
	case NetHTTP:
		return NewNetHTTPHandler(opts...)
	default:
		return nil, fmt.Errorf("unable to create newHTTPHandler: %w", ErrInvalidHandlerType)
	}
}

// HTTPHandler is an interface used to abstract the functionality of different HTTP frameworks.
type HTTPHandler interface {
	// NewSessionStore creates a new SessionStore of the correct type for the HTTPHandler.
	// An optional secret key can be passed, depending on the underlying implementation, this may
	// be used to encrypt the sessions.
	//
	// eg: A NetHTTPHandler will create a new GorillaSessionStore with the passed secret key.
	NewSessionStore(key ...string) (SessionStore, error)

	// FormValue returns the value for the given field in a http form if it exists.
	FormValue(string) string

	// Redirect creates a redirect to the specified location, with the given status code.
	Redirect(string, int) error

	// SessionStoreOptions returns a slice of SessionStoreOptions which will need to be passed
	// into many of the SessionStore requests.
	SessionStoreOptions() []SessionStoreOption

	// Context returns a context value which implements the context.Context interface.
	Context() context.Context
}

// FiberHandler is a fiber based implementation of the HTTPHandler interface.
//
// NOTE: FiberHandler uses FiberSession and FiberSessionStore types.
type FiberHandler struct {
	ctx *fiber.Ctx
}

// NewFiberHandler creates a new FiberHandler with the given options.
func NewFiberHandler(opts ...HTTPHandlerOption) (HTTPHandler, error) {
	h := &FiberHandler{}

	// Apply options.
	for i, opt := range opts {
		err := opt(h)
		if err != nil {
			return nil, fmt.Errorf("error applying option %d: %w", i, err)
		}
	}

	return h, nil
}

// NewSessionStore implements the NewSessionStore method of the HTTPHandler interface
// for the FiberHandler by creating a creating a new FiberSessionStore.
func (h *FiberHandler) NewSessionStore(key ...string) (SessionStore, error) {
	return NewFiberSessionStore(), nil
}

// Redirect implements the HTTPHandler Redirect method by calling the Redirect method
// of the attached *fiber.Ctx.
func (h *FiberHandler) Redirect(location string, status int) error {
	return h.ctx.Redirect(location, status)
}

// FormValue implements the HTTPHandler FormValue method by calling the FormValue method
// of the attached *fiber.Ctx.
func (h *FiberHandler) FormValue(key string) string {
	return h.ctx.FormValue(key)
}

// SessionStoreOptions implements the HTTPHandler SessionStoreOptions method by
// returning a WithFiberCtx() option to be used with many calls to FiberSessionStore
// methods.
func (h *FiberHandler) SessionStoreOptions() []SessionStoreOption {
	return []SessionStoreOption{WithFiberCtx(h.ctx)}
}

// Context implements the HTTPHandler Context method by calling the *fiber.Ctx.Context
// method.
func (h *FiberHandler) Context() context.Context {
	return h.ctx.Context()
}

// NetHTTPHandler is a net/http based implementation of the HTTPHandler interface.
//
// NOTE: NetHTTPHandler uses GorillaSession and GorillaSessionStore types.
type NetHTTPHandler struct {
	w http.ResponseWriter
	r *http.Request
}

// NewNetHTTPHandler creates a new NetHTTPHandler with the passed options.
func NewNetHTTPHandler(opts ...HTTPHandlerOption) (HTTPHandler, error) {
	h := &NetHTTPHandler{}

	// Apply options.
	for i, opt := range opts {
		err := opt(h)
		if err != nil {
			return nil, fmt.Errorf("error applying option %d: %w", i, err)
		}
	}

	return h, nil
}

// NewSessionStore implements the NewSessionStore method of the HTTPHandler interface
// for the NetHTTPHandler by creating a creating a new GorillaSessionStore.
func (h *NetHTTPHandler) NewSessionStore(key ...string) (SessionStore, error) {
	if len(key) != 1 || key[0] == "" {
		return nil, fmt.Errorf("cannot create new GorillaSessionStore without session key")
	}

	return NewGorillaSessionStore(key[0]), nil
}

// Redirect implements the HTTPHandler Redirect method by calling the Redirect method
// of the attached *http.Request.
func (h *NetHTTPHandler) Redirect(location string, status int) error {
	http.Redirect(h.w, h.r, location, status)
	return nil
}

// FormValue implements the HTTPHandler FormValue method by calling the FormValue method
// of the attached *http.Request
func (h *NetHTTPHandler) FormValue(key string) string {
	return h.r.FormValue(key)
}

// SessionStoreOptions implements the HTTPHandler SessionStoreOptions method by
// returning a WithNetHttpHandler() option to be used with many calls to GorillaSessionStore
// methods.
func (h *NetHTTPHandler) SessionStoreOptions() []SessionStoreOption {
	return []SessionStoreOption{WithNetHttpHandler(h.w, h.r)}
}

// Context implements the HTTPHandler Context method by calling the *http.Request.Context
// method.
func (h *NetHTTPHandler) Context() context.Context {
	return h.r.Context()
}
