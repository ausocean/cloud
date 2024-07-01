/*
NAME
  This file contains default values used for variables across different
	device types.

AUTHORS
  David Sutton <davidsutton@ausocean.org

LICENSE
  Copyright (C) 2018-2024 the Australian Ocean Lab (AusOcean)

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
	"fmt"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

// setupDefaultController takes the minimum settings required to define a Device and the settings store
// and adds all the default variables, sensors, and actuators for a controller.
//
// Args:
// - skey: Site Key of the new controller.
// - name: Name of the device (Mutable).
// - MAC: MAC address of the device optionally with colons (Immutable).
// - SSID: SSID of the network for the controller to connect.
// - password: password of the network.
// - lat, long: latitude and longitude of the device.
func setupDefaultController(ctx context.Context, skey int64, name, MAC, SSID, password string, lat, long float64, settingsStore datastore.Store) error {
	// Check that the passed MAC is valid.
	if !model.IsMacAddress(MAC) {
		return model.ErrInvalidMACAddress
	}

	// Create the Controller with default settings.
	controller := model.Device{
		Skey:          skey,
		Mac:           model.MacEncode(MAC),
		Name:          name,
		Inputs:        "A0,X50,X51,X60,X10",
		Outputs:       "D14,D15,D16",
		Wifi:          SSID + "," + password,
		MonitorPeriod: 60,
		ActPeriod:     60,
		Type:          "Controller",
		Latitude:      lat,
		Longitude:     long,
		Enabled:       true,
	}

	// Save the device to the datastore.
	err := model.PutDevice(ctx, settingsStore, &controller)
	if err != nil {
		return fmt.Errorf("unable to save controller to datastore: %w", err)
	}

	// Default variables used for a controller.
	var variables = []model.Variable{
		{Name: "AlarmNetwork", Value: "10"},
		{Name: "AlarmPeriod", Value: "5"},
		{Name: "AlarmRecoveryVoltage", Value: "840"},
		{Name: "AlarmVoltage", Value: "825"},
		{Name: "AutoRestart", Value: "600"},
		{Name: "Power1", Value: "false"},
		{Name: "Power2", Value: "false"},
		{Name: "Power3", Value: "false"},
		{Name: "Pulses", Value: "3"},
		{Name: "PulseWidth", Value: "2"},
		{Name: "PulseCycle", Value: "30"},
		{Name: "Suppress", Value: "false"},
	}

	// Put all the variables into the datastore.
	for _, variable := range variables {
		err = model.PutVariable(ctx, settingsStore, skey, MAC+"."+variable.Name, variable.Value)
		if err != nil {
			return fmt.Errorf("unable to put variable %s.%s: %w", MAC, variable.Name, err)
		}
	}

	// Default sensors used for a controller.
	var sensors = []model.SensorV2{
		{Name: "Battery Voltage", Pin: "A0", Quantity: "DCV", Func: "scale", Args: "0.0289", Units: "V", Format: "round1"},
		{Name: "Analog Value", Pin: "X10", Quantity: "OTH", Func: "none", Format: "round1"},
		{Name: "Air Temperature", Pin: "X50", Quantity: "MWH", Func: "linear", Args: "0.1,-273.15", Units: "C", Format: "round1"},
		{Name: "Humidity", Pin: "X51", Quantity: "MMB", Func: "scale", Args: "0.1", Units: "%", Format: "round2"},
		{Name: "Water Temperature", Pin: "X60", Quantity: "MTW", Func: "linear", Args: "0.1,-273.15", Units: "C", Format: "round1"},
	}

	// Put all the sensors into the datastore.
	for _, sensor := range sensors {
		sensor.Mac = model.MacEncode(MAC)
		err = model.PutSensorV2(ctx, settingsStore, &sensor)
		if err != nil {
			return fmt.Errorf("unable to put sensor %s: %w", sensor.Name, err)
		}
	}

	// Default Actuators for a controller.
	var actuators = []model.ActuatorV2{
		{Name: "Empty 1", Var: "Power1", Pin: "D16"},
		{Name: "Empty 2", Var: "Power2", Pin: "D14"},
		{Name: "Empty 3", Var: "Power3", Pin: "D15"},
	}

	// Put all the actuators into the datastore.
	for _, actuator := range actuators {
		actuator.Mac = model.MacEncode(MAC)
		err = model.PutActuatorV2(ctx, settingsStore, &actuator)
		if err != nil {
			return fmt.Errorf("unable to put actuator %s: %w", actuator.Name, err)
		}
	}

	return nil
}
