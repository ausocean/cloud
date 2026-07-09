/*
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

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

package forwarding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/model"
)

type vidforwardStatus string

const (
	vidforwardStatusPlay  vidforwardStatus = "play"
	vidforwardStatusSlate vidforwardStatus = "slate"
)

type VidforwardService struct {
	log func(string, ...interface{})

	// This is here as a temporary fix whilst this function cannot be imported directly from utils.go.
	// TODO: Remove this.
	broadcastByName func(sKey int64, name string) (*broadcast.Config, error)
}

func NewVidforwardService(log func(string, ...interface{}), broadcastByName func(sKey int64, name string) (*broadcast.Config, error)) *VidforwardService {
	return &VidforwardService{log, broadcastByName}
}

func (v *VidforwardService) Stream(cfg *broadcast.Config) error {
	return vidforwardRequest(cfg, vidforwardStatusPlay, v.log, v.broadcastByName)
}

type SlateType string

const (
	Default    SlateType = "default"
	LowVoltage SlateType = "low-voltage"
)

// WithType is an option for the Slate function that allows the caller to specify
// the type of slate to display.
// This is currently just a stub.
func WithType(slate SlateType) SlateOption {
	return func(cfg *broadcast.Config) error {
		return nil
	}
}

func (v *VidforwardService) Slate(cfg *broadcast.Config, opts ...SlateOption) error {
	return vidforwardRequest(cfg, vidforwardStatusSlate, v.log, v.broadcastByName)
}

func (v *VidforwardService) UploadSlate(cfg *broadcast.Config, name string, file io.Reader) error {
	body := &bytes.Buffer{}

	// Not closing this just yet, see close below.
	writer := multipart.NewWriter(body)

	formFileWriter, err := writer.CreateFormFile("slate-file", name)
	if err != nil {
		return fmt.Errorf("could not create form file writer: %w", err)
	}

	_, err = io.Copy(formFileWriter, file)
	if err != nil {
		return fmt.Errorf("could not copy slate file to formFileWriter: %w", err)
	}

	// We need to close the writer before we can make the request, otherwise
	// we get a "multipart EOF" error on the other side.
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("could not close writer: %w", err)
	}

	req, err := http.NewRequest("POST", "http://"+cfg.VidforwardHost+"/slate", body)
	if err != nil {
		return fmt.Errorf("could not create new /slate request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	const timeout = 10 * time.Second
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not do /slate request: %w, resp: %v", err, resp)
	}
	return nil
}

func vidforwardRequest(cfg *broadcast.Config, status vidforwardStatus, log func(string, ...interface{}), broadcastByName func(sKey int64, name string) (*broadcast.Config, error)) error {
	primary, secondary := cfg, cfg
	var err error

	// If the cfg is the secondary broadcast then we need to get the primary broadcast.
	if strings.Contains(primary.Name, broadcast.SecondaryPostfix) {
		// We need to use broadcastByName to get the primary broadcast.
		// This will mean that we'll need to trim off the broadcast.SecondaryPostfix.
		primaryName := strings.TrimSuffix(primary.Name, broadcast.SecondaryPostfix)
		primary, err = broadcastByName(primary.SKey, primaryName)
		if err != nil {
			return fmt.Errorf("could not get primary broadcast configuration: %w", err)
		}

		// Otherwise we just need to load the secondary broadcast.
	} else {
		secondary, err = broadcastByName(primary.SKey, primary.Name+broadcast.SecondaryPostfix)
		if err != nil {
			return fmt.Errorf("could not get secondary broadcast configuration: %w", err)
		}
	}

	urls := []string{broadcast.RTMPDestinationAddress + primary.RTMPKey, broadcast.RTMPDestinationAddress + secondary.RTMPKey}

	data := struct {
		MAC, Status string
		URLs        []string
	}{
		MAC:    model.MacDecode(primary.CameraMac),
		URLs:   urls,
		Status: string(status),
	}

	log("attempting to update vidforward configuration, data: %+v", data)

	// We're allowing some tolerance to failed requests here because it may be that we've
	// caught vidforward during a service restart.
	const maxRetries = 3
	err = performRequestWithRetries("http://"+cfg.VidforwardHost+"/control", data, maxRetries, log)
	if err != nil {
		return fmt.Errorf("could not perform request with retries: %v", err)
	}
	return nil
}

func performRequestWithRetries(dest string, data any, maxRetries int, log func(string, ...interface{})) error {
	var retries int
retry:
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(data)
	if err != nil {
		return fmt.Errorf("could not encode data struct: %w", err)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	httpReq, err := http.NewRequest(http.MethodPut, dest, &buf)
	if err != nil {
		return fmt.Errorf("could not create new http request: %w", err)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		log("could not do http request, but retrying: %v", err)
		if retries <= maxRetries {
			retries++
			goto retry
		}
		return fmt.Errorf("could not do http request: %w, resp: %v", err, resp)
	}

	return nil
}
