/*
DESCRIPTION
  Ocean Bench text file handling.

AUTHORS
  Scott Barnard <scott@ausocean.org>
  Trek Hopton <trek@ausocean.org>
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2020-2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Bench. Ocean Bench is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean Bench is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

// getText handles text data requests. The text data and mime type are returned.
func getText(r *http.Request, mid int64, ts []int64, ky []uint64) ([]byte, string, error) {
	// Download text data.
	media, err := model.GetText(r.Context(), mediaStore, mid, ts)
	if err != nil && !errors.Is(err, datastore.ErrNoSuchEntity) {
		return nil, "", fmt.Errorf("could not get text from datastore: %w", err)
	}

	if errors.Is(err, datastore.ErrNoSuchEntity) || len(media) == 0 {
		bds, err := model.GetBinaryData(r.Context(), settingsStore, mid, ts)
		if err != nil {
			return nil, "", fmt.Errorf("could not get binary data from datastore: %w", err)
		}
		if len(bds) == 0 {
			const genericMimeType = "application/octet-stream"
			return []byte{}, genericMimeType, nil
		}
		data, err := joinBinaryText(bds)
		if err != nil {
			return nil, "", fmt.Errorf("could not join binary text: %w", err)
		}
		return data, bds[0].Fmt, nil
	}

	mime := media[0].Type
	return joinText(media), mime, nil
}

// joinText joins text data into a single []byte.
func joinText(texts []model.Text) []byte {
	var data []byte
	for _, t := range texts {
		data = append(data, []byte(t.Data)...)
		data = append(data, []byte("\n")...)
	}
	return data
}

func joinBinaryText(bins []model.BinaryData) ([]byte, error) {
	var data []byte
	for _, b := range bins {
		data = append(data, []byte(b.Data)...)
		data = append(data, []byte("\n")...)
	}
	return data, nil
}
