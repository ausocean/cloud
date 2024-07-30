/*
DESCRIPTION
  storage.go provides functionality for basic storage through the google
  storage bucket API or files. This is designed primarily with authorisation
  data token storage in mind so it is limited in function.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2021 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package broadcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2"
)

// getObject retrieves a google storage bucket object with the provided url.
// The existence of both the bucket and the object are checked and errors
// are returned if they do not exist. If the object does not exist, the object
// value is still returned along with the error, allowing creation of this
// object by writing to it.
func getObject(ctx context.Context, uri string) (*storage.ObjectHandle, error) {
	bktName, objName, err := googleStorageAddr(uri)
	if err != nil {
		return nil, fmt.Errorf("could not parse uri: %w", err)
	}

	c, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not create storage client: %w", err)
	}

	bkt := c.Bucket(bktName)
	obj := bkt.Object(objName)
	_, err = obj.Attrs(ctx)
	if err != nil {
		return obj, fmt.Errorf("error getting object named: %s: %w", objName, err)
	}

	return obj, nil
}

// saveTokObj saves the passed oauth2 token to a bucket object with name url.
// An error is returned if the object already exists.
func saveTokObj(ctx context.Context, tok *oauth2.Token, url string) error {
	obj, err := getObject(ctx, url)

	// If err == nil (object exists already) or object doesn't exist, we'll write.
	// Writing will overwrite previous data in object if exists.
	if err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
		return err
	}

	writer := obj.NewWriter(ctx)
	err = json.NewEncoder(writer).Encode(tok)
	if err != nil {
		return fmt.Errorf("could not encode token to object: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("could not close written object: %w", err)
	}
	return nil
}

// objBytes returns the bytes contained in the object at the given URL.
func objBytes(ctx context.Context, url string) ([]byte, error) {
	r, err := objReader(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("could not read bucket object: %w", err)
	}
	return ioutil.ReadAll(r)
}

// objReader provides a reader for a google storage bucket object at the given URL.
func objReader(ctx context.Context, url string) (io.Reader, error) {
	obj, err := getObject(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("could not get object: %w", err)
	}
	return obj.NewReader(ctx)
}

// objTok retrieves a google storage bucket object of the provided URL and
// extracts an oauth2.Token for return.
func objTok(ctx context.Context, url string) (*oauth2.Token, error) {
	r, err := objReader(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("could not get reader for object: %w", err)
	}

	var tok oauth2.Token
	err = json.NewDecoder(r).Decode(&tok)
	if err != nil {
		return nil, fmt.Errorf("could not read token from object reader: %w", err)
	}

	return &tok, nil
}

// fileTok extracts an oauth2.Token from a file with name specified by the given
// URL.
func fileTok(uri string) (*oauth2.Token, error) {
	var name string
	var err error
	if standalone {
		name = os.Getenv("YOUTUBE_API_CREDENTIALS")
		if name == "" {
			return nil, fmt.Errorf("YOUTUBE_API_CREDENTIALS environment variable is not set for standalone: %w", err)
		}
	} else {
		_, name, err = googleStorageAddr(uri)
		if err != nil {
			return nil, fmt.Errorf("could not parse uri: %w", err)
		}
	}

	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("could not read token file named: %s: %w", name, err)
	}

	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	if err != nil {
		return nil, fmt.Errorf("could not decode token from file: %w", err)
	}

	return t, nil
}

// saveTokFile saves the given oauth2.Token to a file with name of the provided URL.
func saveTokFile(tok *oauth2.Token, uri string) error {
	_, name, err := googleStorageAddr(uri)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	return json.NewEncoder(f).Encode(tok)
}

func googleStorageAddr(addr string) (bucket, object string, err error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", "", err
	}
	if u.Scheme != "gs" {
		return "", "", fmt.Errorf("url does not have gs scheme: %s", u)
	}
	return u.Host, strings.TrimPrefix(u.Path, "/"), nil
}
