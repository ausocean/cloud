package main

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

func TestGetPathValue(t *testing.T) {
	app := fiber.New()

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
		// Create fiber ctx with correct path.
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.URI().SetPath(tt.path)
		c := app.AcquireCtx(fctx)
		defer app.ReleaseCtx(c)

		val, err := getPathValue(c, tt.index)
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
