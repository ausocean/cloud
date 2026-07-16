/*
DESCRIPTION
  data.go provides a handler for data requests.
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2022-2024 the Australian Ocean Lab (AusOcean).

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

package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/gofiber/fiber/v2"
)

const validFmts = "raw,csv,json,gviz"

const (
	defaultOutFmt     = "csv"
	defaultResolution = 60 // 60 data points per hour.
)

// dataHandler handles data requests.
// A valid request is of form scheme://host/data/<skey>.
// Unlike NetReceiver, we only support timestamps for start (ds) and finish (df) times.
// Data duration (dd) and data unit (du) params are currently unsupported.
func dataHandler(c *fiber.Ctx) error {
	logRequest(c)
	ctx := c.UserContext()
	setup(ctx)

	ma := c.Query("ma") // Mac.
	pn := c.Query("pn") // Pin.
	do := c.Query("do") // Data output format (e.g. csv).
	ds := c.Query("ds") // Data start as Unix timestamp.
	df := c.Query("df") // Data finish as Unix timestamp.
	dr := c.Query("dr") // Data resolution.
	tz := c.Query("tz") // Timezone.

	res := defaultResolution
	var err error
	if dr != "" {
		res, err = strconv.Atoi(dr)
		if err != nil {
			c.Status(fiber.StatusBadRequest)
			writeError(c, fmt.Errorf("could not convert data resolution to integer: %w", err))
			return nil
		}
	}

	if do == "" {
		do = defaultOutFmt
	}
	if !strings.Contains(validFmts, do) {
		c.Status(fiber.StatusBadRequest)
		writeError(c, fmt.Errorf("invalid data format: %s", do))
		return nil
	}

	// Get the site key from the request.
	req := strings.Split(c.Path(), "/")
	if len(req) < 3 {
		c.Status(fiber.StatusBadRequest)
		writeError(c, fmt.Errorf("invalid request: %s", c.Path()))
		return nil
	}
	skey, err := strconv.ParseInt(req[2], 10, 64)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		writeError(c, fmt.Errorf("invalid site key in request: %s", req[2]))
		return nil
	}
	site, err := model.GetSite(ctx, settingsStore, skey)
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		writeError(c, fmt.Errorf("could not get the site of provided site key: %w", err))
		return nil
	}

	stUnix, err := strconv.ParseInt(ds, 10, 64)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		writeError(c, fmt.Errorf("invalid start time: %w", err))
		return nil
	}
	ftUnix, err := strconv.ParseInt(df, 10, 64)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		writeError(c, fmt.Errorf("invalid finish time: %w", err))
		return nil
	}
	tzUnix, err := strconv.ParseFloat(tz, 64)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		writeError(c, fmt.Errorf("invalid timezone: %w", err))
		return nil
	}

	scalars, err := model.GetScalars(ctx, mediaStore, model.ToSID(ma, pn), []int64{stUnix, ftUnix})
	if err != nil {
		c.Status(fiber.StatusInternalServerError)
		writeError(c, fmt.Errorf("could not get scalars for provided period: %w", err))
		return nil
	}

	// Apply resolution (points per hour) by skipping some records.
	if res < 60 {
		stepSize := 60.0 / float64(res)
		var newScalars []model.Scalar
		for i := 0; i < len(scalars); i += int(stepSize) {
			newScalars = append(newScalars, scalars[i])
		}
		scalars = newScalars
	}

	// Apply sensors, if any.
	sensor, err := model.GetSensorV2(ctx, settingsStore, model.MacEncode(ma), pn)
	if err != nil && err != datastore.ErrNoSuchEntity {
		c.Status(fiber.StatusInternalServerError)
		writeError(c, fmt.Errorf("could not get sensor: %w", err))
		return nil
	}
	if sensor != nil {
		for i := range scalars {
			scalars[i].Value, err = sensor.Transform(scalars[i].Value)
			if err != nil {
				c.Status(fiber.StatusInternalServerError)
				writeError(c, fmt.Errorf("could not transform value %f: %w", scalars[i].Value, err))
				return nil
			}
		}
	}

	const timeFmt = "2006-01-02 15:04"
	switch do {
	case "csv":
		csvw := csv.NewWriter(c)
		for _, s := range scalars {
			ts := time.Unix(s.Timestamp, 0).In(fixedTimezone(tzUnix)).Format(timeFmt)
			err := csvw.Write([]string{ts, s.FormatValue(3)})
			if err != nil {
				c.Status(fiber.StatusInternalServerError)
				writeError(c, fmt.Errorf("could not write csv scalar record: %w", err))
				return nil
			}
		}
		csvw.Flush()

	case "json":
		enc := json.NewEncoder(c)

		type scalarData struct {
			d string
			v float64
		}

		type scalarOut struct {
			ma, pn, tz string
			sd         []scalarData
		}

		out := scalarOut{
			ma: ma,
			pn: pn,
			tz: tz,
			sd: make([]scalarData, len(scalars)),
		}

		for i, s := range scalars {
			ts := time.Unix(s.Timestamp, 0).Add(time.Duration(int64(60.0*site.Timezone)) * time.Minute).Format(timeFmt)
			out.sd[i].d = ts
			out.sd[i].v = s.Value
		}

		err = enc.Encode(out)
		if err != nil {
			c.Status(fiber.StatusInternalServerError)
			writeError(c, fmt.Errorf("could not encode json scalar record: %w", err))
			return nil
		}
	default:
		c.Status(fiber.StatusBadRequest)
		writeError(c, fmt.Errorf("unimplemented data output format: %s", do))
		return nil
	}
	return nil
}

func throughputsHandler(c *fiber.Ctx) error {
	logRequest(c)
	ctx := c.UserContext()
	setup(ctx)

	ma := c.Query("ma") // Mac.
	do := c.Query("do") // Data output format (e.g. csv).
	ds := c.Query("ds") // Data start as Unix timestamp.
	df := c.Query("df") // Data finish as Unix timestamp.
	tz := c.Query("tz") // Timezone.

	if do == "" {
		do = defaultOutFmt
	}
	if !strings.Contains(validFmts, do) {
		writeError(c, fmt.Errorf("invalid data format: %s", do))
	}

	stUnix, err := strconv.ParseInt(ds, 10, 64)
	if err != nil {
		writeError(c, fmt.Errorf("invalid start time: %w", err))
	}
	ftUnix, err := strconv.ParseInt(df, 10, 64)
	if err != nil {
		writeError(c, fmt.Errorf("invalid finish time: %w", err))
	}
	tzUnix, err := strconv.ParseFloat(tz, 64)
	if err != nil {
		writeError(c, fmt.Errorf("invalid timezone: %w", err))
	}

	// Get the throughput data for the device.
	dev, err := model.GetDevice(ctx, settingsStore, model.MacEncode(ma))
	if err != nil {
		writeError(c, fmt.Errorf("could not get device: %w", err))
	}
	start := time.Unix(stUnix, 0).In(fixedTimezone(tzUnix))
	end := time.Unix(ftUnix, 0).In(fixedTimezone(tzUnix))
	throughputs, err := throughputsFor(ctx, dev, start, end)
	if err != nil {
		writeError(c, fmt.Errorf("could not get throughputs for provided period: %w", err))
	}

	const timeFmt = "2006-01-02 15:04"
	switch do {
	case "csv":
		csvw := csv.NewWriter(c)
		for i, throughput := range throughputs {
			ts := start.Add(countPeriod * time.Duration(i)).Format(timeFmt)
			err := csvw.Write([]string{ts, strconv.FormatFloat(throughput, 'f', 3, 64)})
			if err != nil {
				writeError(c, fmt.Errorf("could not write csv scalar record: %w", err))
			}
		}
		csvw.Flush()

	case "json":
		enc := json.NewEncoder(c)

		type throughputData struct {
			d string
			v float64
		}

		type throughputOut struct {
			MA string           `json:"ma"`
			TZ string           `json:"tz"`
			TD []throughputData `json:"td"`
		}

		out := throughputOut{
			MA: ma,
			TZ: tz,
			TD: make([]throughputData, len(throughputs)),
		}

		for i, throughput := range throughputs {
			ts := start.Add(countPeriod * time.Duration(i)).Format(timeFmt)
			out.TD[i].d = ts
			out.TD[i].v = throughput
		}

		err = enc.Encode(out)
		if err != nil {
			writeError(c, fmt.Errorf("could not encode json scalar record: %w", err))
		}
	default:
		writeError(c, fmt.Errorf("unimplemented data output format: %s", do))
		return nil
	}
	return nil
}
