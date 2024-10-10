/*
DESCRIPTION
  openfish.go provides functions for registering a completed stream with
  Openfish.

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
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func RegisterStream(SID string, captureSource int, start time.Time, end time.Time) error {

	client := &http.Client{}

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
