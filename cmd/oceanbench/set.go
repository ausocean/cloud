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
	"github.com/ausocean/cloud/system"
	"github.com/ausocean/openfish/datastore"
	"github.com/ausocean/utils/nmea"
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
	data.Users, err = getUsersForSiteMenu(w, r, ctx, profile, data)
	if err != nil {
		writeTemplate(w, r, "set/device.html", &data, fmt.Sprintf("could not populate site menu: %v", err.Error()))
		return
	}

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
		reportDevicesError(w, r, data, "get site error: %v", err)
		return
	}
	data.Timezone = site.Timezone

	data.Devices, err = model.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		reportDevicesError(w, r, data, "get devices by site error: %v", err)
		return
	}

	if msg != "" {
		reportDevicesError(w, r, data, msg, args...)
		return
	}

	if !model.IsMacAddress(data.Mac) {
		writeTemplate(w, r, "set/device.html", &data, "")
		return
	}

	data.Device, err = model.GetDevice(ctx, settingsStore, model.MacEncode(data.Mac))
	if err != nil {
		reportDevicesError(w, r, data, "get device error for ma: %s, %v", data.Mac, err)
		return
	}

	data.Vars, err = model.GetVariablesBySite(ctx, settingsStore, skey, data.Device.Hex())
	if err != nil && !errors.Is(err, datastore.ErrNoSuchEntity) {
		reportDevicesError(w, r, data, "get device variables error: %v", err)
		return
	}

	data.VarTypes, err = model.GetVariablesBySite(ctx, settingsStore, skey, "_type")
	if err != nil && !errors.Is(err, datastore.ErrNoSuchEntity) {
		reportDevicesError(w, r, data, "get device variable types error: %v", err)
		return
	}

	data.Sensors, err = model.GetSensorsV2(ctx, settingsStore, data.Device.Mac)
	if err != nil {
		reportDevicesError(w, r, data, "get sensors error: %v", err)
		return
	}

	data.Actuators, err = model.GetActuatorsV2(ctx, settingsStore, data.Device.Mac)
	if err != nil {
		reportDevicesError(w, r, data, "get actuators error: %v", err)
		return
	}

	// Provide uptime and device status information.
	v, err := model.GetVariable(ctx, settingsStore, data.Device.Skey, "_"+data.Device.Hex()+".uptime")
	switch {
	case errors.Is(err, datastore.ErrNoSuchEntity):
		data.Device.SetOther("sending", "black")
	case err != nil:
		reportDevicesError(w, r, data, "get uptime error: %v", err)
		return
	case time.Since(v.Updated) < time.Duration(2*int(data.Device.MonitorPeriod))*time.Second:
		data.Device.SetOther("sending", "green")
		ut, err := strconv.Atoi(v.Value)
		if err == nil {
			data.Device.SetOther("uptime", (time.Duration(ut) * time.Second).String())
		}
	default:
		data.Device.SetOther("sending", "red")
	}

	// Get the local address only if available.
	v, err = model.GetVariable(ctx, settingsStore, data.Device.Skey, "_"+data.Device.Hex()+".localaddr")
	if err == nil {
		data.Device.SetOther("localaddr", v.Value)
	}

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

// newDevicesHandler handles provisioning of new devices.
//
// Form Fields:
//
//	NAME = Name of the new device
//	MA = MAC address
//	DT = device type
//	SSID = WiFi name
//	PASS = WiFi password
func newDevicesHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := context.Background()
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	skey, _ := profileData(profile)

	log.Println(skey)

	// Parse the form values.
	name := r.FormValue("name")
	ma := r.FormValue("ma")
	dt := r.FormValue("dt")
	ssid := r.FormValue("ssid")
	pass := r.FormValue("pass")
	sLat := r.FormValue("lat")
	sLong := r.FormValue("long")
	r.ParseForm()

	var lat, long float64
	if sLat != "" && sLong != "" {
		lat, err = strconv.ParseFloat(sLat, 64)
		if err != nil {
			writeError(w, fmt.Errorf("unable to parse float64 from: %s, err:", err))
			return
		}
		long, err = strconv.ParseFloat(sLong, 64)
		if err != nil {
			writeError(w, fmt.Errorf("unable to parse float64 from: %s, err:", err))
			return
		}
	}

	if !model.IsMacAddress(ma) {
		writeError(w, model.ErrInvalidMACAddress)
		return
	}

	var isValidType bool
	for _, t := range devTypes {
		if dt == t {
			isValidType = true
			break
		}
	}
	if !isValidType {
		writeError(w, model.ErrInvalidDevType)
		return
	}

	// Create the device.
	var sys *system.RigSystem
	switch dt {
	case model.DevTypeController:
		// Create a controller with all default values defined in rig_system.go.
		sys, err = system.NewRigSystem(skey, ma, name,
			system.WithDefaults(),
			system.WithWifi(ssid, pass),
			system.WithLocation(lat, long),
		)
		if err != nil {
			writeError(w, err)
			return
		}
	default:
		writeError(w, errNotImplemented)
		return
	}

	for _, v := range sys.Variables {
		log.Printf("%+v", v)
	}

	err = system.PutRigSystem(ctx, settingsStore, sys)
	if err != nil {
		writeError(w, fmt.Errorf("unable to put rig system: %w", err))
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/set/devices?ma=%s", ma), http.StatusSeeOther)
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

	data.Users, err = getUsersForSiteMenu(w, r, ctx, profile, data)
	if err != nil {
		writeTemplate(w, r, "set/cron.html", &data, fmt.Sprintf("could not populate site menu: %v", err.Error()))
		return
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
		writeCrons(w, r, errInvalidID.Error())
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
		writeCrons(w, r, fmt.Sprintf("could not get site: %v", err))
		return
	}

	c := model.Cron{Skey: skey, ID: id, Action: ca, Var: cv, Data: cd, Enabled: ce != ""}
	err = c.ParseTime(ct, site.Timezone)
	if err != nil {
		writeCrons(w, r, fmt.Sprintf("could not parse time: %v", err))
		return
	}

	err = model.PutCron(ctx, settingsStore, &c)
	if err != nil {
		writeCrons(w, r, fmt.Sprintf("could not put cron in datastore: %v", err))
		return
	}

	err = cronScheduler.Set(&c)
	if err != nil {
		writeCrons(w, r, fmt.Sprintf("could not schedule cron: %v", err))
		return
	}

	writeCrons(w, r, "")
}
