/*
DESCRIPTION
  Ocean Bench location support.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2017-2024 the Australian Ocean Lab (AusOcean)

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

// Ocean Bench location support (when in standalone mode).

package main

import (
	"bufio"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/tarm/serial"
)

const (
	ggaMessage       = "$GPGGA"
	ggaSentenceParts = 15
)

// gpsStore stores location information obtained from a GPS receiver or other sources.
var gpsStore struct {
	sync.Mutex
	latitude  float64
	longitude float64
	altitude  float64
	ok        bool
}

// getLocation retrieves the current location as a lat,lng,alt,ok
// quadruplet, where ok is true if the location has been set
// previously, false otherwise.
func getLocation() (lat, lng, alt float64, ok bool) {
	gpsStore.Lock()
	lat = gpsStore.latitude
	lng = gpsStore.longitude
	alt = gpsStore.altitude
	ok = gpsStore.ok
	gpsStore.Unlock()
	return lat, lng, alt, ok
}

// setLocation sets the current location.
func setLocation(lat, lng, alt float64) {
	log.Printf("Location: %0.5f,%0.5f,%0.1f", lat, lng, alt)
	gpsStore.Lock()
	gpsStore.latitude = lat
	gpsStore.longitude = lng
	gpsStore.altitude = alt
	gpsStore.ok = true
	gpsStore.Unlock()
}

// pollGPS continually reads NMEA sentences from a GPS receiver on a
// serial port and updates the gpsStore. Note that the altitude used
// is supplied by the caller, not the one reported by the receiver. A
// negative altitude represents a depth.
func pollGPS(name string, baud int, alt float64) {
	cfg := &serial.Config{Name: name, Baud: baud}
	rd, err := serial.OpenPort(cfg)
	if err != nil {
		log.Fatalf("Error opening serial port %s: %v", name, err)
	}
	log.Printf("Polling GPS on serial port %s", name)

	sc := bufio.NewScanner(rd)
	for sc.Scan() {
		sentence := sc.Text()
		if len(sentence) == 0 {
			continue
		}
		if sentence[0] != '$' {
			continue // Partial sentence.
		}
		i := strings.Index(sentence, ",")
		if i == -1 {
			continue // Malformed sentence
		}
		if sentence[:i] != ggaMessage {
			continue // Not GGA sentence.
		}

		lat, lng, ok := parseLatLng(sentence)
		if ok {
			setLocation(lat, lng, alt)
		}
	}

	err = sc.Err()
	if err != nil {
		log.Fatalf("Error reading serial port %s: %v", name, err)
	}
}

// parseLatLng scans a NMEA GGA sentence and returns the latitude and
// longitude as floats, with ok true upon success or false otherwise.
func parseLatLng(sentence string) (lat, lng float64, ok bool) {
	parts := strings.Split(sentence, ",")
	if len(parts) != ggaSentenceParts {
		return
	}

	var err error
	lat, err = strconv.ParseFloat(parts[2][:2], 64)
	if err != nil {
		return
	}
	latMins, err := strconv.ParseFloat(parts[2][2:], 64)
	if err != nil {
		return
	}
	lat += latMins / 60
	if parts[3] == "S" {
		lat *= -1
	}

	lng, err = strconv.ParseFloat(parts[4][:3], 64)
	if err != nil {
		return
	}
	lngMins, err := strconv.ParseFloat(parts[4][3:], 64)
	if err != nil {
		return
	}
	lng += lngMins / 60
	if parts[5] == "W" {
		lng *= -1
	}
	ok = true
	return
}
