/*
NAME
  Ocean Bench settings handlers.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2019-2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Bench. Ocean Bench is free software: you can
  redistribute it and/or modify it under the terms of the GNU General
  Public License as published by the Free Software Foundation, either
  version 3 of the License, or (at your option) any later version.

  This software is distributed in the hope that it will be useful, but
  WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
  General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt.  If not, see
  http://www.gnu.org/licenses/.
*/

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
	"github.com/ausocean/utils/nmea"
	"github.com/ausocean/utils/sliceutils"
	"golang.org/x/sync/errgroup"
)

var (
	errInvalidEmail = errors.New("invalid email")
	errInvalidName  = errors.New("invalid name")
	errInvalidTask  = errors.New("invalid task")
	errInvalidID    = errors.New("invalid ID")
	errInvalidTime  = errors.New("invalid time")
)

// Device settings:

// setDevicesHandler handles requests to the devices page.
func setDevicesHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	writeDevices(w, r, "")
}

// devTypes are valid device types.
var devTypes = []string{
	model.DevTypeController,
	model.DevTypeCamera,
	model.DevTypeHydrophone,
	model.DevTypeSpeaker,
	model.DevTypeAligner,
	model.DevTypeTest,
}

// devicesData contains data required by the device.html template, and is populated
// by the writeDevices function.
type devicesData struct {
	Timezone   float64
	Mac        string
	Device     *model.Device
	Devices    []model.Device
	Vars       []model.Variable
	Sensors    []model.SensorV2
	Actuators  []model.ActuatorV2
	VarTypes   []model.Variable
	DevTypes   []string
	Quantities []nmea.Quantity
	Funcs      []string
	Formats    []string
	commonData
}

// writeDevices writes the devices page.
// If the query includes sk=auto and a MAC address (ma) param is specified,
// the parent site is automatically selected.
// If msg is not-empty it means the previous call generated an error message.
// The following system variables are used:
//
//   - _<hex>.uptime: uptime for device with given hexadecimal MAC address.
//   - _<hex>.localaddr: local IP address for device with given hexadecimal MAC address.
//   - _type.<var>: type of var
func writeDevices(w http.ResponseWriter, r *http.Request, msg string, args ...interface{}) {
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	skey, _ := profileData(profile)

	data := devicesData{
		commonData: commonData{
			Pages: pages("devices"),
		},
		Mac:        r.FormValue("ma"),
		Device:     &model.Device{Enabled: true},
		Quantities: nmea.DefaultQuantities(),
		Funcs:      model.SensorFuncs(),
		Formats:    model.SensorFormats(),
		DevTypes:   devTypes,
	}

	ctx := r.Context()
	setup(ctx)

	// If a MAC is present, fetch once here and reuse later.
	var siteChanged bool
	if model.IsMacAddress(data.Mac) {
		d, err := model.GetDevice(ctx, settingsStore, model.MacEncode(data.Mac))
		if err != nil {
			reportDevicesError(w, r, data, "get device error for ma: %s, %v", data.Mac, err)
			return
		}
		data.Device = d
		if data.Device.Skey != skey && r.FormValue("sk") == "auto" {
			skey = data.Device.Skey
			siteChanged = true
			log.Printf("site %d auto selected for device %s", skey, data.Mac)
		}
	}

	user, err := model.GetUser(ctx, settingsStore, skey, profile.Email)
	if errors.Is(err, datastore.ErrNoSuchEntity) || (err == nil && user.Perm&model.WritePermission == 0) {
		log.Println("user does not have write permissions")
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Printf("failed to get permission for user: %v", err)
		http.Redirect(w, r, "/", http.StatusInternalServerError)
		return
	}

	// Fetch site and devices concurrently.
	g, gctx := errgroup.WithContext(ctx)

	var site *model.Site
	g.Go(func() error {
		s, err := model.GetSite(gctx, settingsStore, skey)
		if err != nil {
			return fmt.Errorf("get site error: %v", err)
		}
		site = s
		return nil
	})

	g.Go(func() error {
		ds, err := model.GetDevicesBySite(gctx, settingsStore, skey)
		if err != nil {
			return fmt.Errorf("get devices by site error: %v", err)
		}
		data.Devices = ds
		return nil
	})

	if err := g.Wait(); err != nil {
		reportDevicesError(w, r, data, "site or device list error: %v", err)
		return
	}

	if siteChanged {
		err := putProfileData(w, r, fmt.Sprintf("%d:%s", site.Skey, site.Name))
		if err != nil {
			log.Printf("could not put profile data: %v", err)
		}
	}
	data.Timezone = site.Timezone

	if msg != "" {
		reportDevicesError(w, r, data, msg, args...)
		return
	}

	// If no MAC, render the selection page early. Avoid extra calls.
	if !model.IsMacAddress(data.Mac) {
		writeTemplate(w, r, "set/device.html", &data, "")
		return
	}

	// Parallelize per-device lookups.
	var (
		uptimeVar    *model.Variable
		localAddrVar *model.Variable
		varTypes     []model.Variable
		sensors      []model.SensorV2
		actuators    []model.ActuatorV2
	)

	g, gctx2 := errgroup.WithContext(ctx)

	g.Go(func() error {
		v, err := model.GetVariable(gctx2, settingsStore, data.Device.Skey, "_"+data.Device.Hex()+".uptime")
		if err == nil {
			uptimeVar = v
		} else if !errors.Is(err, datastore.ErrNoSuchEntity) {
			return fmt.Errorf("get uptime variable error: %v", err)
		}
		return nil
	})
	g.Go(func() error {
		v, err := model.GetVariable(gctx2, settingsStore, data.Device.Skey, "_"+data.Device.Hex()+".localaddr")
		if err == nil {
			localAddrVar = v
		} else if !errors.Is(err, datastore.ErrNoSuchEntity) {
			return fmt.Errorf("get localaddr variable error: %v", err)
		}
		return nil
	})
	g.Go(func() error {
		vt, err := model.GetVariablesBySite(gctx2, settingsStore, skey, "_type")
		if err == nil || errors.Is(err, datastore.ErrNoSuchEntity) {
			varTypes = vt
			return nil
		}
		return fmt.Errorf("get vartype variable error: %v", err)
	})
	g.Go(func() error {
		ss, err := model.GetSensorsV2(gctx2, settingsStore, data.Device.Mac)
		if err != nil {
			return fmt.Errorf("get sensors error: %v", err)
		}
		sensors = ss
		return nil
	})
	g.Go(func() error {
		aa, err := model.GetActuatorsV2(gctx2, settingsStore, data.Device.Mac)
		if err != nil {
			return fmt.Errorf("get actuators error: %v", err)
		}
		actuators = aa
		return nil
	})

	if err := g.Wait(); err != nil {
		reportDevicesError(w, r, data, "per-device data error: %v", err)
		return
	}

	// Provide uptime and device status information.
	thresh := time.Duration(2*int(data.Device.MonitorPeriod)) * time.Second
	switch {
	case uptimeVar == nil:
		data.Device.SetOther("sending", "black")
	case time.Since(uptimeVar.Updated) < thresh:
		data.Device.SetOther("sending", "green")
		if ut, err := strconv.Atoi(uptimeVar.Value); err == nil {
			data.Device.SetOther("uptime", (time.Duration(ut) * time.Second).String())
		}
	default:
		data.Device.SetOther("sending", "red")
	}

	// Get the local address only if available.
	if localAddrVar != nil {
		data.Device.SetOther("localaddr", localAddrVar.Value)
	}

	data.VarTypes = varTypes
	data.Sensors = sensors
	data.Actuators = actuators

	writeTemplate(w, r, "set/device.html", &data, msg)
}

// reportDevicesError handles error encountered during writing of the devices page.
// Errors are firstly logged, and then written to the device.html template.
func reportDevicesError(w http.ResponseWriter, r *http.Request, d devicesData, f string, args ...interface{}) {
	msg := fmt.Sprintf(f, args...)
	log.Println(msg)
	writeTemplate(w, r, "set/device.html", &d, msg)
}

// editDevicesHandler handles device edit/deletion requests.
func editDevicesHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	skey, _ := profileData(profile)

	ma := r.FormValue("ma")
	dn := r.FormValue("dn")
	task := r.FormValue("task")

	mac := model.MacEncode(ma)
	if mac == 0 {
		writeDevices(w, r, "MAC address missing")
		return
	}

	setup(ctx)
	var dev *model.Device
	if task != "Add" {
		dev, err = model.GetDevice(ctx, settingsStore, mac)
		if err != nil {
			writeDevices(w, r, err.Error())
			return
		}
		if dev.Skey != skey {
			writeDevices(w, r, errPermissionDenied.Error())
			return
		}
	}

	if task == "Delete" {
		model.DeleteDevice(ctx, settingsStore, mac)
		model.DeleteVariables(ctx, settingsStore, skey, dev.Hex())
		http.Redirect(w, r, "/set/devices", http.StatusFound)
		return
	}

	// Update the device.
	// Note that the MAC address is immutable.
	ip := r.FormValue("ip")
	op := r.FormValue("op")
	wi := r.FormValue("wi")
	mp := r.FormValue("mp")
	ap := r.FormValue("ap")
	ct := r.FormValue("ct")
	cv := r.FormValue("cv")
	lt := r.FormValue("lt")
	ln := r.FormValue("ln")
	dk := r.FormValue("dk")
	de := r.FormValue("de")

	if task == "Add" {
		if dn == "" {
			writeDevices(w, r, "Device ID missing")
			return
		}
		// Generate an 8-digit random number for the device key.
		rand.Seed(time.Now().UnixNano())
		dkey := 10000000 + rand.Intn(100000000)
		dev = &model.Device{Skey: skey, Mac: mac, Name: dn, Dkey: int64(dkey), Status: 1, Enabled: true}
	} else {
		i, err := strconv.ParseInt(dk, 10, 64)
		if err == nil {
			dev.Dkey = i
		}
	}

	dev.Name = dn
	dev.Inputs = ip
	dev.Outputs = op
	dev.Wifi = wi
	i, err := strconv.ParseInt(mp, 10, 64)
	if err == nil {
		dev.MonitorPeriod = i
	}
	i, err = strconv.ParseInt(ap, 10, 64)
	if err == nil {
		dev.ActPeriod = i
	}
	dev.Type = ct
	dev.Version = cv
	f, err := strconv.ParseFloat(lt, 64)
	if err == nil {
		dev.Latitude = f
	}
	f, err = strconv.ParseFloat(ln, 64)
	if err == nil {
		dev.Longitude = f
	}
	if de == "" {
		dev.Enabled = false
	} else {
		dev.Enabled = true
	}
	switch task {
	case "Update":
		dev.Status = model.DeviceStatusUpdate
	case "Reboot":
		dev.Status = model.DeviceStatusReboot
	case "Shutdown":
		dev.Status = model.DeviceStatusShutdown
	case "Debug":
		dev.Status = model.DeviceStatusDebug
	case "Upgrade":
		dev.Status = model.DeviceStatusUpgrade
	case "Alarm":
		dev.Status = model.DeviceStatusAlarm
	case "Test":
		dev.Status = model.DeviceStatusTest
	}

	err = model.PutDevice(ctx, settingsStore, dev)
	if err != nil {
		writeDevices(w, r, err.Error())
		return
	}

	http.Redirect(w, r, "/set/devices?ma="+ma, http.StatusFound)
}

// calibrateDevicesHandler handles calibration of controller-type
// device voltages.
//
// Query params:
//   - ma:  MAC address
//   - vb:  Battery Voltage
//   - vnw: Network Voltage
//   - vp1: Power 1 Voltage
//   - vp2: Power 2 Voltage
//   - vp3: Power 3 Voltage
//   - va:  Alarm Voltage
//   - vr:  Alarm Recovery Voltage
//
// NOTE: All voltages are parsed in Volts.
func calibrateDevicesHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	if r.Method != http.MethodPost {
		log.Println("calibration request must use POST action")
		http.Redirect(w, r, "/set/devices?ma="+r.FormValue("ma"), http.StatusSeeOther)
		return
	}
	ctx := context.Background()
	p, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}

	mac := r.FormValue("ma")
	vb, err := strconv.ParseFloat(r.FormValue("vb"), 64)
	if err != nil && vb != 0 {
		writeDevices(w, r, "unable to parse battery voltage: %v", err)
		return
	}
	vnw, err := strconv.ParseFloat(r.FormValue("vnw"), 64)
	if err != nil && vnw != 0 {
		writeDevices(w, r, "unable to parse network voltage: %v", err)
		return
	}
	vp1, err := strconv.ParseFloat(r.FormValue("vp1"), 64)
	if err != nil && vp1 != 0 {
		writeDevices(w, r, "unable to parse power 1 voltage: %v", err)
		return
	}
	vp2, err := strconv.ParseFloat(r.FormValue("vp2"), 64)
	if err != nil && vp2 != 0 {
		writeDevices(w, r, "unable to parse power 2 voltage: %v", err)
		return
	}
	vp3, err := strconv.ParseFloat(r.FormValue("vp3"), 64)
	if err != nil && vp3 != 0 {
		writeDevices(w, r, "unable to parse power 3 voltage: %v", err)
		return
	}
	va, err := strconv.ParseFloat(r.FormValue("va"), 64)
	if err != nil && va != 0 {
		writeDevices(w, r, "unable to parse alarm voltage: %v", err)
		return
	}
	vr, err := strconv.ParseFloat(r.FormValue("vr"), 64)
	if err != nil && vr != 0 {
		writeDevices(w, r, "unable to parse alarm recovery voltage: %v", err)
		return
	}

	device, err := model.GetDevice(ctx, settingsStore, model.MacEncode(mac))
	if err != nil {
		writeDevices(w, r, "unable to get device to calibrate (%s): %v", mac, err)
		return
	}

	// Names of the voltage sensors to calibrate.
	var voltageSensors = []string{
		model.NameBatterySensor,
		model.NameNWVoltage,
		model.NameP1Voltage,
		model.NameP2Voltage,
		model.NameP3Voltage,
	}

	// Load the most recent sensor values.
	sensors, err := model.GetSensorsV2(ctx, settingsStore, model.MacEncode(mac))
	if err != nil {
		writeDevices(w, r, "unable to get sensors (%s): %v", mac, err)
		return
	}

	// Calibrate each of the voltage sensors.
	var msgs []string
	for _, sensor := range sensors {
		if !sliceutils.ContainsString(voltageSensors, sensor.Name) {
			continue
		}

		scalar, err := model.GetLatestScalar(ctx, mediaStore, model.ToSID(mac, sensor.Pin))
		if err != nil {
			msgs = append(msgs, fmt.Sprintf("unable to get latest scalar for %s: %v", sensor.Name, err))
			continue
		}
		reportedTime := time.Unix(scalar.Timestamp, 0)

		// Check if the scalar was recently reported (last 2 monitor periods).
		if reportedTime.Before(time.Now().Add(-2 * time.Duration(device.MonitorPeriod) * time.Second)) {
			msgs = append(msgs, fmt.Sprintf("scalar (%s) is out of date (timestamp: %s)(current time: %s)",
				sensor.Name, reportedTime.Format(time.ANSIC), time.Now().Format(time.ANSIC)))

			// Continue to calibrate other sensors that are still reporting.
			continue
		}

		var actual float64
		switch sensor.Name {
		case model.NameBatterySensor:
			actual = vb
		case model.NameNWVoltage:
			actual = vnw
		case model.NameP1Voltage:
			actual = vp1
		case model.NameP2Voltage:
			actual = vp2
		case model.NameP3Voltage:
			actual = vp3
		default:
			// This shouldn't be possible with the ContainsString check.
			log.Panicln("cannot handle unexpected sensor name")
		}

		// This most likely means the field was left blank. This is not a
		// meaningful way of calibrating the system.
		if actual == 0 {
			continue
		}

		// If the scalar value is zero, the division calculation will result in a
		// divide by zero calculation. Return a message to the user, and do not
		// calibrate this sensor.
		const nearZeroValue = 0.05
		if scalar.Value <= nearZeroValue {
			msgs = append(msgs, "cannot calibrate sensor reading 0")
			continue
		}

		// Calculate the new scale value.
		scaleFactor := actual / scalar.Value
		sensor.Args = strconv.FormatFloat(scaleFactor, 'f', -1, 64)
		log.Printf("calibrated sensor value for %s: %s", sensor.Name, sensor.Args)

		// Save the sensor with the new scale factor.
		model.PutSensorV2(ctx, settingsStore, &sensor)

		if sensor.Name != model.NameBatterySensor {
			continue
		}

		// Calibrate the alarm voltage and alarm recovery voltage variables.
		skey, _ := profileData(p)
		if va > 0 {
			err := model.PutVariable(ctx, settingsStore, skey, model.NameAlarmVoltage, fmt.Sprintf("%d", int(va/scaleFactor)))
			if err != nil {
				msgs = append(msgs, fmt.Sprintf("unable to set alarm voltage: %v", err))
			}
		}
		if vr > 0 {
			err := model.PutVariable(ctx, settingsStore, skey, model.NameAlarmRecoveryVoltage, fmt.Sprintf("%d", int(vr/scaleFactor)))
			if err != nil {
				msgs = append(msgs, fmt.Sprintf("unable to set alarm recovery voltage: %v", err))
			}
		}

	}

	msg := strings.Join(msgs, ",")
	if msg != "" {
		writeDevices(w, r, "errors during calibration: %s", msg)
		return
	}
	writeDevices(w, r, "Device Calibrated")
}

// editVarHandler handles per-device variable update/deletion requests.
// Query params:
//
//   - ma: MAC address
//   - vn: variable name
//   - vv: variable value
//   - vd: variable delete
func editVarHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	skey, _ := profileData(profile)

	ma := r.FormValue("ma")
	vn := strings.TrimSpace(r.FormValue("vn"))
	vv := strings.Join(r.Form["vv"], ",")
	vd := r.FormValue("vd")

	mac := model.MacEncode(ma)
	if mac == 0 {
		writeDevices(w, r, "MAC address missing")
		return
	}

	if vn == "" {
		writeDevices(w, r, "Name missing")
		return
	}

	setup(ctx)
	dev, err := model.GetDevice(ctx, settingsStore, mac)
	if err != nil {
		writeDevices(w, r, err.Error())
		return
	}

	if vd == "true" {
		err = model.DeleteVariable(ctx, settingsStore, skey, dev.Hex()+"."+vn)
	} else {
		err = model.PutVariable(ctx, settingsStore, skey, dev.Hex()+"."+vn, vv)
	}

	if err != nil {
		writeDevices(w, r, err.Error())
		return
	}

	http.Redirect(w, r, "/set/devices?ma="+ma, http.StatusFound)
}

// editSensorHandler handles requests to /set/device/edit/sensor.
func editSensorHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("edit sensor handler")
	logRequest(r)
	ctx := r.Context()
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	_ = profile // ToDo: Check for write access.

	ma := r.FormValue("ma")
	mac := model.MacEncode(ma)
	if mac == 0 {
		writeDevices(w, r, "MAC address missing")
		return
	}
	pin := r.FormValue("pin")
	if pin == "" {
		writeDevices(w, r, "pin missing")
		return
	}

	formSensor := model.SensorV2{
		Name:     r.FormValue("name"),
		Mac:      mac,
		Pin:      pin,
		Quantity: r.FormValue("sqty"),
		Func:     r.FormValue("sfunc"),
		Args:     r.FormValue("sargs"),
		Units:    r.FormValue("sunits"),
		Format:   r.FormValue("sfmt"),
	}

	setup(ctx)
	if r.FormValue("delete") == "true" {
		log.Printf("deleting sensor %d.%s", mac, pin)
		err := model.DeleteSensorV2(ctx, settingsStore, mac, pin)
		if err != nil {
			writeDevices(w, r, "delete sensor error: %v", err)
			return
		}
		http.Redirect(w, r, "/set/devices?ma="+ma, http.StatusFound)
		return
	}

	if formSensor.Name == "" {
		writeDevices(w, r, "sensor name missing")
		return
	}
	if formSensor.Func == "" {
		writeDevices(w, r, "sensor func missing")
		return
	}

	log.Printf("putting sensor: %v", formSensor)
	err = model.PutSensorV2(ctx, settingsStore, &formSensor)
	if err != nil {
		writeDevices(w, r, "put sensor error: %v", err)
		return
	}

	http.Redirect(w, r, "/set/devices?ma="+ma, http.StatusFound)
}

// editActuatorHandler handles requests to /set/device/edit/actuator.
func editActuatorHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	_ = profile // ToDo: Check for write access.

	ma := r.FormValue("ma")
	mac := model.MacEncode(ma)
	if mac == 0 {
		writeDevices(w, r, "MAC address missing")
		return
	}
	pin := r.FormValue("pin")
	if pin == "" {
		writeDevices(w, r, "actuator pin missing")
		return
	}

	actuatorForm := model.ActuatorV2{
		Name: r.FormValue("name"),
		Mac:  mac,
		Pin:  pin,
		Var:  r.FormValue("var"),
	}

	setup(ctx)
	if r.FormValue("delete") == "true" {
		log.Printf("deleting actuator, %d.%s", mac, pin)
		err := model.DeleteActuatorV2(ctx, settingsStore, mac, pin)
		if err != nil {
			writeDevices(w, r, "delete actuator error: %v", err)
			return
		}
		http.Redirect(w, r, "set/devices?ma="+ma, http.StatusFound)
		return
	}

	if actuatorForm.Name == "" {
		writeDevices(w, r, "actuator name missing")
		return
	}
	if actuatorForm.Var == "" {
		writeDevices(w, r, "actuator var missing")
		return
	}

	log.Printf("putting actuator: %v", actuatorForm)
	err = model.PutActuatorV2(ctx, settingsStore, &actuatorForm)
	if err != nil {
		writeDevices(w, r, "put actuator error: %v", err)
		return
	}

	http.Redirect(w, r, "/set/devices?ma="+ma, http.StatusFound)
}

// Cron settings:

// setCronsHandler handles requests to the crons page.
func setCronsHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	writeCrons(w, r, "")
}

type dataFields struct {
	Timezone float64
	Crons    []model.Cron
	Actions  []string
	commonData
}

// writeCrons writes the crons page.
// If msg is not-empty it means the previous call generated an error message.
func writeCrons(w http.ResponseWriter, r *http.Request, msg string) {
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	if r.URL.Path != "/set/crons/" {
		// Redirect all requests to the cron base path to clear any actions.
		http.Redirect(w, r, "/set/crons/", http.StatusFound)
		return
	}

	ctx := r.Context()
	setup(ctx)

	skey, _ := profileData(profile)

	user, err := model.GetUser(ctx, settingsStore, skey, profile.Email)
	if errors.Is(err, datastore.ErrNoSuchEntity) || user.Perm&model.WritePermission == 0 {
		log.Println("user does not have write permissions")
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Printf("failed to get permission for user: %v", err)
		http.Redirect(w, r, "/", http.StatusInternalServerError)
		return
	}

	site, err := model.GetSite(ctx, settingsStore, skey)
	if err != nil {
		writeTemplate(w, r, "set/cron.html", &dataFields{commonData: commonData{}}, fmt.Sprintf("could not get site: %v", err))
		return
	}

	crons, err := model.GetCronsBySite(ctx, settingsStore, skey)
	if err != nil {
		writeTemplate(w, r, "set/cron.html", &dataFields{commonData: commonData{}}, fmt.Sprintf("could not get crons by site: %v", err))
		return
	}

	data := dataFields{
		commonData: commonData{
			Pages: pages("crons"),
			Msg:   msg,
		},
		Timezone: site.Timezone,
		Crons:    crons,
		Actions:  []string{"set", "del", "call", "rpc", "email"},
	}

	writeTemplate(w, r, "set/cron.html", &data, msg)
}

// editCronsHandler handles cron edit/deletion requests.
// Query params:
//
//   - ci: cron ID
//   - ct: cron time
//   - ca: cron action
//   - cv: cron variable
//   - cd: cron data (variable value)
//   - ce: cron enabled
func editCronsHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := r.Context()
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	skey, _ := profileData(profile)

	id := r.FormValue("ci")
	ct := strings.Trim(r.FormValue("ct"), " ")
	ca := strings.Trim(r.FormValue("ca"), " ")
	cv := strings.Trim(r.FormValue("cv"), " ")
	cd := r.FormValue("cd")
	ce := r.FormValue("ce")
	task := r.FormValue("task")

	if id == "" {
		writeError(w, errInvalidID)
		return
	}

	if task == "Delete" {
		err := model.DeleteCron(ctx, settingsStore, skey, id)
		if err != nil {
			writeCrons(w, r, fmt.Sprintf("could not delete crons: %v", err))
			return
		}

		err = cronScheduler.Set(&model.Cron{Skey: skey, ID: id, Enabled: false})
		if err != nil {
			writeCrons(w, r, fmt.Sprintf("could not unset cron: %v", err))
			return
		}

		writeCrons(w, r, "")
		return
	}

	site, err := model.GetSite(ctx, settingsStore, skey)
	if err != nil {
		writeError(w, fmt.Errorf("could not get site: %v", err))
		return
	}

	c := model.Cron{Skey: skey, ID: id, Action: ca, Var: cv, Data: cd, Enabled: ce != ""}
	err = c.ParseTime(ct, site.Timezone)
	if err != nil {
		writeError(w, fmt.Errorf("could not parse time: %v", err))
		return
	}

	err = model.PutCron(ctx, settingsStore, &c)
	if err != nil {
		writeError(w, fmt.Errorf("could not put cron in datastore: %v", err))
		return
	}

	err = cronScheduler.Set(&c)
	if err != nil {
		writeError(w, fmt.Errorf("could not schedule cron: %v", err))
		return
	}
}
