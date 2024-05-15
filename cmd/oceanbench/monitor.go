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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"bitbucket.org/ausocean/iotsvc/gauth"
	"bitbucket.org/ausocean/iotsvc/iotds"
)

const (
	secondsToDay    = 86400
	secondsToHour   = 3600
	secondsToMinute = 60
	hoursToDay      = 24
	minutesToHour   = 60
	countPeriod     = 60 * time.Minute
)

// sensorData holds the relevant information for each sensor.
type sensorData struct {
	Name   string
	Units  string
	Scalar string
	Date   string
}

// monitorDevice holds the relevant information for each device.
type monitorDevice struct {
	Device     iotds.Device
	Address    string
	Sending    string
	StatusText string
	Uptime     string
	Count      int // Number of scalars sent in the monitor period.
	MaxCount   int // Max number of scalars that could be sent.
	Throughput int // Percentage of successful scalars.
	Sensors    []sensorData
}

// monitorData holds the relevant information for the monitor page
type monitorData struct {
	Ma        string
	Devices   []monitorDevice
	WritePerm bool
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
	setup(ctx)
	data.Users, err = getUsersForSiteMenu(w, r, ctx, profile, data)
	if err != nil {
		writeTemplate(w, r, "monitor.html", &data, fmt.Sprintf("could not populate site menu: %v", err.Error()))
		return
	}

	skey, _ := profileData(profile)

	// Check if user has write permissions to link to devices page.
	user, err := iotds.GetUser(ctx, settingsStore, skey, profile.Email)
	if err == nil && user.Perm&iotds.WritePermission != 0 {
		data.WritePerm = true
	} else if err != nil && err != iotds.ErrNoSuchEntity {
		log.Println("failed getting user permissions", err)
	}

	devices, err := iotds.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		reportMonitorError(w, r, &data, "could not get devices: %v", err)
		return
	}

	ch := make(chan monitorDevice, len(devices))

	site, err := iotds.GetSite(ctx, settingsStore, skey)
	if err != nil {
		reportMonitorError(w, r, &data, "could not get devices: %v", err)
		return
	}

	monitorDevices := make([]monitorDevice, len(devices))
	var wg sync.WaitGroup
	for _, device := range devices {
		wg.Add(1)
		go monitorLoadRoutine(device, site.Timezone, &wg, ch, data, skey, ctx, w, r)
	}
	wg.Wait()
	close(ch)
	i := 0
	for device := range ch {
		monitorDevices[i] = device
		i++
	}
	sort.Slice(monitorDevices, func(i, j int) bool {
		return monitorDevices[i].Device.Name < monitorDevices[j].Device.Name
	})
	data.Devices = monitorDevices
	writeTemplate(w, r, "monitor.html", &data, "")
}

// throughput determines the number of scalars received within the count
// period for a device, as well as the maximum number of scalars expected.
// The first A, D or X pin that provides count data is used.
func throughput(ctx context.Context, device iotds.Device) (count, maxCount int, err error) {
	if device.Inputs != "" && device.MonitorPeriod != 0 {
		pins := strings.Split(device.Inputs, ",")
		monitorDuration := time.Duration(device.MonitorPeriod) * time.Second
		maxCount = int(countPeriod / monitorDuration)

		start := time.Now().Add(-countPeriod).Unix()
		for _, pin := range pins {
			if count != 0 || (pin[0] != 'A' && pin[0] != 'D' && pin[0] != 'X') {
				continue
			}
			sid := iotds.ToSID(iotds.MacDecode(device.Mac), pin)
			keys, err := iotds.GetScalarKeys(ctx, mediaStore, sid, []int64{start, -1})
			if err != nil {
				return 0, 0, fmt.Errorf("could not get scalar keys: %v", err)
			}
			count = len(keys)
			break
		}
	}
	return count, maxCount, nil
}

func monitorLoadRoutine(
	dev iotds.Device,
	tz float64,
	wg *sync.WaitGroup,
	ch chan monitorDevice,
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
	md.Count, md.MaxCount, err = throughput(ctx, dev)
	if err != nil {
		reportMonitorError(w, r, &data, "could not get throughput: %v", err)
		return
	}
	md.Throughput = int(100.0 * (float64(md.Count) / float64(md.MaxCount)))

	// Set the address variable.
	v, err := iotds.GetVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".localaddr")
	switch {
	case errors.Is(err, iotds.ErrNoSuchEntity):
		md.Address = "None"
	case err != nil:
		reportMonitorError(w, r, &data, "could not get address variable: %v", err)
		return
	default:
		md.Address = v.Value
	}

	// Set the uptime and sending variable.
	v, err = iotds.GetVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".uptime")
	switch {
	case errors.Is(err, iotds.ErrNoSuchEntity):
		md.Sending = "black"
	case err != nil:
		reportMonitorError(w, r, &data, "could not get uptime variable: %v", err)
		return
	case time.Since(v.Updated) < time.Duration(2*int(dev.MonitorPeriod))*time.Second:
		md.Sending = "green"
	default:
		md.Sending = "red"
	}

	md.Uptime, err = secondsToUptime(v)
	if err != nil {
		reportMonitorError(w, r, &data, "failed to parse uptime: %v", err)
		return
	}

	sensors, err := iotds.GetSensorsV2(ctx, settingsStore, dev.Mac)
	if err != nil {
		reportMonitorError(w, r, &data, "could not get sensors: %v", err)
		return
	}

	for _, sensor := range sensors {
		id := iotds.ToSID(iotds.MacDecode(sensor.Mac), sensor.Pin)
		scalar, err := getLatestScalar(ctx, mediaStore, id)
		if err == iotds.ErrNoSuchEntity {
			continue
		} else if err != nil {
			reportMonitorError(w, r, &data, "could not get latest scalar %d: %v", id, err)
			return
		}
		value, err := sensor.Transform(scalar.Value)
		if err != nil {
			reportMonitorError(w, r, &data, "could not transform scalar for sensor %d.%s: %v", sensor.Mac, sensor.Pin, err)
			return
		}

		sensorData := sensorData{
			Name:   sensor.Name,
			Units:  sensor.Units,
			Scalar: fmt.Sprintf("%.2f", value),
			Date:   time.Unix(scalar.Timestamp, 0).In(fixedTimezone(tz)).Format("Jan 2 15:04:05"),
		}
		md.Sensors = append(md.Sensors, sensorData)
	}
	ch <- md
	wg.Done()
}

// secondsToUptime converts the uptime variable of a device to a formatted
// string to be rendered on the page.
func secondsToUptime(v *iotds.Variable) (uptime string, err error) {
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

// getLatestScalar finds the most recent scalar within the countPeriod.
func getLatestScalar(ctx context.Context, store iotds.Store, id int64) (*iotds.Scalar, error) {
	start := time.Now().Add(-countPeriod).Unix()
	keys, err := iotds.GetScalarKeys(ctx, mediaStore, id, []int64{start, -1})
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, iotds.ErrNoSuchEntity
	}
	_, ts, _ := iotds.SplitIDKey(keys[len(keys)-1].ID)
	return iotds.GetScalar(ctx, store, id, ts)
}
