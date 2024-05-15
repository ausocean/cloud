/*
DESCRIPTION
  Ocean Bench HTTP Live Streaming (HLS) support

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2019-2024 the Australian Ocean Lab (AusOcean)

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

// Ocean Bench HTTP Live Streaming (HLS) support.
// See https://developer.apple.com/library/archive/technotes/tn2288/_index.html
// and https://tools.ietf.org/html/draft-pantos-http-live-streaming-21.
// See also sample play lists:
//   https://s3-us-west-2.amazonaws.com/hls-playground/hls.m3u8
//   https://video-dev.github.io/url_0/193039199_mp4_h264_aac_hd_7.m3u8

package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"bitbucket.org/ausocean/iotsvc/iotds"
)

// M3U tokens. Tokens ending with a colon are followed by values.
const (
	m3uBegin          = "#EXTM3U"
	m3uVersion        = "#EXT-X-VERSION:3"
	m3uPlaylistType   = "#EXT-X-PLAYLIST-TYPE:"
	m3uTargetDuration = "#EXT-X-TARGETDURATION:"
	m3uMediaSequence  = "#EXT-X-MEDIA-SEQUENCE:"
	m3uInf            = "#EXTINF:"
	m3uDiscontinuity  = "#EXT-X-DISCONTINUITY"
	m3uEnd            = "#EXT-X-ENDLIST"
	m3uFilename       = "playlist.m3u8"
)

// staleStream is the number of seconds after which a stream is considered stale.
const staleStream = 300

// liveStream keeps track of the first and last timestamp for each live stream, per Media ID.
var (
	liveStreamMutex sync.Mutex
	liveStream      map[int64][2]int64 = map[int64][2]int64{}
)

// writePlaylist writes a playlist in M3U format, comprising fragments of fd (fragment duration) seconds.
// See:
//    https://developer.apple.com/documentation/http_live_streaming/example_playlists_for_http_live_streaming/video_on_demand_playlist_construction.
//    https://developer.apple.com/library/archive/technotes/tn2288/_index.html#//apple_ref/doc/uid/DTS40012238-CH1-TNTAG2
func writePlaylist(w http.ResponseWriter, r *http.Request, mid int64, ts []int64, fd int64) {
	ctx := r.Context()

	keys, err := iotds.GetMtsMediaKeys(ctx, mediaStore, mid, nil, ts)
	if err != nil {
		log.Printf("iotds.GetMtsMediaKeys returned error: %v", err.Error())
		writeError(w, err)
		return
	}
	if len(keys) == 0 {
		w.WriteHeader(http.StatusNotFound)
		writeError(w, errors.New("no data"))
		return
	}
	_, from, _ := iotds.SplitIDKey(keys[0].ID)
	_, to, _ := iotds.SplitIDKey(keys[len(keys)-1].ID)
	from = from / fd * fd
	to = (to/fd + 1) * fd

	// Generate an M3U playlist comprising media fragments of fd seconds duration.
	output := m3uBegin + "\n"
	output += m3uPlaylistType + "VOD\n"
	output += m3uTargetDuration + strconv.Itoa(int(fd)+1) + "\n"
	output += m3uVersion + "\n"
	output += m3uMediaSequence + "0\n"

	url := "get?id=" + strconv.Itoa(int(mid))

	// Fragment clips every fd seconds
	for ts := from; ts < to; ts += fd {
		output += m3uInf + strconv.Itoa(int(fd)) + ".0,\n"
		output += url + "&ts=" + strconv.FormatInt(ts, 10) + "," + strconv.FormatInt(fd, 10) + "\n"
	}
	output += m3uEnd + "\n"

	h := w.Header()
	h.Add("Content-Disposition", "attachment; filename=\""+m3uFilename+"\"")
	h.Add("Access-Control-Allow-Origin", "*")
	h.Add("Content-Type", "application/vnd.apple.mpegurl") // preferable to "application/x-mpegURL"?
	fmt.Fprint(w, output)
}

// writeLivePlaylist writes playlists for live streaming returning pd
// (playlist duration) seconds of live data in fragments of fd
// (fragment duration). Because the MTS data is live, it is is assumed
// to be continuous and therefore discontinuities are not checked. The
// fragment duration should not be changed once the stream has
// started.
func writeLivePlaylist(w http.ResponseWriter, r *http.Request, mid int64, pd, fd int64) {
	ctx := r.Context()

	// Ensure playlist duration is a multiple of fragment duration.
	if pd%fd != 0 {
		pd = (pd / fd) * fd
	}

	// Fetch keys from now-pd seconds ago. Keys (rather than whole
	// entities) are sufficient to check for the existence of data
	// and to extract times.
	now := (time.Now().Unix() / fd) * fd
	from := now - pd
	keys, err := iotds.GetMtsMediaKeys(ctx, mediaStore, mid, nil, []int64{from, iotds.EpochEnd})
	if err != nil {
		log.Printf("iotds.GetMtsMediaKeys returned error: %v", err.Error())
		writeError(w, err)
		return
	}

	// If we have no data, either the requested MID is not
	// streaming, or the stream went stale. If the latter, we
	// reset the liveStream state.
	if len(keys) == 0 {
		var stale bool
		liveStreamMutex.Lock()
		ts := liveStream[mid]
		if ts[0] != 0 && now-ts[1] > staleStream {
			liveStream[mid] = [2]int64{0, 0}
			stale = true
		}
		liveStreamMutex.Unlock()
		w.WriteHeader(http.StatusNotFound)
		if stale {
			writeError(w, errors.New("live stream stopped"))
		} else {
			writeError(w, errors.New("no live data"))
		}
		return
	}

	// Buffer at least fd seconds of data.
	_, first, _ := iotds.SplitIDKey(keys[0].ID)
	_, last, _ := iotds.SplitIDKey(keys[len(keys)-1].ID)
	if last-first < fd {
		w.WriteHeader(http.StatusNotFound)
		writeError(w, errors.New("buffering"))
		return
	}

	// Compute the media sequence count, and update stream times.
	var count int64
	liveStreamMutex.Lock()
	ts := liveStream[mid]
	if ts[0] == 0 {
		// First time we've seen this MID.
		log.Printf("New live streaming request for MID %d starting from %d", mid, now)
		liveStream[mid] = [2]int64{now, now}
		count = 0
	} else {
		// The media sequence count is the number of fragment periods since the start of the stream.
		start := ts[0]
		count = (now - start) / fd
		liveStream[mid] = [2]int64{start, now}
	}
	liveStreamMutex.Unlock()

	// Generate a live stream playlist, i.e., a list without an end.
	output := m3uBegin + "\n"
	output += m3uPlaylistType + "VOD\n"
	output += m3uTargetDuration + strconv.Itoa(int(fd)+1) + "\n"
	output += m3uVersion + "\n"
	output += m3uMediaSequence + strconv.Itoa(int(count)) + "\n"

	url := "get?id=" + strconv.Itoa(int(mid))

	// The playlist comprises fragments of fd seconds, at times
	// which are modulo fd. Since we are dealing with time ranges
	// and are not concerned about discontinuities, we can quickly
	// construct the playlist without actually reading any media.
	for ts := from; ts < now; ts += fd {
		output += m3uInf + strconv.Itoa(int(fd)) + ".0,\n"
		output += url + "&ts=" + strconv.FormatInt(ts, 10) + "," + strconv.FormatInt(fd, 10) + "\n"
	}

	h := w.Header()
	h.Add("Content-Disposition", "attachment; filename=\""+m3uFilename+"\"")
	h.Add("Access-Control-Allow-Origin", "*")
	h.Add("Content-Type", "application/vnd.apple.mpegurl") // preferable to "application/x-mpegURL"?
	fmt.Fprint(w, output)
}
