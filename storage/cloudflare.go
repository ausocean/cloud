/*
DESCRIPTION
  Interface for S3 storage providers.

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
	"time"

	"github.com/ausocean/cloud/gauth"
)

type Cloudflare struct {
	accountID string
	accessKey string
	secretKey string
	bucket    string
}

func NewCloudflare(accountID, accessKey, secretKey, bucket string) *Cloudflare {
	return &Cloudflare{accountID: accountID, accessKey: accessKey, secretKey: secretKey, bucket: bucket}
}

// GenerateTempCredentials generates temporary credentials for CloudFlare R2 requests from a set of parent credentials.
// ttl: The time to live for the temporary credentials.
// prefix: The prefix to generate temporary credentials for (optional).
func (c *Cloudflare) GenerateTempCredentials(ctx context.Context, ttl time.Duration, prefix string) (*TempCredentials, error) {
	// Default to one hour if ttl is zero.
	if ttl == 0 {
		ttl = time.Hour
	}

	now := time.Now()
	claims := map[string]interface{}{
		"exp":    now.Add(ttl).Unix(),
		"iat":    now.Unix(),
		"sub":    c.accountID,
		"aud":    fmt.Sprintf("%s.r2.cloudflarestorage.com", c.accountID),
		"bucket": c.bucket,
	}
	if prefix != "" {
		claims["paths"] = map[string][]string{
			"prefixPaths": {prefix},
		}
	}
	jwt, err := gauth.PutClaims(claims, []byte(c.secretKey))
	if err != nil {
		return nil, err
	}

	// The secret key is the SHA256 hash of the JWT.
	secretKey := sha256.Sum256([]byte(jwt))

	// The session token is base64("jwt/"+jwt).
	sessionToken := base64.StdEncoding.EncodeToString([]byte("jwt/" + jwt))

	return &TempCredentials{
		AccessKey:    c.accessKey,
		SecretKey:    string(secretKey[:]),
		SessionToken: sessionToken,
	}, nil
}

// GetBaseURL returns the base URL for the CloudFlare R2 storage provider.
// This is of the form "https://<accountID>.r2.cloudflarestorage.com"
func (c *Cloudflare) GetBaseURL() string {
	return fmt.Sprintf("https://%s.r2.cloudflarestorage.com", c.accountID)
}
