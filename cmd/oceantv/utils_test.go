package main

import (
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
