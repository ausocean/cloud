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

// handlers.go implements device data handlers, except for MPEG-TS data.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
	"github.com/ausocean/utils/sliceutils"
)

// Device state statuses.
const (
	deviceStatusOK = iota
	deviceStatusUpdate
)

var (
	errInvalidBody  = errors.New("invalid body")
	errInvalidJSON  = errors.New("invalid JSON")
	errInvalidMID   = errors.New("invalid MID")
	errInvalidPin   = errors.New("invalid pin")
	errInvalidRange = errors.New("invalid range")
	errInvalidAPI   = errors.New("invalid API request")
	errInvalidSize  = errors.New("invalid size")
	errInvalidValue = errors.New("invalid value")
)

// configHandler handles configuration requests for a given device.
//
// Query params represent various client properties:
// - ma: MAC address.
// - dk: Device key.
// - vn: Protocol version number.
// - ut: Uptime.
// - la: Local (IP) address.
// - vt: Var types present in body when non-zero.
func configHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()

	q := r.URL.Query()
	ma := q.Get("ma")
	dk := q.Get("dk")
	vn := q.Get("vn")
	ut := q.Get("ut")
	la := q.Get("la")
	vt := q.Get("vt")

	// Is this request for a valid device?
	setup(ctx)
	dev, err := model.CheckDevice(ctx, settingsStore, ma, dk)

	var dkey int64
	switch err {
	case nil, model.ErrInvalidDeviceKey:
		dkey, _ = strconv.ParseInt(dk, 10, 64) // Can't fail.
	case model.ErrMissingDeviceKey:
		// Device key defaults to zero.
	case datastore.ErrNoSuchEntity:
		log.Printf("/config from unknown device %s", ma)
		writeError(w, model.ErrDeviceNotFound)
		return
	default:
		writeDeviceError(w, dev, err)
		return
	}

	// Extract var types from the body if vt is present.
	var varTypes map[string]string
	if vt != "" {
		n, err := strconv.Atoi(vt)
		if err != nil {
			log.Printf("error parsing vt param for device %s: %v", ma, err)
			writeError(w, errInvalidSize)
			return
		}
		body := make([]byte, n)
		_, err = io.ReadFull(r.Body, body)
		if err != nil {
			log.Printf("error reading body for device %s: %v", ma, err)
			writeError(w, errInvalidBody)
			return
		}
		err = json.Unmarshal(body, &varTypes)
		if err != nil {
			log.Printf("error unmarshalling var types for device %s: %v", ma, err)
			writeError(w, errInvalidJSON)
			return
		}
	}

	// NB: Only reveal the device key if it has changed.
	dk = ""

	if dev.Status == deviceStatusOK {
		// Device is configured, so check the device key matches.
		if dkey != dev.Dkey {
			// We should not get here. A known, configured device is using the wrong key,
			// so we return an error rather than forcing the device to reconfigure.
			log.Printf("/config from device %s with invalid device key %d", ma, dkey)
			writeError(w, model.ErrInvalidDeviceKey)
			return
		}

	} else {
		// Device is not configured
		log.Printf("/config from unconfigured device %s", ma)
		if dkey != dev.Dkey {
			// Inform the device of its new key.
			dk = strconv.FormatInt(dev.Dkey, 10)
		}
		dev.Status = deviceStatusOK
	}

	vs, err := model.GetVarSum(ctx, settingsStore, dev.Skey, dev.Hex())
	if err != nil {
		log.Printf("could not get var sum for device %s: %v", ma, err)
	}

	resp, err := configJSON(dev, vs, dk)
	if err != nil {
		log.Printf("could not generate config response JSON for device %s: %v", ma, err)
		writeError(w, err)
		return
	}
	fmt.Fprint(w, resp)

	// NB: Perform datastore operations _after_ responding to the client.
	// Update the device.
	dev.Updated = time.Now()
	if vn != "" && vn != dev.Protocol {
		log.Printf("netsender %s updated to protocol %s", ma, vn)
		dev.Protocol = vn
	}
	model.PutDevice(ctx, settingsStore, dev)

	// Update the variables corresponding to the client's uptime, local address and var types.
	if ut != "" {
		model.PutVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".uptime", ut)
	}
	if la != "" {
		model.PutVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".localaddr", la)
	}
	if varTypes != nil {
		for k, v := range varTypes {
			model.PutVariable(ctx, settingsStore, dev.Skey, "_type."+k, v)
		}
	}
}

// configJSON generates JSON for a config request response given a device, varsum, and device key.
func configJSON(dev *model.Device, vs int64, dk string) (string, error) {
	config := struct {
		MAC           string `json:"ma"`
		Wifi          string `json:"wi"`
		Inputs        string `json:"ip"`
		Outputs       string `json:"op"`
		MonitorPeriod int    `json:"mp"`
		ActPeriod     int    `json:"ap"`
		Version       string `json:"cv"`
		Vs            int64  `json:"vs"`
		DK            string `json:"dk,omitempty"`
	}{
		MAC:           dev.MAC(),
		Wifi:          dev.Wifi,
		Inputs:        dev.Inputs,
		Outputs:       dev.Outputs,
		MonitorPeriod: int(dev.MonitorPeriod),
		ActPeriod:     int(dev.ActPeriod),
		Version:       dev.Version,
		Vs:            vs,
		DK:            dk,
	}

	jsonBytes, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// pollHandler handles poll requests.
func pollHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()

	q := r.URL.Query()
	ma := q.Get("ma")
	dk := q.Get("dk")
	ut := q.Get("ut")

	// Is this request for a valid device?
	setup(ctx)
	dev, err := model.CheckDevice(ctx, settingsStore, ma, dk)
	if err != nil {
		writeDeviceError(w, dev, err)
		return
	}

	for _, pin := range dev.InputList() {
		// Get numeric value for pin, if present.
		v := q.Get(pin)
		if v == "" {
			continue
		}
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			writeError(w, errInvalidValue)
			break
		}

		switch pin[0] {
		case 'A', 'D', 'X':
			err = writeScalar(r, ma, pin, n)

		case 'B':
			// Not implemented.

		case 'S', 'V':
			// Handled by mtsHandler.

		case 'T':
			err = writeText(r, ma, pin, int(n))

		default:
			log.Printf("device %s sending invalid pin: %s", ma, pin)
			err = errInvalidPin
		}

		if err != nil {
			writeError(w, err)
			return
		}
	}

	vs, err := model.GetVarSum(ctx, settingsStore, dev.Skey, dev.Hex())
	if err != nil {
		log.Printf("error getting varsum: %v", err)
	}

	respMap := map[string]interface{}{"ma": ma, "vs": int(vs)}
	if dev.Status != deviceStatusOK {
		respMap["rc"] = int(dev.Status)
	}

	err = processActuators(ctx, dev, respMap)
	if err != nil {
		writeError(w, err)
		return
	}

	resp, err := json.Marshal(respMap)
	if err != nil {
		writeError(w, fmt.Errorf("could not marshal response map %w", err))
		return
	}
	w.Write(resp)

	// NB: Perform datastore operations _after_ responding to the client.
	// Update the variable corresponding to client's uptime.
	err = model.PutVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".uptime", ut)
	if err != nil {
		log.Printf("error putting variable %s: %v", "_"+dev.Hex()+".uptime", err)
	}
}

// processActuators updates the response map with actuator values, if any.
func processActuators(ctx context.Context, dev *model.Device, respMap map[string]interface{}) error {
	acts, err := model.GetActuatorsV2(ctx, settingsStore, dev.Mac)
	if err != nil {
		return fmt.Errorf("failed to get actuators for device %d: %w", dev.Mac, err)
	}
	for _, act := range acts {
		// Ignore defunct actuators.
		if !sliceutils.ContainsString(dev.OutputList(), act.Pin) {
			continue
		}

		// Actuator var names are relative to their device.
		val, err := model.GetVariable(ctx, settingsStore, dev.Skey, dev.Hex()+"."+act.Var)
		if err != nil {
			return fmt.Errorf("failed to get actuator by %s.%s: %w", dev.Hex(), act.Pin, err)
		}

		n, err := toInt(val.Value)
		if err != nil {
			return fmt.Errorf("could not convert variable value to int: %w", err)
		}
		respMap[act.Pin] = n
	}
	return nil
}

// toInt returns 1 for "true", 0 for "false", or otherwise attempts to parse the string as an integer.
func toInt(s string) (int64, error) {
	s = strings.ToLower(s)
	switch s {
	case "true":
		return 1, nil
	case "false":
		return 0, nil
	default:
		return strconv.ParseInt(s, 10, 64)
	}
}

// writeScalar writes a scalar value.
func writeScalar(r *http.Request, ma, pin string, n float64) error {
	id := model.ToSID(ma, pin)
	ts := time.Now().Unix()
	return model.PutScalar(r.Context(), mediaStore, &model.Scalar{ID: id, Timestamp: ts, Value: n})
}

// writeText writes text data.
func writeText(r *http.Request, ma, pin string, n int) error {
	data := make([]byte, n)
	n_, err := io.ReadFull(r.Body, data)
	if err != nil {
		return err
	}
	if n != n_ {
		return errInvalidSize
	}

	mid := model.ToMID(ma, pin)
	ts := time.Now().Unix()
	tt := r.Header.Get("Content-Type")
	return model.WriteText(r.Context(), mediaStore, &model.Text{MID: mid, Timestamp: ts, Data: string(data), Type: tt})
}

// actHandler handles act requests.
func actHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()
	q := r.URL.Query()
	ma := q.Get("ma")
	dk := q.Get("dk")

	// Is this request for a valid device?
	setup(ctx)
	dev, err := model.CheckDevice(ctx, settingsStore, ma, dk)
	if err != nil {
		writeDeviceError(w, dev, err)
		return
	}

	respMap := map[string]interface{}{"ma": ma}

	// If status is not okay.
	if dev.Status != deviceStatusOK {
		respMap["rc"] = int(dev.Status)
	} else {
		vs, err := model.GetVarSum(ctx, settingsStore, dev.Skey, dev.Hex())
		if err != nil {
			writeError(w, fmt.Errorf("could not get var sum: %w", err))
			return
		}

		respMap["vs"] = int(vs)
	}

	err = processActuators(ctx, dev, respMap)
	if err != nil {
		writeError(w, err)
		return
	}

	err = model.PutVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".uptime", "")
	if err != nil {
		log.Printf("error putting variable %s: %v", "_"+dev.Hex()+".uptime", err)
	}

	resp, err := json.Marshal(respMap)
	if err != nil {
		writeError(w, fmt.Errorf("could not marshal response map %w", err))
		return
	}

	w.Write(resp)
}

// varsHandler returns vars for a given device (except for system variables).
// NB: Format vs as a string, not an int.
func varsHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()

	q := r.URL.Query()
	ma := q.Get("ma")
	dk := q.Get("dk")
	md := q.Get("md")
	er := q.Get("er")

	// Is this request for a valid device?
	setup(ctx)
	dev, err := model.CheckDevice(ctx, settingsStore, ma, dk)
	if err != nil {
		writeDeviceError(w, dev, err)
		return
	}

	if md != "" {
		model.PutVariable(ctx, settingsStore, dev.Skey, dev.Hex()+".mode", md)
		model.PutVariable(ctx, settingsStore, dev.Skey, dev.Hex()+".error", er)
	}
	vars, err := model.GetVariablesBySite(ctx, settingsStore, dev.Skey, dev.Hex())
	if err != nil {
		writeError(w, err)
		return
	}

	resp := `{"id":"` + dev.Hex() + `",`
	for _, v := range vars {
		if v.IsSystemVariable() {
			continue
		}
		resp += `"` + v.Name + `":"` + v.Value + `",`

	}
	vs := model.ComputeVarSum(vars)
	resp += `"vs":"` + strconv.Itoa(int(vs)) + `"}`
	fmt.Fprint(w, resp)
}

// apiHandler handles API requests which take the form:
//
//	/api/operation/property/value
func apiHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	req := strings.Split(r.URL.Path, "/")
	if len(req) < 5 {
		writeError(w, errInvalidAPI)
		return
	}

	var (
		op   = req[2]
		prop = req[3]
		val  = req[4]
	)
	switch op {
	case "test":
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			writeError(w, fmt.Errorf("could not parse size: %w", err))
			return
		}
		body := make([]byte, n)

		// Chunk size is optional.
		chunk := n
		if len(req) == 6 {
			chunk, err = strconv.ParseInt(req[5], 10, 64)
			if err != nil {
				writeError(w, fmt.Errorf("could not parse chunk size: %w", err))
				return
			}
		}

		switch prop {
		case "upload":
			// Receive n bytes from the client.
			_, err = io.ReadFull(r.Body, body)
			if err != nil {
				writeError(w, fmt.Errorf("could not read body: %w", err))
				return
			}
			fmt.Fprint(w, "OK")
			return

		case "download":
			// Send n bytes to the client.
			h := w.Header()
			h.Add("Content-Type", "application/octet-stream")
			h.Add("Content-Disposition", "attachment; filename=\""+val+"\"")
			rand.Read(body)
			var i int64
			for i = 0; i < n; i += chunk {
				w.Write(body[i : i+chunk])
			}
			return
		}

	default:
		writeError(w, errInvalidAPI)
		return
	}
}

// writeError writes an error in JSON format.
func writeError(w http.ResponseWriter, err error) {
	writeDeviceError(w, nil, err)
}

// writeDeviceError writes an error in JSON format with an optional update response code for device key errors.
func writeDeviceError(w http.ResponseWriter, dev *model.Device, err error) {
	var rc string
	switch err {
	case model.ErrMalformedDeviceKey, model.ErrInvalidDeviceKey:
		if dev != nil {
			log.Printf("bad request from %s: %v", dev.MAC(), err)
		}
		fallthrough
	case model.ErrMissingDeviceKey:
		rc = `,"rc":` + strconv.Itoa(deviceStatusUpdate)
	}
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprint(w, `{"er":"`+err.Error()+`"`+rc+`}`)
	if debug {
		log.Println("Wrote error: " + err.Error())
	}
}
