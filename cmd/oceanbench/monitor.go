/*
DESCRIPTION
  Ocean Bench site monitor handling.

AUTHORS
  Russell Stanley <russell@ausocean.org>
  David Sutton <davidsutton@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2022-2024 the Australian Ocean Lab (AusOcean)

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
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

const (
	secondsToDay     = 86400
	secondsToHour    = 3600
	secondsToMinute  = 60
	hoursToDay       = 24
	minutesToHour    = 60
	countPeriod      = 60 * time.Minute
	lastReportFormat = "Mon January 2 2006 15:04:05"
)

// sensorData holds the relevant information for each sensor.
type sensorData struct {
	Name   string
	Units  string
	Scalar string
	Date   string
	Pin    string
}

// monitorDevice holds the relevant information for each device.
type monitorDevice struct {
	Device                model.Device
	Address               string
	Sending               string
	StatusText            string
	Uptime                string
	LastReportedTimestamp int64
	Count                 int // Number of scalars sent in the monitor period.
	MaxCount              int // Max number of scalars that could be sent.
	Throughput            int // Percentage of successful scalars.
	Sensors               []sensorData
	HasT1                 bool
}

// monitorData holds the relevant information for the monitor page
type monitorData struct {
	Ma        string
	Devices   []monitorDevice
	WritePerm bool
	Timezone  float64
	SiteLat   float64
	SiteLon   float64
	commonData
}

// monitorHandler handles monitor requests.
func monitorHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}

	data := monitorData{commonData: commonData{Pages: pages("monitor"), Profile: profile}}

	ctx := r.Context()

	skey, _ := profileData(profile)

	// Check if user has write permissions to link to devices page.
	user, err := model.GetUser(ctx, settingsStore, skey, profile.Email)
	if err == nil && user.Perm&model.WritePermission != 0 {
		data.WritePerm = true
	} else if err != nil && err != datastore.ErrNoSuchEntity {
		log.Println("failed getting user permissions", err)
	}

	devices, err := model.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		reportMonitorError(w, r, &data, "could not get devices: %v", err)
		return
	}

	ch := make(chan monitorDevice, len(devices))
	errCh := make(chan string, 5)

	site, err := model.GetSite(ctx, settingsStore, skey)
	if err != nil {
		reportMonitorError(w, r, &data, "could not get devices: %v", err)
		return
	}
	data.Timezone = site.Timezone
	data.SiteLat = site.Latitude
	data.SiteLon = site.Longitude

	monitorDevices := make([]monitorDevice, len(devices))
	var wg sync.WaitGroup
	for _, device := range devices {
		wg.Add(1)
		go monitorLoadRoutine(device, site.Timezone, &wg, ch, errCh, data, skey, ctx, w, r)
	}
	wg.Wait()
	close(ch)
	close(errCh)
	i := 0
	for device := range ch {
		monitorDevices[i] = device
		i++
	}
	var errMsg string
	for err := range errCh {
		log.Println("got error from channel:", err)
		if errMsg == "" {
			errMsg = err
			continue
		}
		errMsg += ", " + err
	}
	slices.SortFunc(monitorDevices, func(a, b monitorDevice) int {
		return int(b.LastReportedTimestamp - a.LastReportedTimestamp)
	})
	data.Devices = monitorDevices
	writeTemplate(w, r, "monitor.html", &data, errMsg)
}

// scalarCount returns the number of scalars received for the first pin of a device
// for the period of time defined by start and end. This can be used to determine
// the throughput of a device.
func scalarCount(ctx context.Context, device *model.Device, start, end int64) (count int, err error) {
	for _, pin := range strings.Split(device.Inputs, ",") {
		if count != 0 || (pin[0] != 'A' && pin[0] != 'D' && pin[0] != 'X') {
			continue
		}
		sid := model.ToSID(model.MacDecode(device.Mac), pin)
		keys, err := model.GetScalarKeys(ctx, mediaStore, sid, []int64{start, end})
		if err != nil {
			return 0, fmt.Errorf("could not get scalar keys: %v", err)
		}
		count = len(keys)
		break
	}
	return count, nil
}

// throughput determines the number of scalars received within the count
// period for a device, as well as the maximum number of scalars expected.
func throughput(ctx context.Context, device *model.Device) (count, maxCount int, err error) {
	if device.Inputs == "" || device.MonitorPeriod == 0 {
		return count, maxCount, nil
	}

	monitorDuration := time.Duration(device.MonitorPeriod) * time.Second
	maxCount = int(countPeriod / monitorDuration)

	start := time.Now().Add(-countPeriod).Unix()
	count, err = scalarCount(ctx, device, start, -1)
	if err != nil {
		return 0, 0, fmt.Errorf("could not get scalar count: %v", err)
	}
	return count, maxCount, nil
}

// throughputsFor returns the throughput percentages for each count period within the time range
// defined by from and to for a device.
func throughputsFor(ctx context.Context, device *model.Device, from, to time.Time) ([]float64, error) {
	if device.Inputs == "" {
		return nil, errors.New("device has no inputs")
	}

	if device.MonitorPeriod == 0 {
		return nil, errors.New("device has no monitor period")
	}

	monitorDuration := time.Duration(device.MonitorPeriod) * time.Second
	maxCount := int(countPeriod / monitorDuration)

	var percentages []float64
	for start := from; !start.After(to); start = start.Add(countPeriod) {
		count, err := scalarCount(ctx, device, start.Unix(), start.Add(countPeriod).Unix())
		if err != nil {
			return nil, fmt.Errorf("could not get scalar count: %v", err)
		}
		percentages = append(percentages, 100.0*float64(count)/float64(maxCount))
	}
	return percentages, nil
}

func monitorLoadRoutine(
	dev model.Device,
	tz float64,
	wg *sync.WaitGroup,
	ch chan monitorDevice,
	errCh chan string,
	data monitorData,
	skey int64,
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
) {
	var md monitorDevice
	md.Device = dev
	md.StatusText = dev.StatusText()

	// Determine the device throughput.
	var err error
	md.Count, md.MaxCount, err = throughput(ctx, &dev)
	if err != nil {
		reportMonitorError(w, r, &data, "could not get throughput: %v", err)
		return
	}
	md.Throughput = int(100.0 * (float64(md.Count) / float64(md.MaxCount)))

	// Set the address variable.
	v, err := model.GetVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".localaddr")
	switch {
	case errors.Is(err, datastore.ErrNoSuchEntity):
		md.Address = "None"
	case err != nil:
		reportMonitorError(w, r, &data, "could not get address variable: %v", err)
		return
	default:
		md.Address = v.Value
	}

	// Set the uptime and sending variable.
	v, err = model.GetVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".uptime")
	switch {
	case errors.Is(err, datastore.ErrNoSuchEntity):
		md.Sending = "black"
	case err != nil:
		reportMonitorError(w, r, &data, "could not get uptime variable: %v", err)
		return
	case time.Since(v.Updated) < time.Duration(2*int(dev.MonitorPeriod))*time.Second:
		md.Sending = "green"
	default:
		md.Sending = "red"
	}
	md.LastReportedTimestamp = v.Updated.Unix()

	md.Uptime, err = secondsToUptime(v)
	if err != nil {
		reportMonitorError(w, r, &data, "failed to parse uptime: %v", err)
		return
	}

	sensors, err := model.GetSensorsV2(ctx, settingsStore, dev.Mac)
	if err != nil {
		reportMonitorError(w, r, &data, "could not get sensors: %v", err)
		return
	}

	for _, sensor := range sensors {
		id := model.ToSID(model.MacDecode(sensor.Mac), sensor.Pin)
		scalar, err := model.GetLatestScalar(ctx, mediaStore, id)
		if err == datastore.ErrNoSuchEntity {
			continue
		} else if err != nil {
			reportMonitorError(w, r, &data, "could not get latest scalar %d: %v", id, err)
			return
		}
		value, err := sensor.Transform(scalar.Value)
		if err != nil {
			errCh <- fmt.Sprintf("could not transform scalar for sensor %s.%s: %v", model.MacDecode(sensor.Mac), sensor.Name, err)
			continue
		}

		sensorData := sensorData{
			Name:   sensor.Name,
			Units:  sensor.Units,
			Pin:    sensor.Pin,
			Scalar: fmt.Sprintf("%.2f", value),
			Date:   time.Unix(scalar.Timestamp, 0).In(fixedTimezone(tz)).Format("Jan 2 15:04:05"),
		}
		md.Sensors = append(md.Sensors, sensorData)
	}
	if strings.Contains(dev.Inputs, "T1") {
		md.HasT1 = true
	}

	ch <- md
	wg.Done()
}

// secondsToUptime converts the uptime variable of a device to a formatted
// string to be rendered on the page.
func secondsToUptime(v *model.Variable) (uptime string, err error) {
	if v == nil || v.Value == "" {
		return "None", nil
	}

	seconds, err := strconv.Atoi(v.Value)
	if err != nil {
		return "", fmt.Errorf("uptime to int error: %v", err)
	}

	days := seconds / secondsToDay
	hours := (seconds / secondsToHour) % hoursToDay
	minutes := (seconds / secondsToMinute) % minutesToHour
	seconds = (seconds) % secondsToMinute

	// Format duration string.
	uptime = fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	return uptime, nil
}

func reportMonitorError(w http.ResponseWriter, r *http.Request, d *monitorData, f string, args ...interface{}) {
	msg := fmt.Sprintf(f, args...)
	log.Print(msg)
	writeTemplate(w, r, "monitor.html", d, msg)
}
