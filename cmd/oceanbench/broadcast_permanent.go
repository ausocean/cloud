/*
DESCRIPTION
	broadcast_permanent.go provides helpers for setting up permanent broadcasts.

AUTHORS
	Saxon Nelson-Milton <saxon@ausocean.org>

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
	in gpl.txt.  If not, see
	<http://www.gnu.org/licenses/>.
*/

package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"bitbucket.org/ausocean/iotsvc/iotds"
)

type ForwardingService interface {
	Stream(cfg *BroadcastConfig) error
	Slate(cfg *BroadcastConfig) error
	UploadSlate(cfg *BroadcastConfig, name string, file io.Reader) error
}

type vidforwardStatus string

const (
	vidforwardStatusPlay  vidforwardStatus = "play"
	vidforwardStatusSlate vidforwardStatus = "slate"
)

type VidforwardService struct{}

func NewVidforwardService() *VidforwardService {
	return &VidforwardService{}
}

func (v *VidforwardService) Stream(cfg *BroadcastConfig) error {
	return vidforwardRequest(cfg, vidforwardStatusPlay)
}

func (v *VidforwardService) Slate(cfg *BroadcastConfig) error {
	return vidforwardRequest(cfg, vidforwardStatusSlate)
}

func (v *VidforwardService) UploadSlate(cfg *BroadcastConfig, name string, file io.Reader) error {
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

func vidforwardRequest(cfg *BroadcastConfig, status vidforwardStatus) error {
	primary, secondary := cfg, cfg
	var err error

	// If the cfg is the secondary broadcast then we need to get the primary broadcast.
	if strings.Contains(primary.Name, secondaryBroadcastPostfix) {
		// We need to use broadcastByName to get the primary broadcast.
		// This will mean that we'll need to trim off the secondaryBroadcastPostfix.
		primaryName := strings.TrimSuffix(primary.Name, secondaryBroadcastPostfix)
		primary, err = broadcastByName(primary.SKey, primaryName)
		if err != nil {
			return fmt.Errorf("could not get primary broadcast configuration: %w", err)
		}

		// Otherwise we just need to load the secondary broadcast.
	} else {
		secondary, err = broadcastByName(primary.SKey, primary.Name+secondaryBroadcastPostfix)
		if err != nil {
			return fmt.Errorf("could not get secondary broadcast configuration: %w", err)
		}
	}

	urls := []string{rtmpDestinationAddress + primary.RTMPKey, rtmpDestinationAddress + secondary.RTMPKey}

	data := struct {
		MAC, Status string
		URLs        []string
	}{
		MAC:    iotds.MacDecode(primary.CameraMac),
		URLs:   urls,
		Status: string(status),
	}

	log.Printf("broadcast: %s, ID: %s, attempting to update vidforward configuration, data: %+v", cfg.Name, cfg.ID, data)

	// We're allowing some tolerance to failed requests here because it may be that we've
	// caught vidforward during a service restart.
	const maxRetries = 3
	err = performRequestWithRetries("http://"+cfg.VidforwardHost+"/control", data, maxRetries)
	if err != nil {
		return fmt.Errorf("could not perform request with retries: %v", err)
	}
	return nil
}
