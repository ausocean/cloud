/*
DESCRIPTION
  VidGrind audio/video handling.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2018-2023 the Australian Ocean Lab (AusOcean)

  This file is part of VidGrind. VidGrind is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  VidGrind is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with NetReceiver in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

// VidGrind audio/video handling.

package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"

	"bitbucket.org/ausocean/av/codec/wav"
	"bitbucket.org/ausocean/av/container/mts"
	"bitbucket.org/ausocean/av/container/mts/pes"
	"bitbucket.org/ausocean/iotsvc/gauth"
	"bitbucket.org/ausocean/iotsvc/iotds"
)

const (
	mtsPackets      = 7   // # of MPEG-TS packets per VidGrind packet
	rtpHeaderSize   = 12  // per RFC 3550, there are 12 octets in every RTP packet header
	rtpSSRC         = 1   // any value will do
	maxKeys         = 500 // maximum number of keys per datastore call
	hlsFragDuration = 10  // number of seconds per HLS clip
	hlsLiveDuration = 60  // HLS playlist duration when live streaming
)

// Define the MIME types for different audio requests.
const (
	mimePCM = "audio/pcm" // MIME type of PCM audio.
	mimeWAV = "audio/wav" // MIME type of WAV audio.
)

var (
	rtpSeqNum uint16 = 0
)

var (
	errInvalidKey         = errors.New("invalid key")
	errInvalidMID         = errors.New("invalid MID")
	errInvalidPin         = errors.New("invalid pin")
	errInvalidSize        = errors.New("invalid size")
	errInvalidTimestamp   = errors.New("invalid timestamp")
	errInvalidRange       = errors.New("invalid range")
	errInvalidValue       = errors.New("invalid value")
	errMissingFile        = errors.New("file missing")
	errMissingKey         = errors.New("missing key")
	errMissingTimestamp   = errors.New("missing timestamp")
	errPermissionDenied   = errors.New("permission denied")
	errUserAuthRequired   = errors.New("user authorization required")
	errNotImplemented     = errors.New("not implemented")
	errCannotExtractMedia = errors.New("could not extract media from MTS")
	errMissingType        = errors.New("media missing type")
)

// writeMtsMedia splits MTS data on PSI boundaries (~1 second for video) then writes them
// using the supplied write function. Clips should start with PSI
// (PAT and then PMT), anything prior is ignored.
func writeMtsMedia(ctx context.Context, mid int64, gh string, ts int64, data []byte, write func(context.Context, iotds.Store, *iotds.MtsMedia) error) error {
	if len(data) == 0 {
		log.Printf("writeMtsMedia(%d) called with zero-length data", mid)
		return nil
	}

	// First find PSI (PAT+PMT).
	i, s, m, err := mts.FindPSI(data)
	if err != nil {
		log.Printf("writeMtsMedia(%d) PSI not found, len=%d", mid, len(data))
		return write(ctx, mediaStore, &iotds.MtsMedia{MID: mid, Geohash: gh, Timestamp: ts, Continues: true, Clip: data})
	}

	// Get the MIME type of the media. If SIDToMIMEType returns an error i.e.
	// we have an unknown media type, the MtsMedia.Type field will remain unset.
	var mime string
	if len(s) == 0 {
		log.Printf("writeMtsMedia(%d) no elementary streams in media", mid)
	} else {
		for _, v := range s {
			mime, err = pes.SIDToMIMEType(int(v))
			if err != nil {
				log.Printf("writeMtsMedia(%d) could not get MIME type: %v", mid, err)
			}
			break
		}
	}

	// Get the write rate so we can calculate the frame period in PTS frequency units.
	wr, err := strconv.ParseFloat(m["writeRate"], 64)
	if err != nil {
		const defaultRate = 25.0 // ToDo: Write rate depends on the media type and the CODEC used.
		wr = defaultRate
		log.Printf("writeMtsMedia(%d) write rate not found; defaulting to %f", mid, wr)
	}
	fp := int64((1 / wr)) * mts.PTSFrequency

	// Get the first timestamp, or default to supplied ts.
	t, err := strconv.Atoi(m["ts"])
	if err != nil {
		log.Printf("writeMtsMedia(%d) timestamp not found; defaulting to %d", mid, ts)
	} else {
		ts = int64(t)
	}

	// Trim before first PSI.
	if i > 0 {
		log.Printf("writeMtsMedia(%d) trimming %d bytes at start", mid, i)
		data = data[i:]
		i = 0
	}
	const psiSize = 2 * mts.PacketSize // Skip the PAT and PMT.
	p := data[psiSize:]

	// Fragment every PSI, truncating if necessary
	for {
		// Find the next PSI.
		j, _, m, err := mts.FindPSI(p)
		if err != nil {
			break // No more PSI found.
		}

		t, err := strconv.Atoi(m["ts"])
		if err != nil {
			t = int(ts)
		}

		if int64(t) > ts {
			// Output up to the start of this PSI, then start a new clip.
			ts = int64(t)
			sz := i + psiSize + j
			if sz > iotds.MaxBlob && trimMTS {
				sz = iotds.MaxBlob / mts.PacketSize * mts.PacketSize
				log.Printf("writeMtsMedia(%d) trimming %d bytes at end", mid, i+psiSize+j-sz)
			}
			err := write(ctx, mediaStore, &iotds.MtsMedia{MID: mid, Geohash: gh, Timestamp: ts, Continues: true, Type: mime, Clip: data[:sz], FramePTS: fp})
			if err != nil {
				return err
			}
			data = data[i+psiSize+j:]
			p = data[:]
			i = 0
		} else {
			// Skip this PSI since it either has the same time or is lacking a time.
			p = p[j+psiSize:]
			i += j + psiSize
		}
	}

	return write(ctx, mediaStore, &iotds.MtsMedia{MID: mid, Geohash: gh, Timestamp: ts, Continues: true, Type: mime, Clip: data, FramePTS: fp})
}

type uploadData struct {
	MID int64
	commonData
}

// uploadHandler handles MTS data uploading, which requires write
// permission.
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	var isAJAX bool
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		isAJAX = true
	}

	profile, err := getProfile(w, r)
	data := uploadData{
		commonData: commonData{
			Pages:   pages("upload"),
			Profile: profile,
		},
		MID: 0,
	}
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		writeTemplate(w, r, "index.html", &data, "")
		return
	}

	ctx := r.Context()
	setup(ctx)
	data.Users, err = getUsersForSiteMenu(w, r, ctx, profile, data)
	if err != nil {
		writeTemplate(w, r, "upload.html", &data, fmt.Sprintf("could not populate site menu: %v", err.Error()))
		return
	}

	n, err := upload(w, r)
	switch err {
	case nil:
		if isAJAX {
			fmt.Fprint(w, "OK")
			return
		}
		data.MID, _ = strconv.ParseInt(r.FormValue("id"), 10, 64) // Guaranteed to succeed since nil error.
		if n == 0 {
			writeTemplate(w, r, "upload.html", &data, "")
		} else {
			writeTemplate(w, r, "upload.html", &data, fmt.Sprintf("Uploaded %d bytes", n))
		}

	case errUserAuthRequired:
		http.Redirect(w, r, "/", http.StatusUnauthorized)

	default:
		log.Printf("upload failed: %v", err.Error())
		if isAJAX {
			writeHttpError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeTemplate(w, r, "upload.html", &data, err.Error())
	}
}

// upload implements the uploadHandler logic, returning the number of bytes uploaded or an error otherwise.
func upload(w http.ResponseWriter, r *http.Request) (int, error) {
	ctx := r.Context()
	p, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		return 0, errUserAuthRequired
	}

	id := r.FormValue("id")
	var mid int64
	if id != "" {
		mid, err = strconv.ParseInt(id, 10, 64)
		if err != nil {
			return 0, errInvalidMID
		}
	}
	if r.Method == "GET" {
		return 0, nil
	}

	// geohash is optional
	gh := r.FormValue("gh")

	// ts is optional
	v := r.FormValue("ts")
	if v == "" {
		v = strconv.FormatInt(time.Now().UTC().Unix(), 10)
	}
	ts, err := splitTimestamps(v, false)
	if err != nil {
		return 0, errInvalidTimestamp
	}

	setup(ctx)
	ok, err := hasPermission(ctx, p, mid, iotds.WritePermission)
	if err != nil {
		return 0, fmt.Errorf("error checking permission: %w", err)
	}
	if !ok {
		return 0, errPermissionDenied
	}

	f, fh, err := r.FormFile("file")
	if err != nil {
		return 0, errMissingFile
	}
	log.Printf("uploading %s with %d bytes", fh.Filename, fh.Size)

	content := make([]byte, fh.Size)
	n, err := io.ReadFull(f, content)
	if err != nil {
		return 0, fmt.Errorf("error reading body: %w", err)
	}
	if n%mts.PacketSize != 0 {
		if !trimMTS {
			return 0, errInvalidSize
		}
		m := n / mts.PacketSize * mts.PacketSize
		log.Printf("upload trimming %d bytes at end", n-m)
		n = m
		content = content[:n]
	}

	err = writeMtsMedia(ctx, mid, gh, ts[0], content, iotds.WriteMtsMedia)
	if err != nil {
		return 0, fmt.Errorf("error writing MTS media: %w", err)
	}

	return n, nil
}

// recvHandler receives audio/video data from devices in the form of
// short MTS clips and stores it. The response is in JSON format. For
// a normal response, the response mirrors the request query params
// and their values, plus a timestamp (and minus the device key which
// is never revealed to clients). For errors, the response includes
// the "er" param. Server-side errors are also logged. Where we
// receive multiple pin params, POST data represents concatenated
// clips and the pin value indicates the size of each clip. It is
// therefore possible to combine an video with a audio clip in the
// same body or multiple video or audio clips.
//
// The supplied MAC address (ma) must correspond to a valid
// NetReceiver device and the supplied device key (dk) must to match
// the device's. The pin type (pn) must be either V(ideo) or S(ound).
// A missing or invalid device key is treated as zero.
//
// There is experimental support for optionally forwarding data onto
// endpoints via RTP.
func recvHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()

	q := r.URL.Query()
	ma := q.Get("ma")
	dk := q.Get("dk")

	// Is this request for a valid device?
	setup(ctx)
	dev, err := iotds.CheckDevice(ctx, settingsStore, ma, dk)
	if err != nil {
		writeDeviceError(w, dev, err)
		return
	}

	gh := q.Get("gh")

	t := q.Get("ts")
	var ts int64
	if t != "" {
		ts, err = strconv.ParseInt(t, 10, 64)
		if err != nil {
			writeError(w, err)
		}
	}
	if ts == 0 {
		ts = time.Now().Unix()
	}

	resp := make(map[string]interface{})
	resp["ma"] = ma

	var found bool
	for _, pin := range strings.Split(dev.Inputs, ",") {
		if !isMtsPin(pin) {
			continue
		}
		found = true
		v := q.Get(pin)
		if v == "" {
			continue
		}
		sz, err := strconv.Atoi(v)
		if err != nil || sz < 0 {
			resp["er"] = errInvalidValue.Error()
			break
		}
		resp[pin] = sz
		clip := make([]byte, sz)
		n, err := io.ReadFull(r.Body, clip)
		// NB: An empty body (sz == 0) is _not_ considered invalid (as it is useful for testing).
		if err != nil {
			log.Printf("Could not read Body: %v", err)
			break
		}
		if n != sz || n%mts.PacketSize != 0 {
			log.Printf("Invalid size: n = %d, sz=%d", n, sz)
			resp["er"] = errInvalidSize.Error()
			break
		}
		mid := iotds.ToMID(ma, pin)
		err = writeMtsMedia(ctx, mid, gh, ts, clip, iotds.WriteMtsMedia)
		if err != nil {
			log.Printf("Could not create MtsMedia: %v", err)
			resp["er"] = fmt.Sprintf("could not write mts media: %v", err)
			break
		}
		sid := dev.Name + "." + pin
		ep := getEndpointsByStream(dev.Skey, sid)
		for _, e := range ep {
			err := forward(ctx, clip, e)
			if err != nil {
				log.Printf("Could not forward to endpoint %s: %v", sid, err)
			}
		}
	}

	if !found {
		log.Printf("recv called without MTS data")
	}

	err = r.Body.Close()
	if err != nil {
		log.Printf("Could not close body: %v", err)
		// Don't bother to inform the client
	}

	// Insert timestamp
	resp["ts"] = ts

	// Insert location, if any
	lat, lng, alt, ok := getLocation()
	if !ok && dev.Latitude != 0 && dev.Longitude != 0 {
		// Fall back to the device location.
		lat = dev.Latitude
		lng = dev.Longitude
		alt = 0
		ok = true
	}
	if ok {
		resp["ll"] = fmt.Sprintf("%0.5f,%0.5f,%0.1f", lat, lng, alt)
	}

	// Return response to client as JSON
	jsn, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Could not marshal JSON: %v", err)
		return
	}
	fmt.Fprint(w, string(jsn))
}

// Endpoint represents a communications network destination for a stream.
// Currently it is used just for video streams.
type Endpoint struct {
	Skey    string
	Eid     string
	Sid     string
	Address string
	Port    int
	Proto   string
	Timeout int
	Enabled bool
	Updated time.Time
	Started time.Time
}

// GetEndpointsByStream returns endpoints for a given stream ID.
// To play it in VLC, open the network stream rtp://<rtpEndpoint>:16384
// ToDo: implement it!
func getEndpointsByStream(skey int64, sid string) []Endpoint {
	if rtpEndpoint == "" {
		return []Endpoint{}
	}
	e := Endpoint{
		Address: rtpEndpoint,
		Port:    16384,
		Proto:   "rtp",
	}
	return []Endpoint{e}
}

// forward sends a video clip to an endpoint.
// We don't care if UDP packet writing fails.
func forward(ctx context.Context, clip []byte, ep Endpoint) error {
	conn, err := net.Dial("udp", ep.Address+":"+strconv.Itoa(ep.Port))
	if err != nil {
		return err
	}
	sz := len(clip)
	pktSize := mtsPackets * mts.PacketSize

	switch ep.Proto {
	case "udp":
		for pos := 0; pos < sz; pos += pktSize {
			conn.Write(clip[pos : pos+pktSize])
		}
	case "rtp":
		pkt := make([]byte, rtpHeaderSize+mtsPackets*mts.PacketSize)
		for pos := 0; pos < sz; pos += pktSize {
			encapsulateRtp(clip[pos:pos+pktSize], pkt, &rtpSeqNum)
			conn.Write(pkt)
		}
	default:
		return errNotImplemented
	}
	return nil
}

// encapsulateRtp encapsulates an MPEG-TS packet, mtsPkt, within
// an RTP packet, pkt, setting the RTP header payload type (to 33),
// the timestamp and incrementing the RTP sequence number.
func encapsulateRtp(mtsPkt, pkt []byte, seq *uint16) {
	// RTP packet encapsulates the MP2T
	// first 12 bytes is the header
	// byte 0: version=2, padding=0, extension=0, cc=0
	pkt[0] = 0x80 // version (2)
	// byte 1: marker=0, pt = 33 (MP2T)
	pkt[1] = 33
	// bytes 2 & 3: sequence number
	binary.BigEndian.PutUint16(pkt[2:4], *seq)
	if *seq == ^uint16(0) {
		*seq = 0
	} else {
		*seq++
	}
	// bytes 4,5,6&7: timestamp
	timestamp := uint32(time.Now().UnixNano() / 1e6) // ms timestamp
	binary.BigEndian.PutUint32(pkt[4:8], timestamp)
	// bytes 8,9,10&11: SSRC
	binary.BigEndian.PutUint32(pkt[8:12], rtpSSRC)

	// payload follows
	copy(pkt[rtpHeaderSize:rtpHeaderSize+mtsPackets*mts.PacketSize], mtsPkt)
}

type playData struct {
	MID int64
	commonData
}

// playHandler renders the HLS player player and starts playing the
// URL supplied as a query param, if any.  Users must be logged in to
// render the player, and must have read permissions for the media
// they wish to play.
func playHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	profile, _ := getProfile(w, r)
	if profile == nil {
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()
	id := q.Get("id")

	var mid int64
	var err error
	var msg string
	if id != "" {
		mid, err = strconv.ParseInt(id, 10, 64)
		if err != nil {
			msg = errInvalidMID.Error()
		}
	}

	data := playData{
		commonData: commonData{
			Pages: pages("play"),
		},
		MID: mid,
	}

	ctx := r.Context()
	setup(ctx)
	data.Users, err = getUsersForSiteMenu(w, r, ctx, profile, data)
	if err != nil {
		writeTemplate(w, r, "play.html", &data, fmt.Sprintf("could not populate site menu: %v", err.Error()))
		return
	}

	writeTemplate(w, r, "play.html", &data, msg)
}

// getMedia handles media requests, depending on the out parameter:
//
//	m3u:   output a fixed video on demand (VOD) playlist for HLS.
//	live:  output a playlist for live HLS streaming.
//	media: extract and output MTS payload
//	ts:    output MTS as is (default)
func getMedia(w http.ResponseWriter, r *http.Request, mid int64, ts []int64, ky []uint64) (content []byte, mimeType string, err error) {
	ctx := r.Context()
	q := r.URL.Query()

	// Get optional fragment duration and (live) playlist duration.
	d := q.Get("fd")
	fd, err := strconv.ParseInt(d, 10, 6)
	if err != nil {
		fd = hlsFragDuration
	}
	d = q.Get("pd")
	pd, err := strconv.ParseInt(d, 10, 64)
	if err != nil {
		pd = hlsLiveDuration
	}

	out := q.Get("out")
	switch out {
	case "m3u":
		writePlaylist(w, r, mid, ts, fd)

	case "live":
		writeLivePlaylist(w, r, mid, pd, fd)

	case "ts", "media":
		fallthrough
	default:
		// Download media data.
		var media []iotds.MtsMedia
		var err error
		if len(ky) == 0 {
			media, err = iotds.GetMtsMedia(ctx, mediaStore, mid, nil, ts)
		} else {
			media, err = iotds.GetMtsMediaByKeys(ctx, mediaStore, ky)
		}
		if err != nil {
			return nil, "", err
		}

		if out == "media" {
			clip, err := mts.Extract(joinMedia(media))
			if err != nil {
				return nil, "", errCannotExtractMedia
			}
			mime := media[0].Type
			if mime == "" {
				return nil, "", errMissingType
			} else if mime == mimePCM {
				// Convert PCM to WAV using the metadata from the clip.
				wavFile, err := convertPcmToWav(clip, err)
				if err != nil {
					return nil, "", fmt.Errorf("unable to convert PCM to WAV: %w", err)
				}
				return wavFile, mimeWAV, nil
			}

			return clip.Bytes(), mime, nil
		} else {
			return joinMedia(media), "video/mp2t", nil
		}
	}
	return nil, "", nil
}

// splitUints splits a string representing a range of unsigned
// integers into a []uint64 slice. Legal possibilities are:
//
//	"X"    [X]
//	"-"    [0,0]
//	"X-"   [X,0]
//	"-Y"   [0,Y]
//	"X-Y", [X,Y]
//	"X,y"  [X,X+y]
//
// Missing X or Y default to 0.
func splitUints(s string) ([]uint64, error) {
	if s == "" {
		return nil, errInvalidRange
	}
	var sl []string
	var relative bool
	if strings.Contains(s, ",") {
		relative = true
		sl = strings.Split(s, ",")
	} else {
		sl = strings.Split(s, "-")
	}
	var n uint64
	var m uint64
	var err error
	if sl[0] != "" {
		n, err = strconv.ParseUint(sl[0], 10, 64)
		if err != nil {
			return nil, err
		}
	}
	if len(sl) == 1 {
		return []uint64{n}, nil
	}
	if sl[1] != "" {
		m, err = strconv.ParseUint(sl[1], 10, 64)
		if err != nil {
			return nil, err
		}
	}
	if relative {
		m += n
	}
	return []uint64{n, m}, nil
}

// splitTimestamps is a wrapper for splitUints that converts numbers
// to signed integers and provides appropriate defaults for zero
// values. When pair is true, a pair is always returned. If the first
// character is a 'T' sign, then times are interpreted as relative to
// the AusOcean epoch.
func splitTimestamps(s string, pair bool) ([]int64, error) {
	var relative bool
	if s[0] == 'T' {
		relative = true
		s = s[1:]
	}
	sl, err := splitUints(s)
	if err != nil {
		return nil, err
	}
	if pair && len(sl) != 2 {
		sl = append(sl, 0)
	}
	ts := make([]int64, len(sl))
	if sl[0] == 0 {
		ts[0] = iotds.EpochStart
	} else {
		ts[0] = int64(sl[0])
		if relative {
			ts[0] += iotds.EpochStart
		}
	}
	if len(sl) == 1 {
		return ts, nil
	}
	if sl[1] == 0 {
		ts[1] = iotds.EpochEnd
	} else {
		ts[1] = int64(sl[1])
		if relative {
			ts[1] += iotds.EpochStart
		}
	}
	return ts, nil
}

// joinMedia joins media clips into a single []byte.
func joinMedia(clips []iotds.MtsMedia) []byte {
	var data []byte
	for _, c := range clips {
		data = append(data, c.Clip...)
	}
	return data
}

// writeData writes MTS data using the supplied MIME type.
func writeData(w http.ResponseWriter, data []byte, mimeType, filename string) {
	h := w.Header()
	h.Add("Access-Control-Allow-Origin", "*")
	h.Add("Content-Type", mimeType)
	if filename != "" {
		h.Add("Content-Disposition", "attachment; filename=\""+filename+"\"")
	}
	fmt.Fprint(w, string(data))
}

// isMtsPin returns true if the pin is a video (V) or sound (S) pin, false otherwise.
func isMtsPin(pn string) bool {
	if pn == "" {
		return false
	}
	if pn[0] == 'V' || pn[0] == 'S' {
		_, err := strconv.Atoi(pn[1:])
		if err == nil {
			return true
		}
	}
	return false
}

// isLatLng returns true if ll is a valid "latitude,longitude" pair, false otherwise.
// No spaces are permitted, so these must be stripped ahead of time.
func isLatLng(ll string) bool {
	if ll == "" {
		return false
	}
	parts := strings.Split(ll, ",")
	if len(parts) < 2 {
		return false
	}
	lat, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return false
	}
	if lat > 90 || lat < -90 {
		return false
	}
	lng, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return false
	}
	if lng > 180 || lng < -180 {
		return false
	}
	return true
}

// convertPcmToWav gets metadata from the given clip, and uses this data to append a WAV header
// to the PCM data.
func convertPcmToWav(clip *mts.Clip, err error) ([]byte, error) {
	// Initialise return value.
	var wavFile wav.WAV

	// Constant metadata strings.
	const (
		strBitDepth = "bitDepth"
		strFormat   = "codec"
		strRate     = "sampleRate"
		strChannels = "channels"
	)

	// Get metadata.
	meta := clip.Frames()[0].Meta

	// Check all relevant metadata exists.
	for _, key := range []string{strBitDepth, strFormat, strRate, strChannels} {
		if _, ok := meta[key]; !ok {
			return nil, fmt.Errorf("metadata does not contain %s", key)
		}
	}

	// Parse metadata.
	bitDepth, err := strconv.Atoi(meta[strBitDepth])
	if err != nil {
		return nil, fmt.Errorf("could not parse bitdepth: %w", err)
	}
	format := meta[strFormat]
	rate, err := strconv.Atoi(meta[strRate])
	if err != nil {
		return nil, fmt.Errorf("could not parse sample rate: %w", err)
	}
	channels, err := strconv.Atoi(meta[strChannels])
	if err != nil {
		return nil, fmt.Errorf("could not parse number of channels: %w", err)
	}

	// Create a metadata struct for conversion.
	wavFile.Metadata = wav.Metadata{AudioFormat: wav.ConvertFormat[format], Channels: channels, SampleRate: rate, BitDepth: bitDepth}

	// Convert PCM to WAV.
	_, err = wavFile.Write(clip.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to write WAV data from PCM: %w", err)
	}

	return wavFile.Audio, nil
}
