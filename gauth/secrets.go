/*
AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  It is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package gauth

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"cloud.google.com/go/storage"

	"github.com/ausocean/utils/filemap"
)

// The URL scheme that represents a Google Storage Bucket.
const gsbScheme = "gs://"

// GetSecrets looks up secrets from either a file or Google Storage
// bucket specified by the <PROJECTID>_SECRETS environment variable.
// Each line is a colon-separated key and value.
// The keys argument specifies required keys.
func GetSecrets(ctx context.Context, projectID string, keys []string) (map[string]string, error) {
	var m map[string]string
	ev := strings.ToUpper(projectID) + "_SECRETS"
	url := os.Getenv(ev)
	if url == "" {
		return m, errors.New(ev + " environment variable not defined")
	}

	var bytes []byte
	var err error
	if strings.HasPrefix(url, gsbScheme) {
		bytes, err = ReadGoogleStorageBucket(ctx, url)
	} else {
		bytes, err = ioutil.ReadFile(url)
	}
	if err != nil {
		return m, err
	}

	// Strip carriage carriage returns, if any.
	s := strings.ReplaceAll(string(bytes), "\r", "")

	// There is one colon-separated secret per line.
	m = filemap.Split(s, "\n", ":")
	for _, k := range keys {
		v := m[k]
		if v == "" {
			return m, fmt.Errorf("missing key %s", k)
		}
	}
	return m, nil
}

// ReadGoogleStorageBucket read the contents of the Google Storage
// bucket specified by the URL.  The URL must take the form:
// gs://<bucket_name>/<object_name>
func ReadGoogleStorageBucket(ctx context.Context, url string) ([]byte, error) {
	if !strings.HasPrefix(url, gsbScheme) {
		return nil, fmt.Errorf("invalid GSB URL %s", url)
	}
	url = url[len(gsbScheme):]
	sep := strings.IndexByte(url, '/')
	if sep == -1 {
		return nil, fmt.Errorf("invalid GSB URL %s", url)
	}

	clt, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot create GSB client: %w", err)
	}
	bkt := clt.Bucket(url[:sep])
	obj := bkt.Object(url[sep+1:])
	r, err := obj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot create GSB reader: %w", err)
	}

	defer r.Close()
	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		return bytes, fmt.Errorf("cannot read GSB: %w", err)
	}

	return bytes, nil
}

// GetSecret gets a single secret from either a file or Google Storage
// bucket specified by the <PROJECTID>_SECRETS environment variable.
func GetSecret(ctx context.Context, projectID, key string) (string, error) {
	secrets, err := GetSecrets(ctx, projectID, []string{key})
	if err != nil {
		return "", err
	}
	return secrets[key], nil
}

// GetHexSecret gets a single hex-encoded secret and returns the decoded bytes.
func GetHexSecret(ctx context.Context, projectID, key string) ([]byte, error) {
	v, err := GetSecret(ctx, projectID, key)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(v)
}
