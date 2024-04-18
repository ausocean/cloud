/*
DESCRIPTION
  broadcast_service.go provides an interface for broadcasting services,
  and some implementations of this interface, for example,
  YouTubeBroadcastService.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2023 the Australian Ocean Lab (AusOcean)

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

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
)

// ServerResponse is an interface for a server response.
type ServerResponse interface {
	fmt.Stringer
	StatusCode() int
	HTTPHeader() http.Header
}

type BroadcastOption func(interface{}) error

// BroadcastService is an interface for a broadcast service where video
// can be streamed to and then viewed by users.
type BroadcastService interface {
	CreateBroadcast(
		ctx context.Context,
		broadcastName, description, streamName, privacy, resolution string,
		start, end time.Time,
		opts ...BroadcastOption,
	) (ServerResponse, broadcast.IDs, string, error)

	StartBroadcast(
		name, bID, sID string,
		saveLink func(key, link string) error,
		extStart, extStop func() error,
		notify func(msg string) error,
		onLiveActions func() error,
	) error

	BroadcastStatus(ctx context.Context, id string) (string, error)
	RTMPKey(ctx context.Context, streamName string) (string, error)
	CompleteBroadcast(ctx context.Context, id string) error
}

// YouTubeResponse implements the ServerResponse interface for YouTube.
// This is a wrapper for the googleapi.ServerResponse type.
type YouTubeResponse googleapi.ServerResponse

func (y YouTubeResponse) String() string          { return fmt.Sprintf("%v", googleapi.ServerResponse(y)) }
func (y YouTubeResponse) StatusCode() int         { return googleapi.ServerResponse(y).HTTPStatusCode }
func (y YouTubeResponse) HTTPHeader() http.Header { return googleapi.ServerResponse(y).Header }

// YouTubeBroadcastService is a BroadcastService implementation for YouTube.
type YouTubeBroadcastService struct {
	limiter RateLimiter
}

// WithRateLimiter is a BroadcastOption that sets the rate limiter for a
// YouTubeBroadcastService.
func WithRateLimiter(limiter RateLimiter) BroadcastOption {
	return func(i interface{}) error {
		if s, ok := i.(*YouTubeBroadcastService); ok {
			s.limiter = limiter
			return nil
		}
		return errors.New("this option is not for YouTubeBroadcastService")
	}
}

// ErrRequestLimitExceeded is an error that is returned when a request limit is
// exceeded.
var ErrRequestLimitExceeded = errors.New("request limit exceeded")

// CreateBroadcast creates a broadcast with the given parameters using the
// YouTube API.
func (s *YouTubeBroadcastService) CreateBroadcast(
	ctx context.Context,
	broadcastName, description, streamName, privacy, resolution string,
	start, end time.Time,
	opts ...BroadcastOption,
) (ServerResponse, broadcast.IDs, string, error) {
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, broadcast.IDs{}, "", fmt.Errorf("could not apply option: %w", err)
		}
	}

	if s.limiter != nil {
		if !s.limiter.RequestOK() {
			return nil, broadcast.IDs{}, "", ErrRequestLimitExceeded
		}
	}

	svc, err := broadcast.GetService(ctx, youtube.YoutubeScope)
	if err != nil {
		return YouTubeResponse{}, broadcast.IDs{}, "", fmt.Errorf("could not get service: %w", err)
	}

	const (
		typ       = "rtmp"
		framerate = "30fps"
	)
	resp, ids, err := broadcast.BroadcastStream(
		svc,
		broadcastName,
		description,
		streamName,
		privacy,
		resolution,
		typ,
		framerate,
		start,
		end,
	)
	if err != nil {
		return YouTubeResponse{}, broadcast.IDs{}, "", fmt.Errorf("could not broadcast stream: %w response: %v", err, resp)
	}

	key, err := broadcast.RTMPKey(svc, streamName)
	if err != nil {
		return YouTubeResponse{}, broadcast.IDs{}, "", fmt.Errorf("could not get stream RTMP key: %w", err)
	}

	return YouTubeResponse(resp), ids, key, nil
}

// StartBroadcast transitions a broadcast with provided name, bID, and sID to
// live status using the YouTube API. We can provide functions to be called
// before and after the broadcast is started, as well as a function to be
// called when the broadcast is live.
func (s *YouTubeBroadcastService) StartBroadcast(
	name, bID, sID string,
	saveLink func(key, link string) error,
	extStart, extStop func() error,
	notify func(msg string) error,
	onLiveActions func() error,
) error {
	return broadcast.Start(
		name,
		bID,
		sID,
		saveLink,
		extStart,
		extStop,
		notify,
		onLiveActions,
	)
}

// BroadcastStatus gets the status of the broadcast identification id using the
// YouTube API.
func (s *YouTubeBroadcastService) BroadcastStatus(ctx context.Context, id string) (string, error) {
	svc, err := broadcast.GetService(ctx, youtube.YoutubeScope)
	if err != nil {
		return "", fmt.Errorf("get service error: %w", err)
	}
	status, err := broadcast.GetBroadcastStatus(svc, id)
	if err != nil && !errors.Is(err, broadcast.ErrNoBroadcastItems) {
		return "", fmt.Errorf("get broadcast status error: %w", err)
	}
	return status, nil
}

// CompleteBroadcast transitions a broadcast with identification id to complete
// status using the YouTube API.
func (s *YouTubeBroadcastService) CompleteBroadcast(ctx context.Context, id string) error {
	svc, err := broadcast.GetService(ctx, youtube.YoutubeScope)
	if err != nil {
		return fmt.Errorf("get service error: %w", err)
	}
	err = broadcast.CompleteBroadcast(svc, id)
	if err != nil {
		return fmt.Errorf("complete broadcast error: %w", err)
	}
	return nil
}

// RTMPKey gets the broadcast RTMP key for the provided stream name using the
// YouTube API.
func (s *YouTubeBroadcastService) RTMPKey(ctx context.Context, streamName string) (string, error) {
	svc, err := broadcast.GetService(ctx, youtube.YoutubeScope)
	if err != nil {
		return "", fmt.Errorf("get service error: %w", err)
	}
	key, err := broadcast.RTMPKey(svc, streamName)
	if err != nil {
		return "", fmt.Errorf("get RTMP key error: %w", err)
	}
	return key, nil
}
