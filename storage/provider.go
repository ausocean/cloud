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
	"time"

	"gocloud.dev/blob"
)

// DefaultSignedURLExpiry is used by providers when a non-positive URL time
// to live is supplied.
const DefaultSignedURLExpiry = 12 * time.Hour

type TempCredentials struct {
	AccessKey    string
	SecretKey    string
	SessionToken string
}

type Provider interface {
	GenerateTempCredentials(ctx context.Context, ttl time.Duration, prefix string) (*TempCredentials, error)
	GetBaseURL() string

	// OpenBucket opens the named bucket using the provider's endpoint and
	// credentials. The bucket is returned by pointer because it must not
	// be copied.
	OpenBucket(ctx context.Context, bucket string) (*blob.Bucket, error)

	// SignURL returns a pre-signed GET URL for the object identified by
	// rawURI. A non-positive ttl uses a provider-specific default.
	SignURL(ctx context.Context, rawURI string, ttl time.Duration) (string, error)
}
