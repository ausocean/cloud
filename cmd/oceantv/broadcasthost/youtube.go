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

package broadcasthost

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/ratelimit"
	"github.com/ausocean/cloud/ytclient"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
)

// YouTubeResponse implements the ServerResponse interface for YouTube.
// This is a wrapper for the googleapi.ServerResponse type.
type YouTubeResponse googleapi.ServerResponse

func (y YouTubeResponse) String() string          { return fmt.Sprintf("%v", googleapi.ServerResponse(y)) }
func (y YouTubeResponse) StatusCode() int         { return googleapi.ServerResponse(y).HTTPStatusCode }
func (y YouTubeResponse) HTTPHeader() http.Header { return googleapi.ServerResponse(y).Header }

// YouTube is a BroadcastHost implementation for YouTube.
type YouTube struct {
	limiter  ratelimit.RateLimiter
	log      func(string, ...interface{})
	tokenURI string
}

func NewYouTube(tokenURI string, log func(string, ...interface{})) *YouTube {
	return &YouTube{log: log, tokenURI: tokenURI}
}

// WithRateLimiter is a Option that sets the rate limiter for a
// YouTube.
func WithRateLimiter(limiter ratelimit.RateLimiter) Option {
	return func(h BroadcastHost) error {
		if s, ok := h.(*YouTube); ok {
			s.limiter = limiter
			return nil
		}
		return errors.New("this option is only for YouTube")
	}
}

// ErrRequestLimitExceeded is an error that is returned when a request limit is
// exceeded.
var ErrRequestLimitExceeded = errors.New("request limit exceeded")

// CreateBroadcast creates a broadcast with the given parameters using the
// YouTube API.
func (s *YouTube) CreateBroadcast(
	ctx context.Context,
	broadcastName, description, streamName, privacy, resolution string,
	start, end time.Time,
	opts ...Option,
) (Response, ytclient.IDs, string, error) {
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, ytclient.IDs{}, "", fmt.Errorf("could not apply option: %w", err)
		}
	}

	if s.limiter != nil {
		if !s.limiter.RequestOK() {
			return nil, ytclient.IDs{}, "", ErrRequestLimitExceeded
		}
	}

	svc, err := ytclient.GetService(ctx, youtube.YoutubeScope, s.tokenURI)
	if err != nil {
		return YouTubeResponse{}, ytclient.IDs{}, "", fmt.Errorf("could not get service: %w", err)
	}

	const (
		typ       = "rtmp"
		framerate = "30fps"
	)
	resp, ids, err := ytclient.BroadcastStream(
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
		s.log,
	)
	if err != nil {
		return YouTubeResponse{}, ytclient.IDs{}, "", fmt.Errorf("could not broadcast stream: %w response: %v", err, resp)
	}

	key, err := ytclient.RTMPKey(svc, streamName)
	if err != nil {
		return YouTubeResponse{}, ytclient.IDs{}, "", fmt.Errorf("could not get stream RTMP key: %w", err)
	}

	return YouTubeResponse(resp), ids, key, nil
}

// StartBroadcast transitions a broadcast with provided name, bID, and sID to
// live status using the YouTube API. We can provide functions to be called
// before and after the broadcast is started, as well as a function to be
// called when the broadcast is live.
func (s *YouTube) StartBroadcast(
	name, bID, sID string,
	saveLink func(key, link string) error,
	extStart, extStop func() error,
	notify func(msg string) error,
	onLiveActions func() error,
) error {
	return ytclient.Start(
		name,
		bID,
		sID,
		saveLink,
		extStart,
		extStop,
		notify,
		onLiveActions,
		s.tokenURI,
		s.log,
	)
}

// BroadcastStatus gets the status of the broadcast identification id using the
// YouTube API.
func (s *YouTube) BroadcastStatus(ctx context.Context, id string) (string, error) {
	svc, err := ytclient.GetService(ctx, youtube.YoutubeScope, s.tokenURI)
	if err != nil {
		return "", fmt.Errorf("get service error: %w", err)
	}
	status, err := ytclient.GetBroadcastStatus(svc, id)
	if err != nil && !errors.Is(err, ytclient.ErrNoBroadcastItems) {
		return "", fmt.Errorf("get broadcast status error: %w", err)
	}
	return status, nil
}

// BroadcastHealth gets the health of the stream with identification sid using
// the YouTube API. Currently the implementation returns an empty string if we
// consider the health to be OK.
//
// NOTE: an empty string is returned on good, ok or bad health, otherwise the
// type of the issue is returned. This is done because one of good, ok, or
// bad is generally a function of the bandwidth at the time, which there is
// little we can do about. The possibility remains that at some point we'll
// want to know of what it is however.
//
// Similarly, we don't consider configuration issues to be problematic,
// unless they are of error severity. This may also need to be revisited.
func (s *YouTube) BroadcastHealth(ctx context.Context, sid string) (string, error) {
	svc, err := ytclient.GetService(ctx, youtube.YoutubeScope, s.tokenURI)
	if err != nil {
		return "", fmt.Errorf("could not get youtube service: %w", err)
	}

	health, err := ytclient.GetHealthStatus(svc, sid)
	if err != nil {
		return "", fmt.Errorf("could not get health status: %w", err)
	}

	for _, v := range health.ConfigurationIssues {
		if v.Severity != "error" {
			continue
		}

		return fmt.Sprintf(
			"configuration issue: %s, reason: %s, severity: %s, type: %s, last updated (seconds): %d",
			v.Description,
			v.Reason,
			v.Severity,
			v.Type,
			health.LastUpdateTimeSeconds,
		), nil
	}

	switch health.Status {
	case "noData", "revoked":
		return health.Status, nil
	}

	return "", nil
}

// BroadcastScheduledStartTime returns the scheduled start time of a broadcast.
func (s *YouTube) BroadcastScheduledStartTime(ctx context.Context, id string) (time.Time, error) {
	svc, err := ytclient.GetService(ctx, youtube.YoutubeScope, s.tokenURI)
	if err != nil {
		return time.Time{}, fmt.Errorf("get service error: %w", err)
	}
	start, err := ytclient.GetBroadcastScheduledStart(svc, id)
	if err != nil && !errors.Is(err, ytclient.ErrNoBroadcastItems) {
		return time.Time{}, fmt.Errorf("get broadcast status error: %w", err)
	}
	startTime, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return time.Time{}, fmt.Errorf("error parsing time: %w", err)
	}
	return startTime, nil
}

// CompleteBroadcast transitions a broadcast with identification id to complete
// status using the YouTube API.
func (s *YouTube) CompleteBroadcast(ctx context.Context, id string) error {
	svc, err := ytclient.GetService(ctx, youtube.YoutubeScope, s.tokenURI)
	if err != nil {
		return fmt.Errorf("get service error: %w", err)
	}
	err = ytclient.CompleteBroadcast(svc, id, s.log)
	if err != nil {
		return fmt.Errorf("complete broadcast error: %w", err)
	}
	return nil
}

// RTMPKey gets the broadcast RTMP key for the provided stream name using the
// YouTube API.
func (s *YouTube) RTMPKey(ctx context.Context, streamName string) (string, error) {
	svc, err := ytclient.GetService(ctx, youtube.YoutubeScope, s.tokenURI)
	if err != nil {
		return "", fmt.Errorf("get service error: %w", err)
	}
	key, err := ytclient.RTMPKey(svc, streamName)
	if err != nil {
		return "", fmt.Errorf("get RTMP key error: %w", err)
	}
	return key, nil
}

// PostChatMessage posts a chat message with the provided message and token URI
// to the chat identification cID using the YouTube API.
func (s *YouTube) PostChatMessage(cID, msg string) error {
	return ytclient.PostChatMessage(cID, msg, s.tokenURI)
}

// SetBroadcastPrivacy sets the broadcast privacy of the broadcast with
// identification ID to the provided privacy using the YouTube API.
// The privacy can be one of "public", "unlisted", or "private".
// This can be called before, during or after the
// The broadcast and resulting video share ID and privacy settings.
func (s *YouTube) SetBroadcastPrivacy(ctx context.Context, id, privacy string) error {
	video := &youtube.Video{
		Id: id,
		Status: &youtube.VideoStatus{
			PrivacyStatus: privacy,
			Embeddable:    true, // This must be set, otherwise it defaults to not embeddable.
		},
	}

	svc, err := ytclient.GetService(ctx, youtube.YoutubeScope, s.tokenURI)
	if err != nil {
		return fmt.Errorf("could not get youtube service: %w", err)
	}

	call := svc.Videos.Update([]string{"status"}, video)
	resp, err := call.Do()
	if err != nil {
		return fmt.Errorf("could not update video: %w, resp: %v", err, resp)
	}
	return nil
}
