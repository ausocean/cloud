/*
AUTHORS
  David Sutton <david@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean).

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
  or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
  License for more details.

  You should have received a copy of the GNU General Public License in
  gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package system

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/system/camera"
	"github.com/ausocean/openfish/datastore"
)

// RigSystem represents a controller device which has associated
// variables, sensors, acuators, and peripherals.
type RigSystem struct {
	Controller  model.Device
	Variables   []*model.Variable
	Sensors     []*model.SensorV2
	Actuators   []*model.ActuatorV2
	Peripherals []*model.Device
}

// addVariables adds variables to the rig system controller. This
// implements the variableHolder interface.
func (sys *RigSystem) AddVariables(variables ...*model.Variable) {
	sys.Variables = append(sys.Variables, variables...)
}

// SetWifi adds the wifi name and password to the rig system Controller.
// This implements the wifiHolder interface.
func (sys *RigSystem) SetWifi(ssid, pass string) {
	sys.Controller.Wifi = fmt.Sprintf("%s,%s", ssid, pass)
}

// SetLocation adds the location of the system to the system controller,
// as well as any defined peripherals.
func (sys *RigSystem) SetLocation(lat, long float64) {
	sys.Controller.Latitude = lat
	sys.Controller.Longitude = long

	for _, p := range sys.Peripherals {
		p.Latitude = lat
		p.Longitude = long
	}
}

// WithSensors is a functional option that adds the passed sensors to the RigSystem.
func WithSensors(sensors ...*model.SensorV2) func(any) error {
	return func(v any) error {
		sys, ok := v.(*RigSystem)
		if !ok {
			return fmt.Errorf("%v is not a RigSystem", reflect.TypeOf(v).String())
		}
		sys.Sensors = append(sys.Sensors, sensors...)
		return nil
	}
}

// WithActuators is a functional option that adds the passed actuators to the RigSystem.
func WithActuators(actuators ...*model.ActuatorV2) func(any) error {
	return func(v any) error {
		sys, ok := v.(*RigSystem)
		if !ok {
			return fmt.Errorf("%v is not a RigSystem", reflect.TypeOf(v).String())
		}
		sys.Actuators = append(sys.Actuators, actuators...)
		return nil
	}
}

// WithPeripherals is a functional option that adds the passed peripherals to the RigSystem.
func WithPeripherals(peripherals ...*model.Device) func(any) error {
	return func(v any) error {
		sys, ok := v.(*RigSystem)
		if !ok {
			return fmt.Errorf("%v is not a RigSystem", reflect.TypeOf(v).String())
		}
		sys.Peripherals = append(sys.Peripherals, peripherals...)
		return nil
	}
}

// WithRigSystemDefaults is a functional option that uses all of the current defaults for a rig system.
func WithRigSystemDefaults() func(any) error {
	const (
		defaultVoltageScaleFactor = 0.0289
		defaultCurrentScaleFactor = 0.451
	)
	return func(v any) error {
		sys, ok := v.(*RigSystem)
		if !ok {
			return fmt.Errorf("%v is not a RigSystem", reflect.TypeOf(v).String())
		}
		sys.Variables = append(sys.Variables,
			model.NewAlarmNetworkVar(10),
			model.NewAlarmPeriodVar(5*time.Second),
			model.NewAlarmRecoveryVoltageVar(905),
			model.NewAlarmVoltageVar(890),
			model.NewAutoRestartVar(10*time.Minute),
			model.NewPower1Var(false),
			model.NewPower2Var(false),
			model.NewPower3Var(false),
			model.NewPulsesVar(3),
			model.NewPulseWidthVar(2*time.Second),
			model.NewPulseCycleVar(30*time.Second),
			model.NewPulseSuppressVar(false),
		)

		sys.Sensors = append(sys.Sensors,
			model.AirTemperatureSensor(),
			model.HumiditySensor(),
			model.WaterTemperatureSensor(),
			model.ESP32BatterySensor(defaultVoltageScaleFactor),
			model.ESP32Power1Sensor(defaultVoltageScaleFactor),
			model.ESP32Power2Sensor(defaultVoltageScaleFactor),
			model.ESP32Power3Sensor(defaultVoltageScaleFactor),
			model.ESP32NetworkSensor(defaultVoltageScaleFactor),
			model.ESP32CurrentSensor(defaultCurrentScaleFactor),
		)

		sys.Actuators = append(sys.Actuators,
			model.NewESP32Device1Actuator(),
			model.NewESP32Device2Actuator(),
			model.NewESP32Device3Actuator(),
		)

		return nil
	}
}

// NewRigSystem returns a new RigSystem with the given options. It is the callers
// responsibility to put the components into the datastore.
//
// dkey, MAC and name refer to the device key, MAC Address and name of the Controller which will
// be the heart of the RigSystem.
func NewRigSystem(skey, dkey int64, MAC, name string, options ...Option) (*RigSystem, error) {
	if model.MacEncode(MAC) == 0 {
		return nil, model.ErrInvalidMACAddress
	}

	sys := &RigSystem{
		Controller: model.Device{
			Skey:          skey,
			Dkey:          dkey,
			Mac:           model.MacEncode(MAC),
			Name:          name,
			Type:          model.DevTypeController,
			Enabled:       true,
			MonitorPeriod: 60,
			ActPeriod:     60,
		},
	}

	// Apply functional options.
	for i, opt := range options {
		err := opt(sys)
		if err != nil {
			return nil, fmt.Errorf("failed to apply option[%d]: %w", i, err)
		}
	}

	// Append site/device information to actuators, sensors, and variables.
	for _, variable := range sys.Variables {
		variable.Skey = skey
		variable.Scope = strings.ReplaceAll(sys.Controller.MAC(), ":", "")
	}
	for _, sensor := range sys.Sensors {
		sensor.Mac = sys.Controller.Mac
		sys.Controller.Inputs += sensor.Pin + ","
	}
	for _, actuator := range sys.Actuators {
		actuator.Mac = sys.Controller.Mac
		sys.Controller.Outputs += actuator.Pin + ","
	}
	sys.Controller.Inputs, _ = strings.CutSuffix(sys.Controller.Inputs, ",")
	sys.Controller.Outputs, _ = strings.CutSuffix(sys.Controller.Outputs, ",")

	log.Printf("Inputs: %s, Outputs: %s", sys.Controller.Inputs, sys.Controller.Outputs)

	return sys, nil
}

// Put puts a RigSystem and all of its components into the datastore.
func PutRigSystem(ctx context.Context, store datastore.Store, system *RigSystem) error {
	// Put the Controller.
	err := model.PutDevice(ctx, store, &system.Controller)
	if err != nil {
		return fmt.Errorf("unable to put system controller: %w", err)
	}

	// Put all variables.
	for _, v := range system.Variables {
		err = model.PutVariable(ctx, store, v.Skey, system.Controller.Hex()+"."+v.Name, v.Value)
		if err != nil {
			return fmt.Errorf("unable to put variable with name: %s, err: %w", v.Name, err)
		}
	}

	// Put all sensors.
	for _, s := range system.Sensors {
		err = model.PutSensorV2(ctx, store, s)
		if err != nil {
			return fmt.Errorf("unable to put sensor with name: %s, err: %w", s.Name, err)
		}
	}

	// Put all actuators.
	for _, a := range system.Actuators {
		err = model.PutActuatorV2(ctx, store, a)
		if err != nil {
			return fmt.Errorf("unable to put actuator with name: %s, err: %w", a.Name, err)
		}
	}

	// Put all Peripherals
	for _, p := range system.Peripherals {
		err = model.PutDevice(ctx, store, p)
		if err != nil {
			return fmt.Errorf("unable to put peripheral with name: %s, err: %w", p.Name, err)
		}
	}

	return nil
}

// CameraSystem contains a camera device and the associated variables.
type CameraSystem struct {
	Cam  *model.Device
	Vars []*model.Variable
}

// SetWifi sets the wifi field of the camera.
func (sys *CameraSystem) SetWifi(ssid, pass string) {
	sys.Cam.Wifi = ssid + "," + pass
}

// SetLocation sets the location fields of the camera.
func (sys *CameraSystem) SetLocation(lat, long float64) {
	sys.Cam.Latitude = lat
	sys.Cam.Longitude = long
}

// AddVariables adds the associated variables to the system.
func (sys *CameraSystem) AddVariables(variables ...*model.Variable) {
	sys.Vars = variables
}

// WithCameraDefaults applies all of the current defaults to the system.
func (sys *CameraSystem) WithCameraDefaults() {
	sys.AddVariables(
		model.NewAutoWhiteBalanceVar(camera.DefaultAutoWhiteBalance),
		model.NewBitrateVar(camera.DefaultBitrate),
		model.NewContrastVar(camera.DefaultContrast),
		model.NewFrameRateVar(camera.DefaultFrameRate),
		model.NewHDRVar(camera.DefaultHDR),
		model.NewHeightVar(camera.DefaultHeight),
		model.NewInputVar(camera.DefaultInput),
		model.NewOutputVar(camera.DefaultOutput),
		model.NewRotationVar(camera.DefaultRotation),
		model.NewSaturationVar(camera.DefaultSaturation),
		model.NewSharpnessVar(camera.DefaultSharpness),
		model.NewWidthVar(camera.DefaultWidth),
		model.NewLoggingVar(camera.DefaultLogging),
		model.NewModeVar(camera.DefaultMode),
		model.NewRTMPURLVar(""),
	)
}

// NewCamera returns a new camera with the given name and mac address, with the given options applied.
func NewCamera(skey, dkey int64, name string, mac string, opts ...Option) (*CameraSystem, error) {
	MAC := model.MacEncode(mac)
	if MAC == 0 {
		return nil, model.ErrInvalidMACAddress
	}

	sys := &CameraSystem{
		Cam: &model.Device{
			Skey: skey,
			Dkey: dkey,
			Name: name,
			Mac:  MAC,
			Type: model.DevTypeCamera,
		},
	}

	for i, opt := range opts {
		err := opt(sys)
		if err != nil {
			return nil, fmt.Errorf("unable to apply option (%d): %w", i, err)
		}
	}

	for _, variable := range sys.Vars {
		variable.Skey = skey
		variable.Scope = strings.ReplaceAll(sys.Cam.MAC(), ":", "")
	}

	return sys, nil
}
