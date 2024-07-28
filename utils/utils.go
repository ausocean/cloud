package utils

import (
	"net/http"
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

// RecoveryCallbackServeMux extends the default http.ServeMux and accepts
// a callback function to be called in case of handler panic recovery.
type RecoveryCallbackServeMux struct {
	*http.ServeMux
	recover func(w http.ResponseWriter, err any)
}

// NewRecoveryCallbackServeMux creates a new RecoveryCallbackServeMux.
// recover is the callback function to be called in case of handler panic recovery.
func NewRecoveryCallbackServeMux(recover func(w http.ResponseWriter, err any)) *RecoveryCallbackServeMux {
	return &RecoveryCallbackServeMux{http.NewServeMux(), recover}
}

// ServeHTTP applies the recovery middleware and serves the HTTP request.
func (m *RecoveryCallbackServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			m.recover(w, err)
		}
	}()
	m.ServeMux.ServeHTTP(w, r)
}

// Handle registers the handler for the given pattern.
func (m *RecoveryCallbackServeMux) Handle(pattern string, handler http.Handler) {
	m.ServeMux.Handle(pattern, handler)
}

// HandleFunc registers the handler function for the given pattern.
func (m *RecoveryCallbackServeMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.ServeMux.HandleFunc(pattern, handler)
}
