/*
DESCRIPTION
  Ocean Bench search handling.

AUTHORS
  Alan Noble <alan@ausocean.org>
  Saxon Nelson-Milton <saxon@ausocean.org>
  Scott Barnard <scott@ausocean.org>

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

// Ocean Bench search handling.

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
)

const (
	timeFormat     = "2006-01-02 15:04"
	searchTemplate = "search.html"
)

// NOTE: this was implemented as a work around whilst Sensors were not yet
// implemented. They have been left here to be default values for pins if they do not
// have a properly defined sensor with name.
var pinMap = map[string]string{
	"V0":  "video",
	"S0":  "sound",
	"T0":  "logs",
	"T1":  "gps",
	"A0":  "battery voltage",
	"A4":  "battery voltage",
	"A2":  "24v current draw",
	"A15": "network voltage",
	"A26": "power 1 voltage",
	"A27": "power 2 voltage",
	"A14": "power 3 voltage",
	"B0":  "binary data",
	"X1":  "download",
	"X2":  "upload",
	"X10": "analog value",
	"X11": "alarmed",
	"X12": "alarm count",
	"X13": "boot reason",
	"X20": "cpu temp",
	"X21": "cpu utilization",
	"X22": "virtual mem",
	"X23": "aligner median error",
	"X24": "aligner error std dev",
	"X25": "aligner signal strength",
	"X26": "aligner link quality",
	"X27": "aligner link noise",
	"X28": "aligner link bitrate",
	"X29": "aligner reference angle",
	"X35": "salinity",
	"X36": "rv bitrate",
	"X37": "dissolved oxygen",
	"X50": "air temperature",
	"X51": "humidity",
	"X60": "sea surface temperature",
}

// searchData is data used by the template and handling code.
type searchData struct {
	Id, Pi, St, Ft, Sd, Fd, Cp, Tz, Ma, Lv, Pn, Ts string
	Period                                         int
	Resolution                                     string
	SKey                                           int64
	Exporting                                      bool
	Searching                                      bool
	Timestamps                                     []int64
	Clips                                          []*model.MtsMedia
	Type                                           string
	Device                                         *model.Device
	Devices                                        []model.Device
	PinType                                        byte
	Log                                            bool
	DataHost                                       string
	PinNames                                       map[string]string
	commonData
}

// searchHandler handles search requests.
//
// Query params:
//
//	id: Media ID
//	st: start timestamp
//	ft: finish timestamp
//	ts: timestamp range
//	tz: timezone
//	cp: clip period
//
// ToDo: log users performing searches.
func searchHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	skey, _ := profileData(profile)

	// searchData struct is used by the template.
	sd := searchData{
		commonData: commonData{
			Pages:      pages("search"),
			Standalone: standalone,
		},
		SKey:       skey,
		Id:         r.FormValue("id"),
		St:         r.FormValue("st"),
		Ft:         r.FormValue("ft"),
		Sd:         r.FormValue("sd"),
		Fd:         r.FormValue("fd"),
		Cp:         r.FormValue("cp"),
		Tz:         r.FormValue("tz"),
		Ma:         r.FormValue("ma"),
		Pn:         r.FormValue("pn"),
		Lv:         r.FormValue("lv"),
		Ts:         r.FormValue("ts"),
		Resolution: r.FormValue("resolution"),
		Exporting:  r.FormValue("export") == "true",
		Searching:  r.FormValue("search") == "true",
		DataHost:   dataHost,
		PinNames:   pinMap,
	}

	ctx := r.Context()

	site, err := model.GetSite(ctx, settingsStore, skey)
	if err != nil {
		writeTemplate(w, r, searchTemplate, &sd, fmt.Sprintf("site %d not found", skey))
		return
	}
	if !site.Public {
		_, err = model.GetUser(ctx, settingsStore, skey, profile.Email)
		if err != nil {
			writeTemplate(w, r, searchTemplate, &sd, fmt.Sprintf("site %d is private", skey))
			return
		}
	}
	if sd.Tz == "" {
		sd.Tz = formatTimezone(site.Timezone, "0")
	}

	// Compute default values.
	sd.Devices, err = model.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		writeTemplate(w, r, searchTemplate, &sd, "Datastore error: "+err.Error())
		return
	}
	if len(sd.Devices) == 0 {
		writeTemplate(w, r, searchTemplate, &sd, "No Devices to Search")
		return
	}

	// If user has input MAC/Device, set Pin field, otherwise return a blank search page.
	if sd.Ma != "" {
		sd.Device, err = model.GetDevice(ctx, settingsStore, model.MacEncode(sd.Ma))
		if err != nil {
			writeTemplate(w, r, searchTemplate, &sd, "Device not found")
			return
		}
		if sd.Pn == "" && len(sd.Device.Inputs) >= 2 {
			sd.Pn = sd.Device.Inputs[0:2]
		}
		if sd.Pn == "" {
			writeTemplate(w, r, searchTemplate, &sd, "Cannot search devices without inputs")
			return
		}

	} else {
		writeTemplate(w, r, searchTemplate, &sd, "")
		return
	}

	sensors, err := model.GetSensorsV2(ctx, settingsStore, sd.Device.Mac)
	if err != nil {
		writeError(w, fmt.Errorf("unable to get sensors for device with MAC: %s, err: %w", sd.Device.MAC(), err))
		return
	}
	for _, s := range sensors {
		sd.PinNames[s.Pin] = s.Name
	}

	// Calculate search period.
	sd.Period = calcPeriod(sd.St, sd.Ft)

	if sd.Pn == "throughput" {
		writeTemplate(w, r, searchTemplate, &sd, "")
		return
	}

	sd.PinType = sd.Pn[0]
	switch sd.PinType {
	case 'T':
		sd.Log = (sd.Pn == "T0")
		fallthrough

	case 'S', 'V':
		mid := model.ToMID(sd.Ma, sd.Pn)
		sd.Id = strconv.Itoa(int(mid))
		if !sd.Searching {
			writeTemplate(w, r, searchTemplate, &sd, "")
			return
		}
		err = searchMedia(&sd, ctx, mid)
		if err != nil {
			writeTemplate(w, r, searchTemplate, &sd, fmt.Sprintf("could not search media: %v", err))
			return
		}
		writeTemplate(w, r, searchTemplate, &sd, "")

	case 'A', 'D', 'X':
		sid := model.ToSID(sd.Ma, sd.Pn)
		sd.Id = strconv.Itoa(int(sid))
		if !sd.Exporting {
			writeTemplate(w, r, searchTemplate, &sd, "")
			return
		}
		err := export(w, r, &sd)
		if err != nil {
			errStr := fmt.Sprintf("could not export data for period %s to %s, error: %v", sd.St, sd.Ft, err)
			writeTemplate(w, r, searchTemplate, &sd, errStr)
		}

	default:
		writeTemplate(w, r, searchTemplate, &sd, "Unset pin type")
	}
}

// searchMedia finds media data for V and S pins given the start and end data/times
// and sets Timestamps in the searchData value.
func searchMedia(sd *searchData, ctx context.Context, mid int64) error {
	var (
		ts  []int64
		err error
	)

	if sd.Ts != "" {
		ts, err = splitTimestamps(sd.Ts, true)
		if err != nil {
			return fmt.Errorf("could not split timestamps: %w", err)
		}
	}

	cp, err := strconv.ParseInt(sd.Cp, 10, 64)
	if err != nil {
		cp = 0 // Do not combine search results.
	}

	// A quick search just fetches datastore keys, from which we extract timestamps.
	keys, err := model.GetMtsMediaKeys(ctx, mediaStore, mid, nil, ts)
	if err != nil {
		return fmt.Errorf("could not get MTS media keys: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}

	// Fetch the first result to get the type.
	m, err := model.GetMtsMediaByKey(ctx, mediaStore, uint64(keys[0].ID))
	if err != nil {
		return fmt.Errorf("could not get MTS media %d: %w", keys[0].ID, err)
	}
	sd.Type = m.Type

	// Since PCM data is converted to WAV on download, the MIME type should be set accordingly.
	if sd.Type == mimePCM {
		sd.Type = mimeWAV
	}

	// Iterate over the keys to extract the timestamps, combining into clips of cp seconds.
	prevTs := m.Timestamp
	sd.Timestamps = []int64{prevTs}
	for _, k := range keys[1:] {
		_, ts, _ := datastore.SplitIDKey(k.ID)
		if ts > prevTs+cp {
			sd.Timestamps = append(sd.Timestamps, ts)
			prevTs = ts
		}
	}

	return nil
}

// export retrieves data for the selected pin (either X or A) and date/time range,
// concats as required as CSV and then writes to the http.ResponseWriter for
// downloading.
func export(w http.ResponseWriter, r *http.Request, sd *searchData) error {
	stUnix, err := strconv.ParseInt(sd.St, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse start time: %w", err)
	}

	ftUnix, err := strconv.ParseInt(sd.Ft, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse finish time: %w", err)
	}

	tzUnix, err := strconv.ParseFloat(sd.Tz, 64)
	if err != nil {
		return fmt.Errorf("could not parse timezone: %w", err)
	}

	s := time.Unix(stUnix, 0).In(fixedTimezone(tzUnix)).Format(timeFormat)
	f := time.Unix(ftUnix, 0).In(fixedTimezone(tzUnix)).Format(timeFormat)

	csvData, err := retrieveData(sd, sd.SKey, stUnix, ftUnix, tzUnix)
	if err != nil {
		return fmt.Errorf("could not retrieve data: %w", err)
	}

	writeData(w, csvData, "test/csv", sd.Ma+"-"+sd.Pn+"-"+s+"-"+f+".csv")
	sd.Exporting = false
	return nil
}

// retrieveData makes queries (multiple if period longer than max hours) to get
// X and A sensor data.
// Individual queries are limited to 60 hours of data.
func retrieveData(sd *searchData, skey, stUnix, ftUnix int64, tz float64) ([]byte, error) {
	// Query consts.
	const (
		request      = "/data/"
		exportFormat = "csv"
	)

	// Timing consts.
	const maxSeconds = 60.0 * 3600.00

	diff := ftUnix - stUnix
	log.Printf("timestamp diff: %d", diff)

	nMaxPeriods := (diff + maxSeconds) / maxSeconds
	log.Printf("number of max periods in this period: %d", nMaxPeriods)

	baseURL := dataHost + request + strconv.Itoa(int(skey))

	var csvData []byte
	for i := int64(0); i < int64(nMaxPeriods); i++ {
		u, err := url.Parse(baseURL)
		if err != nil {
			return nil, fmt.Errorf("error parsing base URL for data retrieval: %w", err)
		}

		start := stUnix + (i * maxSeconds)
		finish := stUnix + ((i + 1) * maxSeconds)
		q := u.Query()
		q.Add("ma", sd.Ma)
		q.Add("pn", sd.Pn)
		q.Add("do", exportFormat)
		q.Add("ds", strconv.FormatInt(start, 10))
		q.Add("df", strconv.FormatInt(finish, 10))
		q.Add("tz", fmt.Sprintf("%.1f", tz))
		q.Add("dr", sd.Resolution)

		u.RawQuery = q.Encode()

		log.Printf("data URL num: %d = %v", i, u.String())

		resp, err := http.Get(u.String())
		if err != nil {
			return nil, fmt.Errorf("error from HTTP GET to get X/A data: %w", err)
		}

		defer resp.Body.Close()
		pageData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("could not read GET data request response body: %w", err)
		}

		csvData = append(csvData, []byte(pageData)...)
		csvData = append(csvData, byte('\n'))
	}

	return csvData, nil
}

// formatTimezone returns a string corresponding to a timezone offset, tz.
func formatTimezone(tz float64, zulu string) string {
	if tz == 0 {
		return zulu
	} else if float64(int(tz)) == tz {
		return fmt.Sprintf("%+d", int(tz))
	} else {
		return fmt.Sprintf("%+.1f", tz)
	}
}

// fixedTimezone returns a time.Location corresponding to a timezone offset, tz, such as +10.5
func fixedTimezone(tz float64) *time.Location {
	return time.FixedZone(formatTimezone(tz, "Z"), int(tz*3600))
}

// formatLocalDate formats a Unix timestamp as a local date in the given timezeone, tz.
func formatLocalDate(ts int64, tz float64) string {
	const dateFormatStr = "2006-01-02"
	return time.Unix(ts, 0).In(fixedTimezone(tz)).Format(dateFormatStr)
}

// formatLocalTime formats a Unix timestamp as a local time in the given timezeone, tz.
func formatLocalTime(ts int64, tz float64) string {
	const timeFormatStr = "15:04:05"
	return time.Unix(ts, 0).In(fixedTimezone(tz)).Format(timeFormatStr)
}

// formatLocalDateTime formats a Unix timestamp as a local date with time in the given timezeone, tz.
func formatLocalDateTime(ts int64, tz float64) string {
	return formatLocalDate(ts, tz) + " " + formatLocalTime(ts, tz)
}

// parseFloat parses a string representing a float, otherwise returns 0.
func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func calcPeriod(st, ft string) int {
	start, err := strconv.ParseInt(st, 10, 64)
	if err != nil {
		return 0
	}
	finish, err := strconv.ParseInt(ft, 10, 64)
	if err != nil {
		return 0
	}
	period := finish - start
	log.Println(period)
	switch period {
	case 24 * (60 * 60):
		return 24
	case 7 * 24 * (60 * 60):
		return 7
	default:
		return -1
	}
}
