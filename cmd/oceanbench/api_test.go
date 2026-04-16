package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPathValue(t *testing.T) {
	tests := []struct {
		path      string
		index     int
		wantValue string
		wantErr   bool
	}{
		{
			path:      "/api/get/site/42",
			index:     4,
			wantValue: "42",
			wantErr:   false,
		},
		{
			path:    "/api/get/site", // Missing 4th part
			index:   4,
			wantErr: true,
		},
		{
			path:      "/api/get/device/12345",
			index:     4,
			wantValue: "12345",
			wantErr:   false,
		},
		{
			path:    "/api/get",
			index:   4,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)

		val, err := getPathValue(req, tt.index)
		if tt.wantErr {
			if err == nil {
				t.Errorf("getPathValue(%q, %d) expected error, got nil.", tt.path, tt.index)
			}
		} else {
			if err != nil {
				t.Errorf("getPathValue(%q, %d) unexpected error: %v.", tt.path, tt.index, err)
			}
			if val != tt.wantValue {
				t.Errorf("getPathValue(%q, %d) = %q, want %q.", tt.path, tt.index, val, tt.wantValue)
			}
		}
	}
}
