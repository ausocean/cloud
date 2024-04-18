//go:build standalone
// +build standalone

/*
DESCRIPTION
  youtube-standalone.go provides stubs with equivalent signatures of Exported
  functions in youtube.go for use in standalone vidgrind execution.

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

// Package broadcast provides functionality for setting up a YouTube livestream
// service and broadcast scheduling.
package broadcast

import (
	"context"
	"log"
	"net/http"
	"time"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
)

// GetService is a stub version of GetService in youtube.go. This logs the passed
// scope and returns an empty youtube Service.
func GetService(ctx context.Context, w http.ResponseWriter, r *http.Request, scope string) (*youtube.Service, error) {
	log.Printf("getting youtube service with, scope: %s", scope)
	return &youtube.Service{}, nil
}

// BroadcastStream is a stub version of BroadcastStream in youtube.go. This logs
// the passed broadcast/stream parameters and returns an empty google api
// ServerResponse.
func BroadcastStream(svc *youtube.Service, broadcast, stream, privacy, resolution, typ, framerate string, start, end time.Time, opts ...googleapi.CallOption) (googleapi.ServerResponse, error) {
	log.Printf("broadcasting stream with, broadcast-name: %s, stream-name: %s, privacy: %s, resolution: %s, type: %s, framerate: %s, start: %v, end: %v, options: %v", broadcast, stream, privacy, resolution, typ, framerate, start, end, opts)
	return googleapi.ServerResponse{}, nil
}

// RTMPKey is a stub version of RTMPKey in youtube.go and provides a pseudo
// RTMPKey if the passed stream title is consistent with the previously added
// stream title.
func RTMPKey(svc *youtube.Service, title string) (string, error) {
	log.Printf("getting RTMP key for title: %s", title)
	return "twe3-tes6-qbp6-frge-dmwq", nil
}
