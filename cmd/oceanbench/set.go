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

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/system/camera"
	"github.com/ausocean/cloud/system/controller"
	"github.com/ausocean/utils/nmea"
	"github.com/ausocean/utils/sliceutils"
	"github.com/gofiber/fiber/v2"
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
func setDevicesHandler(c *fiber.Ctx) error {
	logRequest(c)
	return writeDevices(c, "")
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
	VarTypes   []model.VarType
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
func writeDevices(c *fiber.Ctx, msg string, args ...interface{}) error {
	profile, err := getProfile(c)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		return c.Redirect("/", fiber.StatusUnauthorized)
	}
	skey, err := getCurrentSkey(c, profile)
	if err != nil {
		log.Printf("unable to get current skey, redirecting: %v", err)
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	data := devicesData{
		commonData: commonData{
			Pages: pages(c, "devices"),
		},
		Mac:        c.FormValue("ma"),
		Device:     &model.Device{Enabled: true},
		Quantities: nmea.DefaultQuantities(),
		Funcs:      model.SensorFuncs(),
		Formats:    model.SensorFormats(),
		DevTypes:   devTypes,
	}

	ctx := c.UserContext()
	setup(ctx)

	// If a MAC is present, fetch once here and reuse later.
	var siteChanged bool
	if model.IsMacAddress(data.Mac) {
		d, err := model.GetDevice(ctx, settingsStore, model.MacEncode(data.Mac))
		if err != nil {
			reportDevicesError(c, data, "get device error for ma: %s, %v", data.Mac, err)
			return nil
		}
		data.Device = d
		if data.Device.Skey != skey && c.FormValue("sk") == "auto" {
			skey = data.Device.Skey
			siteChanged = true
			log.Printf("site %d auto selected for device %s", skey, data.Mac)
		}
	}

	user, err := model.GetUser(ctx, settingsStore, skey, profile.Email)
	if errors.Is(err, datastore.ErrNoSuchEntity) || (err == nil && user.Perm&model.WritePermission == 0) {
		log.Println("user does not have write permissions")
		return c.Redirect("/", fiber.StatusUnauthorized)
	} else if err != nil {
		log.Printf("failed to get permission for user: %v", err)
		return c.Redirect("/", fiber.StatusInternalServerError)
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
		return reportDevicesError(c, data, "site or device list error: %v", err)
	}

	if siteChanged {
		err := putProfileData(c, fmt.Sprintf("%d:%s", site.Skey, site.Name))
		if err != nil {
			log.Printf("could not put profile data: %v", err)
		}
	}
	data.Timezone = site.Timezone

	if msg != "" {
		return reportDevicesError(c, data, msg, args...)
	}

	// If no MAC, render the selection page early. Avoid extra calls.
	if !model.IsMacAddress(data.Mac) {
		return writeTemplate(c, "set/device.html", &data, "")
	}

	// Parallelize per-device lookups.
	var (
		uptimeVar    *model.Variable
		localAddrVar *model.Variable
		varTypes     []model.VarType
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
		switch data.Device.Type {
		case model.DevTypeCamera:
			varTypes = camera.VarTypes()
		case model.DevTypeController:
			varTypes = controller.VarTypes()
		case model.DevTypeAligner:
			fallthrough
		case model.DevTypeSpeaker:
			fallthrough
		case model.DevTypeTest:
			fallthrough
		case model.DevTypeHydrophone:
			fallthrough // This does not need to error.
		default:
			return nil
		}
		return nil
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
		return reportDevicesError(c, data, "per-device data error: %v", err)
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

	return writeTemplate(c, "set/device.html", &data, msg)
}

// reportDevicesError handles error encountered during writing of the devices page.
// Errors are firstly logged, and then written to the device.html template.
func reportDevicesError(c *fiber.Ctx, d devicesData, f string, args ...interface{}) error {
	msg := fmt.Sprintf(f, args...)
	log.Println(msg)
	return writeTemplate(c, "set/device.html", &d, msg)
}

// editDevicesHandler handles device edit/deletion requests.
func editDevicesHandler(c *fiber.Ctx) error {
	logRequest(c)
	ctx := c.UserContext()
	profile, err := getProfile(c)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		return c.Redirect("/", fiber.StatusUnauthorized)
	}
	skey, err := getCurrentSkey(c, profile)
	if err != nil {
		log.Printf("unable to get current skey, redirecting: %v", err)
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	ma := c.FormValue("ma")
	dn := c.FormValue("dn")
	task := c.FormValue("task")

	mac := model.MacEncode(ma)
	if mac == 0 {
		return writeDevices(c, "MAC address missing")
	}

	setup(ctx)
	var dev *model.Device
	if task != "Add" {
		dev, err = model.GetDevice(ctx, settingsStore, mac)
		if err != nil {
			return writeDevices(c, "%s", err.Error())
		}
		if dev.Skey != skey {
			return writeDevices(c, "%s", errPermissionDenied.Error())
		}
	}

	if task == "Delete" {
		model.DeleteDevice(ctx, settingsStore, mac)
		model.DeleteVariables(ctx, settingsStore, skey, dev.Hex())
		return c.Redirect(fmt.Sprintf("/%d/set/devices", skey), fiber.StatusFound)
	}

	// Update the device.
	// Note that the MAC address is immutable.
	ip := c.FormValue("ip")
	op := c.FormValue("op")
	wi := c.FormValue("wi")
	mp := c.FormValue("mp")
	ap := c.FormValue("ap")
	ct := c.FormValue("ct")
	cv := c.FormValue("cv")
	lt := c.FormValue("lt")
	ln := c.FormValue("ln")
	dk := c.FormValue("dk")
	de := c.FormValue("de")

	if task == "Add" {
		if dn == "" {
			return writeDevices(c, "Device ID missing")
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
		return writeDevices(c, "%s", err.Error())
	}

	return c.Redirect(fmt.Sprintf("/%d/set/devices?ma=%s", skey, ma), fiber.StatusFound)
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
func calibrateDevicesHandler(c *fiber.Ctx) error {
	logRequest(c)

	ctx := context.Background()
	p, err := getProfile(c)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		return c.Redirect("/", fiber.StatusUnauthorized)
	}

	skey, err := getCurrentSkey(c, p)
	if err != nil {
		log.Printf("unable to get current skey, redirecting: %v", err)
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	if c.Method() != http.MethodPost {
		log.Println("calibration request must use POST action")
		return c.Redirect(fmt.Sprintf("/%d/set/devices?ma=%s", skey, c.FormValue("ma")), fiber.StatusSeeOther)
	}

	mac := c.FormValue("ma")
	vb, err := strconv.ParseFloat(c.FormValue("vb"), 64)
	if err != nil && vb != 0 {
		return writeDevices(c, "unable to parse battery voltage: %v", err)
	}
	vnw, err := strconv.ParseFloat(c.FormValue("vnw"), 64)
	if err != nil && vnw != 0 {
		return writeDevices(c, "unable to parse network voltage: %v", err)
	}
	vp1, err := strconv.ParseFloat(c.FormValue("vp1"), 64)
	if err != nil && vp1 != 0 {
		return writeDevices(c, "unable to parse power 1 voltage: %v", err)
	}
	vp2, err := strconv.ParseFloat(c.FormValue("vp2"), 64)
	if err != nil && vp2 != 0 {
		return writeDevices(c, "unable to parse power 2 voltage: %v", err)
	}
	vp3, err := strconv.ParseFloat(c.FormValue("vp3"), 64)
	if err != nil && vp3 != 0 {
		return writeDevices(c, "unable to parse power 3 voltage: %v", err)
	}
	va, err := strconv.ParseFloat(c.FormValue("va"), 64)
	if err != nil && va != 0 {
		return writeDevices(c, "unable to parse alarm voltage: %v", err)
	}
	vr, err := strconv.ParseFloat(c.FormValue("vr"), 64)
	if err != nil && vr != 0 {
		return writeDevices(c, "unable to parse alarm recovery voltage: %v", err)
	}

	device, err := model.GetDevice(ctx, settingsStore, model.MacEncode(mac))
	if err != nil {
		return writeDevices(c, "unable to get device to calibrate (%s): %v", mac, err)
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
		return writeDevices(c, "unable to get sensors (%s): %v", mac, err)
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
		skey, _ := requestSiteData(c, p)
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
		return writeDevices(c, "errors during calibration: %s", msg)
	}
	return writeDevices(c, "Device Calibrated")
}

// editSensorHandler handles requests to /set/device/edit/sensor.
func editSensorHandler(c *fiber.Ctx) error {
	log.Println("edit sensor handler")
	logRequest(c)
	ctx := c.UserContext()
	profile, err := getProfile(c)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		return c.Redirect("/", fiber.StatusUnauthorized)
	}
	skey, err := getCurrentSkey(c, profile)
	if err != nil {
		log.Printf("unable to get current skey, redirecting: %v", err)
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	_ = profile // ToDo: Check for write access.

	ma := c.FormValue("ma")
	mac := model.MacEncode(ma)
	if mac == 0 {
		return writeDevices(c, "MAC address missing")
	}
	pin := c.FormValue("pin")
	if pin == "" {
		return writeDevices(c, "pin missing")
	}

	formSensor := model.SensorV2{
		Name:     c.FormValue("name"),
		Mac:      mac,
		Pin:      pin,
		Quantity: c.FormValue("sqty"),
		Func:     c.FormValue("sfunc"),
		Args:     c.FormValue("sargs"),
		Units:    c.FormValue("sunits"),
		Format:   c.FormValue("sfmt"),
	}

	setup(ctx)
	if c.FormValue("delete") == "true" {
		log.Printf("deleting sensor %d.%s", mac, pin)
		err := model.DeleteSensorV2(ctx, settingsStore, mac, pin)
		if err != nil {
			return writeDevices(c, "delete sensor error: %v", err)
		}
		return c.Redirect(fmt.Sprintf("/%d/set/devices?ma=%s", skey, ma), fiber.StatusFound)
	}

	if formSensor.Name == "" {
		return writeDevices(c, "sensor name missing")
	}
	if formSensor.Func == "" {
		return writeDevices(c, "sensor func missing")
	}

	log.Printf("putting sensor: %v", formSensor)
	err = model.PutSensorV2(ctx, settingsStore, &formSensor)
	if err != nil {
		return writeDevices(c, "put sensor error: %v", err)
	}

	return c.Redirect(fmt.Sprintf("/%d/set/devices?ma=%s", skey, ma), fiber.StatusFound)
}

// editActuatorHandler handles requests to /set/device/edit/actuator.
func editActuatorHandler(c *fiber.Ctx) error {
	logRequest(c)
	ctx := c.Context()
	profile, err := getProfile(c)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		return c.Redirect("/", fiber.StatusUnauthorized)
	}

	skey, err := getCurrentSkey(c, profile)
	if err != nil {
		log.Printf("unable to get current skey, redirecting: %v", err)
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	_ = profile // ToDo: Check for write access.

	ma := c.FormValue("ma")
	mac := model.MacEncode(ma)
	if mac == 0 {
		return writeDevices(c, "MAC address missing")
	}
	pin := c.FormValue("pin")
	if pin == "" {
		return writeDevices(c, "actuator pin missing")
	}

	actuatorForm := model.ActuatorV2{
		Name: c.FormValue("name"),
		Mac:  mac,
		Pin:  pin,
		Var:  c.FormValue("var"),
	}

	setup(ctx)
	if c.FormValue("delete") == "true" {
		log.Printf("deleting actuator, %d.%s", mac, pin)
		err := model.DeleteActuatorV2(ctx, settingsStore, mac, pin)
		if err != nil {
			return writeDevices(c, "delete actuator error: %v", err)
		}
		return c.Redirect("set/devices?ma="+ma, fiber.StatusFound)
	}

	if actuatorForm.Name == "" {
		return writeDevices(c, "actuator name missing")
	}
	if actuatorForm.Var == "" {
		return writeDevices(c, "actuator var missing")
	}

	log.Printf("putting actuator: %v", actuatorForm)
	err = model.PutActuatorV2(ctx, settingsStore, &actuatorForm)
	if err != nil {
		return writeDevices(c, "put actuator error: %v", err)
	}

	return c.Redirect(fmt.Sprintf("/%d/set/devices?ma=%s", skey, ma), fiber.StatusFound)
}

// Cron settings:

// setCronsHandler handles requests to the crons page.
func setCronsHandler(c *fiber.Ctx) error {
	logRequest(c)
	return writeCrons(c, "")
}

type dataFields struct {
	Timezone float64
	Crons    []model.Cron
	Actions  []string
	commonData
}

// writeCrons writes the
// If msg is not-empty it means the previous call generated an error message.
func writeCrons(c *fiber.Ctx, msg string) error {
	profile, err := getProfile(c)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		return c.Redirect("/", fiber.StatusUnauthorized)
	}
	if c.Path() != "/set/crons/" {
		// Redirect all requests to the cron base path to clear any actions.
		return c.Redirect("/set/crons/", fiber.StatusFound)
	}

	ctx := c.UserContext()
	setup(ctx)

	skey, _ := requestSiteData(c, profile)

	user, err := model.GetUser(ctx, settingsStore, skey, profile.Email)
	if errors.Is(err, datastore.ErrNoSuchEntity) || user.Perm&model.WritePermission == 0 {
		log.Println("user does not have write permissions")
		return c.Redirect("/", fiber.StatusUnauthorized)
	} else if err != nil {
		log.Printf("failed to get permission for user: %v", err)
		return c.Redirect("/", fiber.StatusInternalServerError)
	}

	site, err := model.GetSite(ctx, settingsStore, skey)
	if err != nil {
		writeTemplate(c, "set/cron.html", &dataFields{commonData: commonData{}}, fmt.Sprintf("could not get site: %v", err))
	}

	crons, err := model.GetCronsBySite(ctx, settingsStore, skey)
	if err != nil {
		writeTemplate(c, "set/cron.html", &dataFields{commonData: commonData{}}, fmt.Sprintf("could not get crons by site: %v", err))
	}

	data := dataFields{
		commonData: commonData{
			Pages: pages(c, "crons"),
			Msg:   msg,
		},
		Timezone: site.Timezone,
		Crons:    crons,
		Actions:  []string{"set", "del", "call", "rpc", "email"},
	}

	writeTemplate(c, "set/cron.html", &data, msg)
	return nil
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
func editCronsHandler(c *fiber.Ctx) error {
	logRequest(c)
	ctx := c.Context()
	profile, err := getProfile(c)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		return c.Redirect("/", fiber.StatusUnauthorized)
	}
	skey, _ := requestSiteData(c, profile)

	id := c.FormValue("ci")
	ct := strings.Trim(c.FormValue("ct"), " ")
	ca := strings.Trim(c.FormValue("ca"), " ")
	cv := strings.Trim(c.FormValue("cv"), " ")
	cd := c.FormValue("cd")
	ce := c.FormValue("ce")
	task := c.FormValue("task")

	if id == "" {
		writeError(c, errInvalidID)
		return errInvalidID
	}

	if task == "Delete" {
		err := model.DeleteCron(ctx, settingsStore, skey, id)
		if err != nil {
			return writeCrons(c, fmt.Sprintf("could not delete crons: %v", err))
		}

		err = cronScheduler.Set(&model.Cron{Skey: skey, ID: id, Enabled: false})
		if err != nil {
			return writeCrons(c, fmt.Sprintf("could not unset cron: %v", err))
		}

		return writeCrons(c, "")
	}

	site, err := model.GetSite(ctx, settingsStore, skey)
	if err != nil {
		writeError(c, fmt.Errorf("could not get site: %v", err))
		return err
	}

	cr := model.Cron{Skey: skey, ID: id, Action: ca, Var: cv, Data: cd, Enabled: ce != ""}
	err = cr.ParseTime(ct, site.Timezone)
	if err != nil {
		writeError(c, fmt.Errorf("could not parse time: %v", err))
		return err
	}

	err = model.PutCron(ctx, settingsStore, &cr)
	if err != nil {
		writeError(c, fmt.Errorf("could not put cron in datastore: %v", err))
		return err
	}

	err = cronScheduler.Set(&cr)
	if err != nil {
		writeError(c, fmt.Errorf("could not schedule cron: %v", err))
		return err
	}

	return nil
}
