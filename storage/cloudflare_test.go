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
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ausocean/cloud/gauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ensure Cloudflare implements Provider interface.
var _ Provider = (*Cloudflare)(nil)

func TestNewCloudflare(t *testing.T) {
	cf := NewCloudflare("test-account", "test-access", "test-secret", "test-bucket")
	require.NotNil(t, cf)
	assert.Equal(t, "test-account", cf.accountID)
	assert.Equal(t, "test-access", cf.accessKey)
	assert.Equal(t, "test-secret", cf.secretKey)
	assert.Equal(t, "test-bucket", cf.bucket)
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
		t.Run(tt.accountID, func(t *testing.T) {
			cf := NewCloudflare(tt.accountID, "access", "secret", "bucket")
			assert.Equal(t, tt.want, cf.GetBaseURL())
		})
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

			require.NoError(t, err)
			require.NotNil(t, creds)

			// Verify AccessKey.
			assert.Equal(t, accessKey, creds.AccessKey)

			// Verify SessionToken is base64 encoded and begins with "jwt/".
			decodedBytes, err := base64.StdEncoding.DecodeString(creds.SessionToken)
			require.NoError(t, err, "SessionToken is not valid base64")
			decodedToken := string(decodedBytes)
			require.True(t, strings.HasPrefix(decodedToken, "jwt/"), "decoded SessionToken %q does not have prefix 'jwt/'", decodedToken)

			jwtStr := strings.TrimPrefix(decodedToken, "jwt/")

			// Verify SecretKey is the hex-encoded sha256 sum of the JWT string.
			expectedSecretKeyBytes := sha256.Sum256([]byte(jwtStr))
			assert.Equal(t, hex.EncodeToString(expectedSecretKeyBytes[:]), creds.SecretKey)

			// Verify JWT claims by parsing with gauth.GetClaims using the secret key.
			claims, err := gauth.GetClaims(jwtStr, []byte(secretKey))
			require.NoError(t, err)

			// Check iss.
			assert.Equal(t, accessKey, claims["iss"])

			// Check sub.
			assert.Equal(t, accountID, claims["sub"])

			// Check aud.
			assert.Equal(t, fmt.Sprintf("%s.r2.cloudflarestorage.com", accountID), claims["aud"])

			// Check bucket.
			assert.Equal(t, bucket, claims["bucket"])

			// Check scope.
			assert.Equal(t, "object-read-write", claims["scope"])

			// Check iat.
			iat, ok := claims["iat"].(float64)
			require.True(t, ok, "claims[iat] is not a number: %v", claims["iat"])
			assert.GreaterOrEqual(t, int64(iat), before)
			assert.LessOrEqual(t, int64(iat), after)

			// Check exp.
			exp, ok := claims["exp"].(float64)
			require.True(t, ok, "claims[exp] is not a number: %v", claims["exp"])
			assert.GreaterOrEqual(t, int64(exp), before+int64(tt.wantTTL.Seconds()))
			assert.LessOrEqual(t, int64(exp), after+int64(tt.wantTTL.Seconds()))

			// Check ttlSeconds.
			ttlSeconds, ok := claims["ttlSeconds"].(float64)
			require.True(t, ok, "claims[ttlSeconds] is not a number: %v", claims["ttlSeconds"])
			assert.Equal(t, tt.wantTTL.Seconds(), ttlSeconds)

			// Check prefix / paths.
			if tt.wantPrefix != "" {
				paths, ok := claims["paths"].(map[string]interface{})
				require.True(t, ok, "claims[paths] missing or invalid type: %v", claims["paths"])
				prefixPaths, ok := paths["prefixPaths"].([]interface{})
				require.True(t, ok, "claims[paths][prefixPaths] missing or invalid type: %v", paths["prefixPaths"])
				require.Len(t, prefixPaths, 1)
				assert.Equal(t, tt.wantPrefix, prefixPaths[0])
			} else {
				assert.NotContains(t, claims, "paths")
			}
		})
	}
}
