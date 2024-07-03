/*
DESCRIPTION
	factories.go provides a factory pattern to generate common
	implementations of entities.

AUTHORS
	David Sutton <davidsutton@ausocean.org>

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

	You should have received a copy of the GNU General Public License
	in gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package model

import (
	"github.com/ausocean/utils/nmea"
)

// Sensor factories //

// Consts used by sensor factories.
const (
	tempLinearArgs      string = "0.1,-273.15"
	humidityScaleFactor string = "0.1"
)

// BatteryVoltageSensor returns a default calibrated battery sensor.
func BatteryVoltageSensor(scaleFactor string) *SensorV2 {
	return &SensorV2{
		Name:     "Battery Voltage",
		Pin:      pinBatteryVoltage,
		Quantity: nmea.DCVoltage,
		Func:     keyScale,
		Args:     scaleFactor,
		Units:    unitVoltage,
		Format:   keyRound1,
	}
}

// AnalogValueSensor returns an analog value sensor.
func AnalogValueSensor() *SensorV2 {
	return &SensorV2{
		Name:     "Analog Value",
		Pin:      pinAnalogValue,
		Quantity: nmea.Other,
		Func:     keyNone,
		Format:   keyRound1,
	}
}

// AirTemperatureSensor returns an air temperature sensor.
func AirTemperatureSensor() *SensorV2 {
	return &SensorV2{
		Name:     "Air Temperature",
		Pin:      pinAirTemperature,
		Quantity: nmea.AirTemperature,
		Func:     keyLinear,
		Args:     tempLinearArgs,
		Units:    unitCelsius,
		Format:   keyRound1,
	}
}

// HumiditySensor returns a humidity sensor.
func HumiditySensor() *SensorV2 {
	return &SensorV2{
		Name:     "Humidity",
		Pin:      pinHumidity,
		Quantity: nmea.Humidity,
		Func:     keyScale,
		Args:     humidityScaleFactor,
		Units:    unitPercent,
		Format:   keyRound1,
	}
}

// WaterTemperatureSensor returns an water temperature sensor.
func WaterTemperatureSensor() *SensorV2 {
	return &SensorV2{
		Name:     "Water Temperature",
		Pin:      pinWaterTemperature,
		Quantity: nmea.WaterTemperature,
		Func:     keyLinear,
		Args:     tempLinearArgs,
		Units:    unitCelsius,
		Format:   keyRound1,
	}
}
