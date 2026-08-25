/*
DESCRIPTION
  Unit tests for Cloudflare R2 storage provider.

AUTHORS
  Elliot Shine <elliot@ausocean.org>

LICENSE
  Copyright (C) 2019-2026 the Australian Ocean Lab (AusOcean).

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
  or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
  License for more details.

  You should have received a copy of the GNU General Public License in
  gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package storage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ausocean/cloud/gauth"
)

// Ensure Cloudflare implements Provider interface.
var _ Provider = (*Cloudflare)(nil)

func TestNewCloudflare(t *testing.T) {
	cf := NewCloudflare("test-account", "test-access", "test-secret", "test-bucket")
	if cf == nil {
		t.Fatal("NewCloudflare returned nil")
	}
	if cf.accountID != "test-account" {
		t.Errorf("got accountID %q, want %q", cf.accountID, "test-account")
	}
	if cf.accessKey != "test-access" {
		t.Errorf("got accessKey %q, want %q", cf.accessKey, "test-access")
	}
	if cf.secretKey != "test-secret" {
		t.Errorf("got secretKey %q, want %q", cf.secretKey, "test-secret")
	}
	if cf.bucket != "test-bucket" {
		t.Errorf("got bucket %q, want %q", cf.bucket, "test-bucket")
	}
}

func TestGetBaseURL(t *testing.T) {
	tests := []struct {
		accountID string
		want      string
	}{
		{
			accountID: "1234567890abcdef",
			want:      "https://1234567890abcdef.r2.cloudflarestorage.com",
		},
		{
			accountID: "my-account",
			want:      "https://my-account.r2.cloudflarestorage.com",
		},
	}

	for _, tt := range tests {
		cf := NewCloudflare(tt.accountID, "access", "secret", "bucket")
		got := cf.GetBaseURL()
		if got != tt.want {
			t.Errorf("GetBaseURL() = %q, want %q", got, tt.want)
		}
	}
}

func TestGenerateTempCredentials(t *testing.T) {
	accountID := "acc-12345"
	accessKey := "access-key-abc"
	secretKey := "secret-key-xyz"
	bucket := "my-data-bucket"

	cf := NewCloudflare(accountID, accessKey, secretKey, bucket)

	tests := []struct {
		name       string
		ttl        time.Duration
		prefix     string
		wantTTL    time.Duration
		wantPrefix string
	}{
		{
			name:       "default ttl (zero) and no prefix",
			ttl:        0,
			prefix:     "",
			wantTTL:    time.Hour,
			wantPrefix: "",
		},
		{
			name:       "custom ttl and no prefix",
			ttl:        30 * time.Minute,
			prefix:     "",
			wantTTL:    30 * time.Minute,
			wantPrefix: "",
		},
		{
			name:       "custom ttl with prefix",
			ttl:        2 * time.Hour,
			prefix:     "videos/2026/",
			wantTTL:    2 * time.Hour,
			wantPrefix: "videos/2026/",
		},
		{
			name:       "default ttl with prefix",
			ttl:        0,
			prefix:     "raw/stream.m3u8",
			wantTTL:    time.Hour,
			wantPrefix: "raw/stream.m3u8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now().Unix()
			creds, err := cf.GenerateTempCredentials(context.Background(), tt.ttl, tt.prefix)
			after := time.Now().Unix()
			if err != nil {
				t.Fatalf("GenerateTempCredentials() error = %v", err)
			}
			if creds == nil {
				t.Fatal("GenerateTempCredentials() returned nil credentials")
			}

			// Verify AccessKey
			if creds.AccessKey != accessKey {
				t.Errorf("got AccessKey %q, want %q", creds.AccessKey, accessKey)
			}

			// Verify SessionToken is base64 encoded and begins with "jwt/"
			decodedBytes, err := base64.StdEncoding.DecodeString(creds.SessionToken)
			if err != nil {
				t.Fatalf("SessionToken is not valid base64: %v", err)
			}
			decodedToken := string(decodedBytes)
			if !strings.HasPrefix(decodedToken, "jwt/") {
				t.Fatalf("decoded SessionToken %q does not have prefix 'jwt/'", decodedToken)
			}

			jwtStr := strings.TrimPrefix(decodedToken, "jwt/")

			// Verify SecretKey is the sha256 sum of the JWT string
			expectedSecretKeyBytes := sha256.Sum256([]byte(jwtStr))
			if creds.SecretKey != string(expectedSecretKeyBytes[:]) {
				t.Errorf("SecretKey does not match sha256 of JWT")
			}

			// Verify JWT claims by parsing with gauth.GetClaims using the secret key
			claims, err := gauth.GetClaims(jwtStr, []byte(secretKey))
			if err != nil {
				t.Fatalf("gauth.GetClaims() failed: %v", err)
			}

			// Check sub
			if sub, ok := claims["sub"].(string); !ok || sub != accountID {
				t.Errorf("claims[sub] = %v, want %q", claims["sub"], accountID)
			}

			// Check aud
			wantAud := fmt.Sprintf("%s.r2.cloudflarestorage.com", accountID)
			if aud, ok := claims["aud"].(string); !ok || aud != wantAud {
				t.Errorf("claims[aud] = %v, want %q", claims["aud"], wantAud)
			}

			// Check bucket
			if b, ok := claims["bucket"].(string); !ok || b != bucket {
				t.Errorf("claims[bucket] = %v, want %q", claims["bucket"], bucket)
			}

			// Check iat and exp
			iatVal, ok := claims["iat"].(float64)
			if !ok {
				t.Fatalf("claims[iat] is not a number: %v", claims["iat"])
			}
			iat := int64(iatVal)
			if iat < before || iat > after {
				t.Errorf("claims[iat] %d out of expected range [%d, %d]", iat, before, after)
			}

			expVal, ok := claims["exp"].(float64)
			if !ok {
				t.Fatalf("claims[exp] is not a number: %v", claims["exp"])
			}
			exp := int64(expVal)
			expectedExpMin := before + int64(tt.wantTTL.Seconds())
			expectedExpMax := after + int64(tt.wantTTL.Seconds())
			if exp < expectedExpMin || exp > expectedExpMax {
				t.Errorf("claims[exp] %d out of expected range [%d, %d]", exp, expectedExpMin, expectedExpMax)
			}

			// Check prefix / paths
			if tt.wantPrefix != "" {
				paths, ok := claims["paths"].(map[string]interface{})
				if !ok {
					t.Fatalf("claims[paths] missing or invalid type: %v", claims["paths"])
				}
				prefixPaths, ok := paths["prefixPaths"].([]interface{})
				if !ok || len(prefixPaths) != 1 || prefixPaths[0] != tt.wantPrefix {
					t.Errorf("claims[paths][prefixPaths] = %v, want [%q]", paths["prefixPaths"], tt.wantPrefix)
				}
			} else {
				if _, exists := claims["paths"]; exists {
					t.Errorf("claims[paths] should not exist when prefix is empty, got %v", claims["paths"])
				}
			}
		})
	}
}
