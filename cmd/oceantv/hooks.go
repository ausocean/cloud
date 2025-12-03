/*
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2025 the Australian Ocean Lab (AusOcean)

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
  along with Ocean TV in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/ausocean/cloud/cmd/oceantv/openfish"
	"github.com/ausocean/cloud/gauth"
)

// sendWebhook makes a POST request to the passed URL, with the data included
// in the request body.
//
// If the response status is OK, Accepted, or Created, the request is considered
// successful, otherwise an error is returned.
func sendWebhook(url string, data subjecter) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	tokString, err := gauth.PutClaims(map[string]any{"iss": oceanTVServiceAccount, "sub": data.subject()}, tvSecret)
	if err != nil {
		return fmt.Errorf("failed to put claims in JWT: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokString)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer func(b io.ReadCloser) {
		if err := b.Close(); err != nil {
			log.Printf("failed to close webhook response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusCreated {
		return nil
	}

	return fmt.Errorf("webhook request failed with status: %s", resp.Status)
}

// openfishEventHook is a callback function to be used to register streams
// with the openfish service.
func openfishEventHook(e event, cfg *Cfg) {
	// Only continue if we have a finished event.
	if _, ok := e.(finishedEvent); !ok {
		return
	}

	if !cfg.RegisterOpenFish {
		return
	}

	// Register stream with openfish so we can annotate the video.
	cs, err := strconv.Atoi(cfg.OpenFishCaptureSource)
	if err != nil {
		log.Printf("could not parse OpenFish capture source: %v", err)
		return
	}

	ofsvc, err := openfish.New()
	if err != nil {
		log.Printf("could not setup openfish service: %v", err)
		return
	}

	err = ofsvc.RegisterStream(cfg.SID, cs, cfg.Start, cfg.End)
	if err != nil {
		log.Printf("could not register stream with OpenFish: %v", err)
		return
	}
}

// subjecter is an interface that defines a method for retrieving the subject
// of an entity. The subject is typically used as the value for the "sub" field
// in a JWT (JSON Web Token), representing the principal that is the subject
// of the token.
type subjecter interface {
	subject() string
}

type aotvWebHookData struct {
	UUID  string `json:"uuid"`
	Name  string `json:"name"`
	BID   string `json:"bid"`
	State string `json:"state"`
}

// subjecter returns the broadcast ID of the passed stream to implement the subjecter
// interface. This will be used as the 'sub' field of the JWT request to AusOceanTV.
func (d *aotvWebHookData) subject() string {
	return d.BID
}

// ausoceanTVWebhook is a callback function used to make webhook requests to the
// AusOceanTV service.
func ausoceanTVWebhook(s state, cfg *Cfg) {
	// Only continue if we have a directLive state.
	// NOTE this can be removed if we wish to webhook for all states.
	if _, ok := s.(*directLive); !ok {
		return
	}

	data := &aotvWebHookData{
		UUID:  cfg.UUID,
		Name:  cfg.Name,
		BID:   cfg.BID,
		State: stateToString(s),
	}
	const ausoceanTVWebHookEndpoint = "/api/v1/webhooks/oceantv"
	ausoceanTVWebHookDest := aotvURL + ausoceanTVWebHookEndpoint
	err := sendWebhook(ausoceanTVWebHookDest, data)
	if err != nil {
		log.Printf("could not send AusOceanTV webhook: %v", err)
		return
	}
}
