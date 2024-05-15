/*
DESCRIPTION
  broadcast_client.go implements a client for broadcast management.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

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
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type OceanBroadcastClient struct{}

// SaveBroadcast sends a request to save a broadcast to the broadcast manager service (oceantv).
// TODO: Add JWT signing.
func (m *OceanBroadcastClient) SaveBroadcast(ctx context.Context, cfg *Cfg) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error marshalling BroadcastConfig: %w", err)
	}

	const saveMethod = "/broadcast/save"
	url := tvURL + saveMethod
	reader := bytes.NewReader(data)
	req, err := http.NewRequest("POST", url, reader)
	if err != nil {
		return fmt.Errorf("error creating %s request: %w", saveMethod, err)
	}
	req.Header.Set("Content-Type", "application/json")

	clt := &http.Client{}
	resp, err := clt.Do(req)
	if err != nil {
		return fmt.Errorf("error sending %s request: %w", saveMethod, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s request failed with status code: %s", saveMethod, http.StatusText(resp.StatusCode))
	}

	log.Printf("%s OK", saveMethod)
	return nil
}
