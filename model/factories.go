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

// Consts/Pseudo used by sensor factories.
const humidityScaleFactor Arg = 0.1

var tempLinearArgs []Arg = []Arg{0.1, -273.15}

// BatteryVoltageSensor returns a default calibrated battery sensor.
func BatteryVoltageSensor(scaleFactor float64) SensorV2 {
	return sensorShim(
		"Battery Voltage",
		pinBatteryVoltage,
		nmea.DCVoltage,
		funcScale,
		unitVoltage,
		formRound1,
		Arg(scaleFactor),
	)
}

// AnalogValueSensor returns an analog value sensor.
func AnalogValueSensor() SensorV2 {
	return sensorShim(
		"Analog Value",
		pinAnalogValue,
		nmea.Other,
		funcNone,
		"",
		formRound1,
	)
}

// AirTemperatureSensor returns an air temperature sensor.
func AirTemperatureSensor() SensorV2 {
	return sensorShim(
		"Air Temperature",
		pinAirTemperature,
		nmea.AirTemperature,
		funcLinear,
		unitCelsius,
		formRound1,
		tempLinearArgs...,
	)
}

// HumiditySensor returns a humidity sensor.
func HumiditySensor() SensorV2 {
	return sensorShim(
		"Humidity",
		pinHumidity,
		nmea.Humidity,
		funcScale,
		unitPercent,
		formRound1,
		humidityScaleFactor,
	)
}

// WaterTemperatureSensor returns an water temperature sensor.
func WaterTemperatureSensor() SensorV2 {
	return sensorShim(
		"Water Temperature",
		pinWaterTemperature,
		nmea.WaterTemperature,
		funcLinear,
		unitCelsius,
		formRound1,
		tempLinearArgs...,
	)
}
