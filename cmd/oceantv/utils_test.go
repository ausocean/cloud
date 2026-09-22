package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestTry(t *testing.T) {
	const errMsg = "got an error calling test func"

	var (
		testErr    = errors.New("test error")
		fmtFullErr = func(err error) string {
			return errMsg + ": " + err.Error()
		}
	)

	tests := []struct {
		name     string
		testFunc func() error
		wantLog  string
		wantBool bool
	}{
		{
			name: "no error",
			testFunc: func() error {
				return nil
			},
			wantLog:  "",
			wantBool: true,
		},
		{
			name: "error",
			testFunc: func() error {
				return testErr
			},
			wantLog:  fmtFullErr(testErr),
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store what was logged.
			var gotLog string
			testLog := func(s string, args ...interface{}) {
				gotLog = fmt.Sprintf(s, args...)
			}

			gotBool := try(tt.testFunc(), errMsg, testLog)
			if gotLog != tt.wantLog {
				t.Errorf("try() gotLog = %v, want %v", gotLog, tt.wantLog)
			}

			if gotBool != tt.wantBool {
				t.Errorf("try() gotBool = %v, want %v", gotBool, tt.wantBool)
			}
		})
	}
}

func TestSetActionVars(t *testing.T) {
	storeErr := errors.New("store failure")

	tests := []struct {
		name    string
		acts    []ActionVar
		store   Store
		wantErr bool
	}{
		{
			name:  "valid multiple actions",
			acts:  []ActionVar{{Name: "ESP.Power2", Value: "true"}, {Name: "Camera.mode", Value: "Normal,Shutdown"}},
			store: newDummyStore(),
		},
		{
			name:    "empty actions",
			acts:    nil,
			store:   newDummyStore(),
			wantErr: true,
		},
		{
			name:    "missing variable name",
			acts:    []ActionVar{{Name: "", Value: "true"}},
			store:   newDummyStore(),
			wantErr: true,
		},
		{
			name:    "store error propagates",
			acts:    []ActionVar{{Name: "ESP.Power2", Value: "true"}},
			store:   newDummyStore(withStoreGetError(storeErr)),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setActionVars(context.Background(), 1, tt.acts, tt.store, func(string, ...interface{}) {})
			if (err != nil) != tt.wantErr {
				t.Errorf("setActionVars() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
