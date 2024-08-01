package utils

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverableServeMux(t *testing.T) {
	var recoveredErr any

	recoveryCallback := func(w http.ResponseWriter, err any) bool {
		recoveredErr = err
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return true
	}

	mux := NewRecoverableServeMux(recoveryCallback)

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

// TestNewConfigurableRecoveryHandler tests the NewConfigurableRecoveryHandler function
func TestNewConfigurableRecoveryHandler(t *testing.T) {
	var (
		logOutputs   []string
		notifyOutput string
	)

	mockLogOutput := func(v ...any) {
		logOutputs = append(logOutputs, fmt.Sprint(v...))
	}

	mockNotification := func(fail bool) func(msg string) error {
		return func(msg string) error {
			if fail {
				return errors.New("notification failed")
			}
			notifyOutput = msg
			return nil
		}
	}

	mockHandler := func(handle bool) func(w http.ResponseWriter, err any) bool {
		return func(w http.ResponseWriter, err any) bool {
			return handle
		}
	}

	mockCriteria := func(handle bool) func(err any) bool {
		return func(err any) bool {
			return handle
		}
	}

	errCustomPanicError := errors.New("custom panic error")

	tests := []struct {
		name                 string
		options              []recoveryOption
		panicValue           any
		expectHandled        bool
		expectedLogOutput    string
		expectedNotifyOutput string
		expectedStatus       int
	}{
		{
			name: "Test WithFmtMsg with log and notify",
			options: []recoveryOption{
				WithFmtMsg(func(panicErr any) string {
					return fmt.Sprintf("custom panic: %v", panicErr)
				}),
				WithLogOutput(mockLogOutput),
				WithNotification((mockNotification(false))),
			},
			panicValue:           "test panic",
			expectedLogOutput:    "custom panic: test panic",
			expectedNotifyOutput: "custom panic: test panic",
			expectHandled:        false,
		},
		{
			name: "Test default fmtMsg",
			options: []recoveryOption{
				WithLogOutput(mockLogOutput),
			},
			panicValue:        "test panic",
			expectHandled:     false,
			expectedLogOutput: "panic: test panic, stack: ",
		},
		{
			name:          "Default handler, no handling",
			options:       []recoveryOption{},
			panicValue:    "test panic",
			expectHandled: false,
		},
		{
			name: "Handler returns true",
			options: []recoveryOption{
				WithHandlers(mockHandler(true)),
			},
			panicValue:    "test panic",
			expectHandled: true,
		},
		{
			name: "Handler returns false",
			options: []recoveryOption{
				WithHandlers(mockHandler(false)),
			},
			panicValue:    "test panic",
			expectHandled: false,
		},
		{
			name: "Test multiple handlers 1",
			options: []recoveryOption{
				WithHandlers(mockHandler(false), mockHandler(false), mockHandler(true)),
			},
			panicValue:    "test panic",
			expectHandled: true,
		},
		{
			name: "Test multiple handlers 2",
			options: []recoveryOption{
				WithHandlers(mockHandler(false), mockHandler(true), mockHandler(false)),
			},
			panicValue:    "test panic",
			expectHandled: true,
		},
		{
			name: "Notification successful, not handled",
			options: []recoveryOption{
				WithNotification((mockNotification(false))),
			},
			panicValue:           "test panic",
			expectHandled:        false,
			expectedNotifyOutput: "panic: test panic, stack: ",
		},
		{
			name: "Notification successful, handled",
			options: []recoveryOption{
				WithNotification((mockNotification(false))),
				WithHandledConditions(HandledConditions{HandledOnNotification: true}),
			},
			panicValue:           "test panic",
			expectHandled:        true,
			expectedNotifyOutput: "panic: test panic, stack: ",
		},
		{
			name: "Notification fails",
			options: []recoveryOption{
				WithNotification((mockNotification(true))),
				WithLogOutput(mockLogOutput),
				WithHandledConditions(HandledConditions{HandledOnNotification: true}),
			},
			panicValue:        "test panic",
			expectHandled:     false,
			expectedLogOutput: "could not notify of panic: notification failed",
		},
		{
			name: "Log output only, not handled",
			options: []recoveryOption{
				WithLogOutput(mockLogOutput),
			},
			panicValue:        "test panic",
			expectHandled:     false,
			expectedLogOutput: "panic: test panic, stack: ",
		},
		{
			name: "Log output only, handled",
			options: []recoveryOption{
				WithLogOutput(mockLogOutput),
				WithHandledConditions(HandledConditions{HandledOnLog: true}),
			},
			panicValue:        "test panic",
			expectHandled:     true,
			expectedLogOutput: "panic: test panic, stack: ",
		},
		{
			name: "HTTP error returned, not handled",
			options: []recoveryOption{
				WithHttpError(http.StatusInternalServerError),
			},
			panicValue:     "test panic",
			expectHandled:  false,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "HTTP error returned, handled",
			options: []recoveryOption{
				WithHttpError(http.StatusInternalServerError),
				WithHandledConditions(HandledConditions{HandledOnHttpError: true}),
			},
			panicValue:     "test panic",
			expectHandled:  true,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "Basic handling conditions not met",
			options: []recoveryOption{
				WithHandledConditions(HandledConditions{
					HandledOnLog: true,
				}),
			},
			panicValue:    "test panic",
			expectHandled: true,
		},
		{
			name: "Handling conditions not met",
			options: []recoveryOption{
				WithHandledConditions(HandledConditions{
					HandledOnHttpError:    true,
					HandledOnLog:          false,
					HandledOnNotification: true,
				}),
			},
			panicValue:    "test panic",
			expectHandled: false,
		},
		{
			name: "Handling criteria true",
			options: []recoveryOption{
				WithHandlingCriteria(mockCriteria(true)),
				WithLogOutput(mockLogOutput),
				WithHandledConditions(HandledConditions{HandledOnLog: true}),
			},
			panicValue:        "test panic",
			expectHandled:     true,
			expectedLogOutput: "panic: test panic, stack: ",
		},
		{
			name: "Handling criteria false",
			options: []recoveryOption{
				WithHandlingCriteria(mockCriteria(false)),
			},
			panicValue:    "test panic",
			expectHandled: false,
		},
		{
			name: "Specific handling criteria, true",
			options: []recoveryOption{
				WithHandlingCriteria(func(panicErr any) bool {
					err, ok := panicErr.(error)
					if !ok {
						return false
					}
					return errors.Is(err, errCustomPanicError)
				}),
				WithLogOutput(mockLogOutput),
				WithHandledConditions(HandledConditions{HandledOnLog: true}),
			},
			panicValue:        errCustomPanicError,
			expectHandled:     true,
			expectedLogOutput: "panic: custom panic error, stack: ",
		},
		{
			name: "Specific handling criteria, false",
			options: []recoveryOption{
				WithHandlingCriteria(func(panicErr any) bool {
					err, ok := panicErr.(error)
					if !ok {
						return false
					}
					return errors.Is(err, errCustomPanicError)
				}),
				WithLogOutput(mockLogOutput),
				WithHandledConditions(HandledConditions{HandledOnLog: true}),
			},
			panicValue:        "test panic",
			expectHandled:     false,
			expectedLogOutput: "",
		},
		{
			name: "With complex options",
			options: []recoveryOption{
				WithFmtMsg(func(panicErr any) string {
					return fmt.Sprintf("custom panic: %v", panicErr)
				}),
				WithHandlers(mockHandler(false), mockHandler(true), mockHandler(false)),
				WithNotification((mockNotification(false))),
				WithLogOutput(mockLogOutput),
				WithHttpError(http.StatusInternalServerError),
				WithHandledConditions(HandledConditions{HandledOnNotification: true}),
				WithHandlingCriteria(mockCriteria(true)),
			},
			panicValue:           "test panic",
			expectHandled:        true,
			expectedLogOutput:    "custom panic: test panic",
			expectedNotifyOutput: "custom panic: test panic",
			expectedStatus:       http.StatusInternalServerError,
		},
		{
			name: "With nested handler, handle",
			options: []recoveryOption{
				WithHandlers(
					NewConfigurableRecoveryHandler(
						WithHandlingCriteria(func(err any) bool {
							return err == "panic 1"
						}),
						WithFmtMsg(func(panicErr any) string {
							return fmt.Sprintf("handler 1 panic: %v", panicErr)
						}),
						WithLogOutput(mockLogOutput),
						WithHandledConditions(HandledConditions{HandledOnLog: true}),
					),
					NewConfigurableRecoveryHandler(
						WithHandlingCriteria(func(err any) bool {
							return err == "panic 2"
						}),
						WithFmtMsg(func(panicErr any) string {
							return fmt.Sprintf("handler 2 panic: %v", panicErr)
						}),
						WithLogOutput(mockLogOutput),
						WithHandledConditions(HandledConditions{HandledOnLog: true}),
					),
					NewConfigurableRecoveryHandler(
						WithHandlingCriteria(func(err any) bool {
							return err == "panic 3"
						}),
						WithFmtMsg(func(panicErr any) string {
							return fmt.Sprintf("handler 3 panic: %v", panicErr)
						}),
						WithLogOutput(mockLogOutput),
						WithHandledConditions(HandledConditions{HandledOnLog: true}),
					),
				),
			},
			panicValue:        "panic 4",
			expectHandled:     false,
			expectedLogOutput: "",
		},
		{
			name: "With nested handler, handle",
			options: []recoveryOption{
				WithHandlers(
					NewConfigurableRecoveryHandler(
						WithHandlingCriteria(func(err any) bool {
							return err == "panic 1"
						}),
						WithFmtMsg(func(panicErr any) string {
							return fmt.Sprintf("handler 1 panic: %v", panicErr)
						}),
						WithLogOutput(mockLogOutput),
						WithHandledConditions(HandledConditions{HandledOnLog: true}),
					),
					NewConfigurableRecoveryHandler(
						WithHandlingCriteria(func(err any) bool {
							return err == "panic 2"
						}),
						WithFmtMsg(func(panicErr any) string {
							return fmt.Sprintf("handler 2 panic: %v", panicErr)
						}),
						WithLogOutput(mockLogOutput),
						WithHandledConditions(HandledConditions{HandledOnLog: true}),
					),
					NewConfigurableRecoveryHandler(
						WithHandlingCriteria(func(err any) bool {
							return err == "panic 3"
						}),
						WithFmtMsg(func(panicErr any) string {
							return fmt.Sprintf("handler 3 panic: %v", panicErr)
						}),
						WithLogOutput(mockLogOutput),
						WithHandledConditions(HandledConditions{HandledOnLog: true}),
					),
				),
			},
			panicValue:        "panic 2",
			expectHandled:     true,
			expectedLogOutput: "handler 2 panic: panic 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logOutputs = nil
			notifyOutput = ""
			handler := NewConfigurableRecoveryHandler(tt.options...)
			rr := httptest.NewRecorder()

			handled := handler(rr, tt.panicValue)

			if handled != tt.expectHandled {
				t.Errorf("expected handled to be %v, got %v", tt.expectHandled, handled)
			}

			if len(logOutputs) != 0 {
				output := logOutputs[len(logOutputs)-1]

				// Check if more than 1 log output, if so, get the second to last to
				// avoid checking the unhandled panic message.
				if len(logOutputs) > 1 {
					output = logOutputs[len(logOutputs)-2]
				}
				if !strings.Contains(output, tt.expectedLogOutput) {
					t.Errorf("expected log output to contain %q, got %q", tt.expectedLogOutput, output)
				}
			}

			if tt.expectedNotifyOutput != "" {
				if !strings.Contains(notifyOutput, tt.expectedNotifyOutput) {
					t.Errorf("expected notify output to contain %q, got %q", tt.expectedNotifyOutput, notifyOutput)
				}
			}

			if tt.expectedStatus != 0 {
				status := rr.Code
				if status != tt.expectedStatus {
					t.Errorf("expected status to be %d, got %d", tt.expectedStatus, status)
				}
			}
		})
	}
}
