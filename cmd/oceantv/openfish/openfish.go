/*
DESCRIPTION
  openfish.go provides functions for registering a completed stream with
  OpenFish.

AUTHORS
  Scott Barnard <scott@ausocean.org>

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

package openfish

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/idtoken"
)

type OpenfishService struct {
	Credentials []byte
	Audience    string
}

func New() (OpenfishService, error) {
	audience := os.Getenv("OPENFISH_OAUTH2_CLIENT_ID")
	if audience == "" {
		return OpenfishService{}, errors.New("OPENFISH_OAUTH2_CLIENT_ID must be set")
	}

	url := os.Getenv("VIDGRIND_CREDENTIALS")
	if url == "" {
		return OpenfishService{}, errors.New("VIDGRIND_CREDENTIAL must be set")
	}

	ctx := context.Background()

	var creds []byte
	if strings.HasPrefix(url, "gs://") {
		// Obtain credentials from a Google Storage bucket.
		url = url[5:]
		sep := strings.IndexByte(url, '/')
		if sep == -1 {
			return OpenfishService{}, fmt.Errorf("invalid gs bucket URL: %s", url)
		}
		client, err := storage.NewClient(ctx)
		if err != nil {
			return OpenfishService{}, fmt.Errorf("storage.NewCient failed: %v ", err)
		}
		bkt := client.Bucket(url[:sep])
		obj := bkt.Object(url[sep+1:])
		r, err := obj.NewReader(ctx)
		if err != nil {
			return OpenfishService{}, fmt.Errorf("NewReader failed for gs bucket %s: %v", url, err)
		}
		defer r.Close()
		creds, err = io.ReadAll(r)
		if err != nil {
			return OpenfishService{}, fmt.Errorf("cannot read gs bucket %s: %v ", url, err)
		}
	} else {
		// Interpret url as a file name.
		var err error
		creds, err = os.ReadFile(url)
		if err != nil {
			return OpenfishService{}, fmt.Errorf("cannot read file %s: %v", url, err)
		}
	}

	return OpenfishService{Credentials: creds, Audience: audience}, nil
}

func (o *OpenfishService) RegisterStream(SID string, captureSource int, start time.Time, end time.Time) error {

	// Create new client to connect to OpenFish.
	client, err := idtoken.NewClient(context.Background(), o.Audience, idtoken.WithCredentialsJSON(o.Credentials))
	if err != nil {
		return err
	}

	jsonBytes, err := json.Marshal(struct {
		StreamUrl     string `json:"stream_url"`
		Capturesource int    `json:"capturesource"`
		Start         string `json:"startTime"`
		End           string `json:"endTime"`
	}{
		StreamUrl:     fmt.Sprintf("https://www.youtube.com/watch?v=%s", SID),
		Capturesource: captureSource,
		Start:         start.Format(time.RFC3339),
		End:           end.Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	body := bytes.NewReader(jsonBytes)

	resp, err := client.Post("https://openfish.appspot.com/api/v1/videostreams", "application/json", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 201 {
		return fmt.Errorf("openfish returned HTTP status: %s", resp.Status)
	}

	return nil
}
