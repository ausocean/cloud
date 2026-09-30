/*
DESCRIPTION
  OceanMedia HLS playlist generation.

AUTHORS
  Elliot Shine <elliot@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean).

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
  or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
  License for more details.

  You should have received a copy of the GNU General Public License in
  gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/gofiber/fiber/v2"
)

// playlistMIME is the MIME type used for HLS playlists.
const playlistMIME = "application/vnd.apple.mpegurl"

// skipUntilTargets is the minimum value of EXT-X-SERVER-CONTROL
// CAN-SKIP-UNTIL expressed in target durations.
const skipUntilTargets = 6

// handlePlaylist serves an HLS media playlist for a broadcast.
//
// The broadcast ID is taken from the request path, e.g.
// GET /<broadcast-id>.m3u8?token=<jwt>.
//
// LL-HLS query parameters:
//
//	_HLS_msn=<n>   - block until media sequence number n is available.
//	_HLS_skip=YES  - return a delta playlist (EXT-X-SKIP).
//	_HLS_part=<n>  - unsupported; whole segments are served.
func (s *server) handlePlaylist(c *fiber.Ctx) error {
	file := c.Params("file")
	if !strings.HasSuffix(file, ".m3u8") {
		return fiber.ErrNotFound
	}
	broadcastID := strings.TrimSuffix(file, ".m3u8")
	if broadcastID == "" {
		return fiber.ErrNotFound
	}

	if err := s.authenticate(c, broadcastID); err != nil {
		status := fiber.StatusForbidden
		if errors.Is(err, errMissingToken) || errors.Is(err, errInvalidToken) {
			status = fiber.StatusUnauthorized
		}
		return c.Status(status).JSON(fiber.Map{"error": err.Error()})
	}

	ctx := c.UserContext()

	var query struct {
		MSN  *int   `query:"_HLS_msn"`
		Part string `query:"_HLS_part"`
		Skip string `query:"_HLS_skip"`
	}
	if err := c.QueryParser(&query); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid query parameters: " + err.Error()})
	}
	if query.Part != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "_HLS_part is not supported"})
	}
	hasMSN := query.MSN != nil
	msn := 0
	if hasMSN {
		msn = *query.MSN
		if msn < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid _HLS_msn"})
		}
	}
	skip := strings.EqualFold(query.Skip, "YES")

	event, err := s.loadEvent(ctx, broadcastID)
	if err != nil {
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			return fiber.ErrNotFound
		}
		s.log("could not get broadcast %s: %v", broadcastID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not get broadcast"})
	}

	segments, err := s.loadSegments(ctx, broadcastID)
	if err != nil {
		s.log("could not get segments for broadcast %s: %v", broadcastID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not get segments"})
	}

	if hasMSN {
		segments, err = s.blockForMSN(ctx, broadcastID, msn, segments, event.EndTime.IsZero())
		if err != nil {
			if errors.Is(err, errMSNTooFarAhead) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
			}
			s.log("blocking reload for broadcast %s failed: %v", broadcastID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not get segments"})
		}
	}

	playlist, err := s.renderPlaylist(ctx, event, segments, msn, hasMSN, skip)
	if err != nil {
		s.log("could not render playlist for broadcast %s: %v", broadcastID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not render playlist"})
	}

	c.Set(fiber.HeaderContentType, playlistMIME)
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set("Access-Control-Allow-Origin", "*")
	return c.SendString(playlist)
}

// errMSNTooFarAhead is returned when a requested media sequence number
// is beyond the next expected segment.
var errMSNTooFarAhead = errors.New("requested media sequence number is too far ahead")

// blockForMSN waits until the segment with the given media sequence
// number is available, or the block timeout elapses. It returns the
// (possibly refreshed) segments and an error if the requested MSN is
// too far ahead. A non-live broadcast will never receive new segments,
// so blocking is skipped for it.
func (s *server) blockForMSN(ctx context.Context, broadcastID string, msn int, segments []model.MtsMediaV2, live bool) ([]model.MtsMediaV2, error) {
	if msn < 0 {
		return nil, errMSNTooFarAhead
	}
	// The client may request at most the next segment after the last
	// available one.
	if msn > len(segments) {
		return nil, errMSNTooFarAhead
	}
	if msn < len(segments) {
		return segments, nil
	}
	if !live {
		// The broadcast has ended and the segment does not exist.
		return nil, errMSNTooFarAhead
	}

	deadline := time.Now().Add(s.blockTimeout)
	for msn >= len(segments) && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(s.blockPoll):
		}
		var err error
		segments, err = s.loadSegments(ctx, broadcastID)
		if err != nil {
			return nil, err
		}
	}
	return segments, nil
}

// renderPlaylist builds an HLS media playlist. msn/hasMSN and skip
// control delta updates. The whole stream is included so that viewers
// can seek within a live stream.
func (s *server) renderPlaylist(ctx context.Context, event *model.BroadcastEvent, segments []model.MtsMediaV2, msn int, hasMSN, skip bool) (string, error) {
	live := event.EndTime.IsZero()

	// When a client requests a delta we omit the segments it already
	// has, i.e. those before the requested media sequence number.
	first := 0
	if hasMSN && skip && msn > 0 && msn <= len(segments) {
		first = msn
	}

	// The target duration is the longest segment duration, rounded up.
	var (
		target    int64 = 1
		totalSecs float64
	)
	for _, seg := range segments {
		secs := float64(seg.Duration) / 1000
		totalSecs += secs
		if d := int64(math.Ceil(secs)); d > target {
			target = d
		}
	}

	// The server can skip as far back as the whole playlist.
	canSkipUntil := int64(math.Ceil(totalSecs))
	if min := skipUntilTargets * target; canSkipUntil < min {
		canSkipUntil = min
	}

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	// Version 9 is required for the LL-HLS tags used below
	// (EXT-X-SERVER-CONTROL and EXT-X-SKIP).
	b.WriteString("#EXT-X-VERSION:9\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", target)
	// The playlist always begins at media sequence number 0 (the start of
	// the broadcast). For a delta playlist, EXT-X-SKIP indicates how many
	// leading segments have been omitted, so the first listed segment has
	// media sequence number "first".
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	b.WriteString("#EXT-X-PLAYLIST-TYPE:EVENT\n")
	fmt.Fprintf(&b, "#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES,CAN-SKIP-UNTIL=%d\n", canSkipUntil)
	if first > 0 {
		fmt.Fprintf(&b, "#EXT-X-SKIP:SKIPPED-SEGMENTS:%d\n", first)
	}

	for i := first; i < len(segments); i++ {
		seg := segments[i]

		signed, err := s.signURL(ctx, seg.StorageURI)
		if err != nil {
			return "", fmt.Errorf("segment %d: %w", i, err)
		}

		if i > 0 && seg.Discontinuity {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		fmt.Fprintf(&b, "#EXTINF:%.3f,\n", float64(seg.Duration)/1000)
		b.WriteString(signed)
		b.WriteString("\n")
	}

	if !live {
		b.WriteString("#EXT-X-ENDLIST\n")
	}

	return b.String(), nil
}
