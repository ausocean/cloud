/*
LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This file is part of Data Blue. This is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Data Blue is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with Data Blue in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

// mts_handler.go implements the device data handler for MPEG-TS data.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"bitbucket.org/ausocean/av/container/mts"
	"bitbucket.org/ausocean/av/container/mts/pes"
	"bitbucket.org/ausocean/iotsvc/iotds"
)

// mtsHandler receives audio/video data from devices in the form of
// short MPEG-TS clips and stores it. The response is in JSON
// format. For a normal response, the response mirrors the request
// query params and their values, plus a timestamp (and minus the
// device key which is never revealed to clients). For errors, the
// response includes the "er" param. Server-side errors are also
// logged. Where we receive multiple pin params, POST data represents
// concatenated clips and the pin value indicates the size of each
// clip. It is therefore possible to combine an video with a audio
// clip in the same body or multiple video or audio clips.
//
// The supplied MAC address (ma) must correspond to a valid
// NetReceiver device and the supplied device key (dk) must to match
// the device's. The pin type (pn) must be either V(ideo) or S(ound).
func mtsHandler(w http.ResponseWriter, r *http.Request) {
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
	for _, pin := range dev.InputList() {
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
	}

	if !found {
		log.Printf("recv called without MTS data")
	}

	err = r.Body.Close()
	if err != nil {
		log.Printf("Could not close body: %v", err)
		// Don't bother to inform the client.
	}

	// Insert timestamp
	resp["ts"] = ts

	// Insert device location, if any
	if dev.Latitude != 0 && dev.Longitude != 0 {
		resp["ll"] = fmt.Sprintf("%0.5f,%0.5f", dev.Latitude, dev.Longitude)
	}

	// Return response to client as JSON
	jsn, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Could not marshal JSON: %v", err)
		return
	}
	fmt.Fprint(w, string(jsn))
}

// writeMtsMedia splits MTS data on PSI boundaries (~1 second for
// video) then writes them using the supplied write function. Clips
// should start with PSI (PAT and then PMT); anything prior is ignored.
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

	// Get the MIME type of the media. If SIDToMIMEType returns an error, i.e.
	// we have an unknown media type, then the MtsMedia.Type field will remain unset.
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
			if sz > iotds.MaxBlob {
				// Trim clips if they exceed the max blob size.
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
