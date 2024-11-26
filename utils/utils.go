package utils

import (
	"fmt"
	"net/http"
	runtime "runtime/debug"

	"golang.org/x/exp/rand"
)

// TokenURIFromAccount forms a Google Cloud Storage URI for a YouTube token
// based on the provided account. If the account is empty, it's assumed that
// the legacy token is being used. Otherwise, the account is used to form the
// URI. This means we can have tokens stored for different YouTube accounts.
// The URI is of the form: gs://ausocean/<account>.youtube.token.json
// e.g. gs://ausocean/social@ausocean.org.youtube.token.json
func TokenURIFromAccount(account string) string {
	const (
		bucket          = "gs://ausocean/"
		legacyTokenName = "youtube-api-credentials.json"
		defaultTokenURI = bucket + legacyTokenName
	)

	if account == "" {
		return defaultTokenURI
	}

	const tokenPostfix = ".youtube.token.json"

	return bucket + account + tokenPostfix
}

// RecoveryHandler is a function that handles a panic. It is called with the
// http.ResponseWriter and the panic value. If the panic is handled, the function
// should return true, otherwise false.
type RecoveryHandler func(w http.ResponseWriter, err any) bool

// HandledConditions specifies which conditions should be considered to
// have handled a panic.
type HandledConditions struct {
	HandledOnLog          bool
	HandledOnNotification bool
	HandledOnHttpError    bool
}

type recoveryConfig struct {
	handlers          []RecoveryHandler
	logOutput         func(v ...any)
	notify            func(msg string) error
	fmtMsg            func(err any) string
	handlingCriteria  func(err any) bool
	handledConditions HandledConditions
	httpError         int
}

type recoveryOption func(*recoveryConfig)

// WithFmtMsg allows the caller to specify a function to format the panic message.
// By default, the panic message is formatted as "panic: <panic>, stack: <stack>".
// This msg is used for logging and notification.
func WithFmtMsg(f func(err any) string) recoveryOption {
	return func(c *recoveryConfig) {
		c.fmtMsg = f
	}
}

// WithHandlers allows the caller to specify one or more handlers to be called
// after logging, notification and returning an HTTP error. These handlers are
// are called in the order they are provided. If a handler returns true, the
// panic is considered handled and no further handlers are called. By default,
// no handlers are called.
func WithHandlers(handlers ...RecoveryHandler) recoveryOption {
	return func(c *recoveryConfig) {
		c.handlers = append(c.handlers, handlers...)
	}
}

// WithNotification allows the caller to specify a function to send a notification
// e.g. email, SMS, etc. to alert of a panic. The function should return an error
// if the notification could not be sent. By default, no notification is sent.
func WithNotification(notify func(msg string) error) recoveryOption {
	return func(c *recoveryConfig) {
		c.notify = notify
	}
}

// WithLogOutput allows the caller to specify a function to output log messages.
// By default, no log output is performed.
func WithLogOutput(l func(v ...any)) recoveryOption {
	return func(c *recoveryConfig) {
		c.logOutput = l
	}
}

// WithHttpError allows the caller to specify an HTTP error code to return
// when a panic occurs. By default, no HTTP error is returned.
func WithHttpError(code int) recoveryOption {
	return func(c *recoveryConfig) {
		c.httpError = code
	}
}

// WithHandledConditions allows the caller to specify which conditions should
// be considered as handled. By default, all conditions are considered handled.
func WithHandledConditions(conditions HandledConditions) recoveryOption {
	return func(c *recoveryConfig) {
		c.handledConditions = conditions
	}
}

// WithHandlingCriteria allow the caller to specify a function that determines
// whether the panic should be handled by this handler. If the function returns
// false, the panic will not be handled by this handler.
// By default, all panics are handled.
func WithHandlingCriteria(criteria func(err any) bool) recoveryOption {
	return func(c *recoveryConfig) {
		c.handlingCriteria = criteria
	}
}

// NewConfigurableRecoveryHandler provides a RecoveryHandler that can be configured
// with various options. It is capable of performing multiple actions when a panic
// occurs, such as logging, sending notifications, returning HTTP errors and calling
// further handlers. This means this factory can be used "nested" if required.
//
// An example with the RecoverableServiceMux to create a handler that has common
// logging, notification and httpError, but then specific handlers for certain panics:
//
// mux := utils.NewRecoverableServeMux(
//
//	utils.NewConfigurableRecoveryHandler(
//		utils.WithHandledConditions(utils.HandledConditions{HandledOnNotification: true}),
//		utils.WithLogOutput(log.Println),
//		utils.WithNotification(func(msg string) error { return sendPanicNotification(publicKey, privateKey, msg) }),
//		utils.WithHttpError(http.StatusInternalServerError),
//		utils.WithHandlers(
//			panicType1Handler,
//			panicType2Handler,
//			panicType3Handler
//		),
//	),
//
// )
//
// or we can have different logging/notification for different panics using a nested
// style of handler construction. The idea is that we may have different panics with
// different levels of severity and we might want to perform different actions based on this.
//
// mux := utils.NewRecoverableServeMux(
//
//	utils.NewConfigurableRecoveryHandler(
//		utils.WithHandlers(
//			utils.NewConfigurableRecoveryHandler(
//				utils.WithHandlingCriteria(func(err any) bool { /* return true for panic 1 */ }),
//				utils.WithLogOutput(panicType1Log),
//			),
//			utils.NewConfigurableRecoveryHandler(
//				utils.WithHandlingCriteria(func(err any) bool { /* return true for panic 2 */ }),
//				utils.WithNotification(panicType2Notifier),
//			),
//			utils.NewConfigurableRecoveryHandler(
//				utils.WithHandlingCriteria(func(err any) bool { /* return true for panic 3 */ }),
//				utils.WithHandlers(panicType3Handler),
//			),
//		),
//	),
//
// )
func NewConfigurableRecoveryHandler(opts ...recoveryOption) RecoveryHandler {
	defaultLog := func(v ...any) {}
	cfg := &recoveryConfig{
		logOutput: defaultLog,
		fmtMsg: func(panicErr any) string {
			return fmt.Sprintf("panic: %v, stack: %v", panicErr, string(runtime.Stack()))
		},
		handlingCriteria: func(err any) bool { return true },
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return func(w http.ResponseWriter, panicErr any) bool {
		if !cfg.handlingCriteria(panicErr) {
			return false
		}

		var handled bool
		panicMsg := cfg.fmtMsg(panicErr)

		cfg.logOutput(panicMsg)
		handled = cfg.handledConditions.HandledOnLog

		if cfg.httpError != 0 {
			handled = cfg.handledConditions.HandledOnHttpError
			http.Error(w, fmt.Sprintf("panic: %v", panicErr), cfg.httpError)
		}

		if cfg.notify != nil {
			handled = cfg.handledConditions.HandledOnNotification
			err := cfg.notify(panicMsg)
			if err != nil {
				if cfg.handledConditions.HandledOnNotification {
					handled = false
				}
				cfg.logOutput(fmt.Sprintf("could not notify of panic: %v", err))
			}
		}

		for _, handle := range cfg.handlers {
			if handle(w, panicErr) {
				handled = true
			}
		}

		if !handled {
			cfg.logOutput(fmt.Sprintf("panic not handled: %v", panicErr))
		}

		return handled
	}
}

// RecoverableServeMux extends the default http.ServeMux and accepts
// a callback function to be called in case of handler panic recovery.
type RecoverableServeMux struct {
	*http.ServeMux
	handle RecoveryHandler
}

// NewRecoverableServeMux creates a new RecoverableServeMux.
// recover is the callback function to be called in case of handler panic recovery.
func NewRecoverableServeMux(handle RecoveryHandler) *RecoverableServeMux {
	return &RecoverableServeMux{http.NewServeMux(), handle}
}

// ServeHTTP applies the recovery middleware and serves the HTTP request.
func (m *RecoverableServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			if !m.handle(w, err) {
				// The panic has not been handled; re-panic and let the
				// default handler take care of it.
				panic(err)
			}
		}
	}()
	m.ServeMux.ServeHTTP(w, r)
}

// Handle registers the handler for the given pattern.
func (m *RecoverableServeMux) Handle(pattern string, handler http.Handler) {
	m.ServeMux.Handle(pattern, handler)
}

// HandleFunc registers the handler function for the given pattern.
func (m *RecoverableServeMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.ServeMux.HandleFunc(pattern, handler)
}

// GenerateInt64ID generates a 10 digit int64 value which can be used as
// an ID in many datastore types.
//
// NOTE: the generated ID can be cast to an int32 if required.
func GenerateInt64ID() int64 {
	// This function generates a random number between 0, and
	// the largest number which can be expressed as a signed int32.
	// Subtracting 1000000000 from the range allows 1000000000 to be
	// added back to the number after generation to ensure that the
	// value is at least 10 digits long.
	return rand.Int63n((1<<31)-1000000000) + 1000000000
}
