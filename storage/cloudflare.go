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
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ausocean/cloud/gauth"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

type Cloudflare struct {
	accountID string
	accessKey string
	secretKey string
	bucket    string

	// opener opens a bucket at a given endpoint. It is a field so that it
	// can be overridden in tests.
	opener bucketOpener
}

func NewCloudflare(accountID, accessKey, secretKey, bucket string) *Cloudflare {
	cf := &Cloudflare{accountID: accountID, accessKey: accessKey, secretKey: secretKey, bucket: bucket}
	cf.opener = func(ctx context.Context, endpoint, bkt string) (*blob.Bucket, error) {
		return openR2Bucket(ctx, endpoint, cf.accessKey, cf.secretKey, bkt)
	}
	return cf
}

// r2URI is the parsed form of a Cloudflare R2 storage URI, namely
// https://<account>.r2.cloudflarestorage.com/<bucket>/<key>
type r2URI struct {
	endpoint string
	bucket   string
	key      string
}

// parseR2URI parses a Cloudflare R2 storage URI.
func parseR2URI(raw string) (*r2URI, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty storage URI")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("could not parse storage URI %q: %w", raw, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("storage URI %q missing scheme or host", raw)
	}
	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("storage URI %q missing bucket or key", raw)
	}
	return &r2URI{
		endpoint: u.Scheme + "://" + u.Host,
		bucket:   parts[0],
		key:      parts[1],
	}, nil
}

// bucketOpener opens a bucket at the given endpoint.
type bucketOpener func(ctx context.Context, endpoint, bucket string) (*blob.Bucket, error)

// OpenBucket opens the specified bucket.
func (c *Cloudflare) OpenBucket(ctx context.Context, bucket string) (*blob.Bucket, error) {
	return c.opener(ctx, c.GetBaseURL(), bucket)
}

// openR2Bucket opens a gocloud.dev S3 bucket at the given R2 endpoint.
// This is not a member of *Cloudflare so that it can be overridden in tests.
func openR2Bucket(ctx context.Context, endpoint, accessKey, secretKey, bucket string) (*blob.Bucket, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion("auto"), // Required by the AWS SDK but unused by R2.
		config.WithBaseEndpoint(endpoint),
	)
	if err != nil {
		return nil, fmt.Errorf("could not load AWS config for %q: %w", endpoint, err)
	}
	b, err := s3blob.OpenBucketV2(ctx, s3.NewFromConfig(cfg), bucket, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open bucket %q at %q: %w", bucket, endpoint, err)
	}
	return b, nil
}

// SignURL returns a pre-signed GET URL for the object identified by rawURI.
func (c *Cloudflare) SignURL(ctx context.Context, rawURI string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = DefaultSignedURLExpiry
	}
	uri, err := parseR2URI(rawURI)
	if err != nil {
		return "", err
	}
	if uri.endpoint != c.GetBaseURL() {
		return "", fmt.Errorf("storage URI endpoint %q does not match provider endpoint %q", uri.endpoint, c.GetBaseURL())
	}
	b, err := c.OpenBucket(ctx, uri.bucket)
	if err != nil {
		return "", err
	}
	signed, err := b.SignedURL(ctx, uri.key, &blob.SignedURLOptions{
		Expiry: ttl,
		Method: "GET",
	})
	if err != nil {
		return "", fmt.Errorf("could not sign URL for %q: %w", rawURI, err)
	}
	return signed, nil
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
		"exp":        now.Add(ttl).Unix(),
		"iat":        now.Unix(),
		"iss":        c.accessKey,
		"sub":        c.accountID,
		"aud":        fmt.Sprintf("%s.r2.cloudflarestorage.com", c.accountID),
		"bucket":     c.bucket,
		"scope":      "object-read-write",
		"ttlSeconds": int64(ttl.Seconds()),
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
		SecretKey:    hex.EncodeToString(secretKey[:]),
		SessionToken: sessionToken,
	}, nil
}

// GetBaseURL returns the base URL for the CloudFlare R2 storage provider.
// This is of the form "https://<accountID>.r2.cloudflarestorage.com"
func (c *Cloudflare) GetBaseURL() string {
	return fmt.Sprintf("https://%s.r2.cloudflarestorage.com", c.accountID)
}
