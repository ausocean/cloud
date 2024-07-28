package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecoveryCallbackServeMux(t *testing.T) {
	var recoveredErr any

	recoveryCallback := func(w http.ResponseWriter, err any) {
		recoveredErr = err
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}

	mux := NewRecoveryCallbackServeMux(recoveryCallback)

	// Add a handler that will panic
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Create a test server
	server := httptest.NewServer(mux)
	defer server.Close()

	// Send a request to the /panic endpoint
	resp, err := http.Get(fmt.Sprintf("%s/panic", server.URL))
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check we got internal server error, recover was called, and the error was as expected.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status code 500, got %d", resp.StatusCode)
	}
	if recoveredErr == nil {
		t.Fatal("expected panic to be recovered, but it was not")
	}
	if recoveredErr != "test panic" {
		t.Fatalf("expected recovered error to be 'test panic', got %v", recoveredErr)
	}
}
