/*
NAME
  filter.go

DESCRIPTION
  filter.go handles incoming requests from /play. This includes uploading audio,
  as well as generating and applying filters to the audio.

AUTHOR
  David Sutton <davidsutton@ausocean.org>

LICENSE
  filter.go is Copyright (C) 2023-2024 the Australian Ocean Lab (AusOcean)

  It is free software: you can redistribute it and/or modify them
  under the terms of the GNU General Public License as published by the
  Free Software Foundation, either version 3 of the License, or (at your
  option) any later version.

  It is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
  FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License
  for more details.

  You should have received a copy of the GNU General Public License in gpl.txt.
  If not, see [GNU licenses](http://www.gnu.org/licenses).
*/

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/ausocean/av/codec/adpcm"
	"github.com/ausocean/av/codec/pcm"
)

// Parameters is a struct which contains the required information required to generate an
// audiofilter.
type Parameters struct {
	BitDepth   uint
	Channels   uint
	SampleRate uint
	FilterType string
	FcLower    float64
	FcUpper    float64
	AmpFactor  float64
}

// filterHandler handles HTTP POST requests sent to play/audiorequest input. The function receives the
// filter parameters and creates an appropriate filter and applies it to the current audio file.
func filterHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setup(ctx)

	// Ensure correct request type.
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Read the body of the HTTP request.
	reader, err := r.MultipartReader()
	if err != nil {
		reportFilterError(w, r, "unable to create multipart reader: %v", err)
		return
	}

	const maxReadSize = 32 << 20
	form, err := reader.ReadForm(maxReadSize)
	if err != nil {
		reportFilterError(w, r, "unable to read form: %v", err)
		return
	}

	file, err := form.File["audio-file"][0].Open()
	if err != nil {
		reportFilterError(w, r, "unable to open file: %v", err)
		return
	}
	audio, err := io.ReadAll(file)
	if err != nil {
		reportFilterError(w, r, "unable to read file: %v", err)
		return
	}

	// Convert all parameters to a usable format.
	parameters, err := convParamTypes(form)
	if err != nil {
		reportFilterError(w, r, "could not convert paramaters to required type: %v", err)
		return
	}

	// Decode the input audio for the right type.
	fileType := strings.ToLower(strings.TrimPrefix(path.Ext(form.File["audio-file"][0].Filename), "."))
	switch fileType {
	case "wav":
		// Get the header info.
		wavFMT := binary.LittleEndian.Uint16(audio[20:22])

		// If the encoding type isn't pcm, report error.
		if wavFMT != 1 {
			reportFilterError(w, r, "unsupported wav encoding type")
		}

		parameters.Channels = uint(binary.LittleEndian.Uint16(audio[22:24]))
		parameters.BitDepth = uint(binary.LittleEndian.Uint16(audio[34:36]))
		parameters.SampleRate = uint(binary.LittleEndian.Uint32(audio[24:28]))

		// Remove header from raw pcm data.
		audio = audio[40:]

		// Include header data in response.
		w.Header().Add("channels", strconv.FormatUint(uint64(parameters.Channels), 10))
		w.Header().Add("bit-depth", strconv.FormatUint(uint64(parameters.BitDepth), 10))
		w.Header().Add("sample-rate", strconv.FormatUint(uint64(parameters.SampleRate), 10))

	case "adpcm":
		// Decode adpcm.
		decoded := bytes.NewBuffer(make([]byte, 0, len(audio)*4))
		dec := adpcm.NewDecoder(decoded)
		_, err = dec.Write(audio)
		if err != nil {
			reportFilterError(w, r, "could not decode adpcm file")
		}

		// Copy decoded audio back into audio.
		audio = decoded.Bytes()

	case "raw", "pcm":
		// Do nothing.
	default:
		reportFilterError(w, r, "unknown/unsupported file type")
	}

	// Create a PCM buffer.
	buffForm := pcm.BufferFormat{SFormat: pcm.S16_LE, Rate: parameters.SampleRate, Channels: parameters.Channels}
	buff := pcm.Buffer{Format: buffForm, Data: audio}

	// Create specified filter.
	const filterLength = 100
	var filter pcm.AudioFilter
	switch parameters.FilterType {
	case "Lowpass":
		filter, err = pcm.NewLowPass(parameters.FcUpper, buff.Format, filterLength)
	case "Highpass":
		filter, err = pcm.NewHighPass(parameters.FcLower, buff.Format, filterLength)
	case "Bandpass":
		filter, err = pcm.NewBandPass(parameters.FcLower, parameters.FcUpper, buff.Format, filterLength)
	case "Bandstop":
		filter, err = pcm.NewBandStop(parameters.FcLower, parameters.FcUpper, buff.Format, filterLength)
	case "Amplifier":
		filter = pcm.NewAmplifier(parameters.AmpFactor)
	case "None":
		_, err = w.Write(buff.Data)
		if err != nil {
			reportFilterError(w, r, "unable to write to response: %v", err)
			return
		}
		log.Println("Returned unfiltered audio.")
		return
	default:
		log.Panicf("an error occurred when trying to generate filter with type: %v", parameters.FilterType)
		return
	}
	if err != nil {
		reportFilterError(w, r, "could not generate %s filter: %v", parameters.FilterType, err)
		return
	}
	log.Printf("Generated %s filter.", parameters.FilterType)

	// Apply the filter to the audio.
	output, err := filter.Apply(buff)
	if err != nil {
		reportFilterError(w, r, "unable to apply audio filter: %v", err)
		return
	}

	// Write the response.
	_, err = w.Write(output)
	if err != nil {
		reportFilterError(w, r, "unable to write to response: %v", err)
		return
	}
}

// convParamTypes takes a pointer to a multipart form and returns a Parameters struct filled with the data from the form,
// but in the correct type.
func convParamTypes(form *multipart.Form) (*Parameters, error) {
	result := &Parameters{}

	// Panic for any undefined values.
	for key, value := range form.Value {
		if value[0] == "" {
			log.Panicf("%v is undefined", key)
		}
	}

	temp, err := strconv.ParseUint(form.Value["channels"][0], 10, 16)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse Channels: %v", err)
	}
	result.Channels = uint(temp)

	temp, err = strconv.ParseUint(form.Value["sample-rate"][0], 10, 16)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse SampleRate: %v", err)
	}
	result.SampleRate = uint(temp)

	result.FilterType = form.Value["filter-type"][0]

	result.FcLower, err = strconv.ParseFloat(form.Value["fc-lower"][0], 64)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse FcLower: %v", err)
	}

	result.FcUpper, err = strconv.ParseFloat(form.Value["fc-upper"][0], 64)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse FcUpper: %v", err)
	}

	result.AmpFactor, err = strconv.ParseFloat(form.Value["amp-factor"][0], 64)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse AmpFactor: %v", err)
	}

	return result, nil
}

// reportFilterError writes a response error, specifying the error which occurred, which can be used in an
// alert to tell the user why the request failed.
func reportFilterError(w http.ResponseWriter, r *http.Request, f string, args ...interface{}) {
	msg := fmt.Sprintf(f, args...)
	log.Print(msg)
	w.Header().Add("msg", f)
	w.WriteHeader(http.StatusBadRequest)
	w.Write(nil)
}
