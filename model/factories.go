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
	"strconv"
	"time"

	"github.com/ausocean/utils/nmea"
)

// Consts/Pseudo used by sensor factories.
const humidityScaleFactor Arg = 0.1

var tempLinearArgs []Arg = []Arg{0.1, -273.15}

// BatteryVoltageSensor returns a default calibrated battery sensor.
func BatteryVoltageSensor(scaleFactor float64) *SensorV2 {
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
func AnalogValueSensor() *SensorV2 {
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
func AirTemperatureSensor() *SensorV2 {
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
func HumiditySensor() *SensorV2 {
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
func WaterTemperatureSensor() *SensorV2 {
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

// NewAlarmNetworkVar returns a new AlarmNetwork var with the
// passed number of failures as its value.
func NewAlarmNetworkVar(numberOfFailures int) *Variable {
	return &Variable{
		Name:  "AlarmNetwork",
		Value: strconv.Itoa(numberOfFailures),
	}
}

// NewAlarmPeriodVar returns a new AlarmPeriod var with the
// passed duration as its value.
func NewAlarmPeriodVar(duration time.Duration) *Variable {
	return &Variable{
		Name:  "AlarmPeriod",
		Value: strconv.FormatFloat(duration.Seconds(), 'f', 0, 64),
	}
}

// NewAlarmRecoveryVoltageVar returns a new AlarmRecoveryVoltage var with the
// passed threshold as its value.
func NewAlarmRecoveryVoltageVar(threshold int) *Variable {
	return &Variable{
		Name:  "AlarmRecoveryVoltage",
		Value: strconv.Itoa(threshold),
	}
}

// NewAlarmVoltageVar returns a new AlarmVoltage var with the
// passed threshold as its value.
func NewAlarmVoltageVar(threshold int) *Variable {
	return &Variable{
		Name:  "AlarmVoltage",
		Value: strconv.Itoa(threshold),
	}
}

// NewAutoRestartVar returns a new AutoRestart var with the
// passed duration as its value.
func NewAutoRestartVar(duration time.Duration) *Variable {
	return &Variable{
		Name:  "AutoRestart",
		Value: strconv.FormatFloat(duration.Seconds(), 'f', 0, 64),
	}
}

// NewPower1Var returns a new Power1 var with the
// power initialised to the passed power.
func NewPower1Var(power bool) *Variable {
	return &Variable{
		Name:  "Power1",
		Value: strconv.FormatBool(power),
	}
}

// NewPower2Var returns a new Power2 var with the
// power initialised to the passed power.
func NewPower2Var(power bool) *Variable {
	return &Variable{
		Name:  "Power2",
		Value: strconv.FormatBool(power),
	}
}

// NewPower3Var returns a new Power3 var with the
// power initialised to the passed power.
func NewPower3Var(power bool) *Variable {
	return &Variable{
		Name:  "Power3",
		Value: strconv.FormatBool(power),
	}
}

// NewPulsesVar returns a new Pulses var with the
// passed number of pulses as its value.
func NewPulsesVar(pulses int) *Variable {
	return &Variable{
		Name:  "Pulses",
		Value: strconv.Itoa(pulses),
	}
}

// NewPulseWidthVar returns a new PulseWidth var with the
// passed width duration as its value.
func NewPulseWidthVar(width time.Duration) *Variable {
	return &Variable{
		Name:  "PulseWidth",
		Value: strconv.FormatFloat(width.Seconds(), 'f', 0, 64),
	}
}

// NewPulseCycleVar returns a new PulseCycle var with the
// passed cycle duration as its value.
func NewPulseCycleVar(cycle time.Duration) *Variable {
	return &Variable{
		Name:  "PulseCycle",
		Value: strconv.FormatFloat(cycle.Seconds(), 'f', 0, 64),
	}
}

// NewPulseSuppressVar returns a new PulseSuppress var with the
// passed value as its value.
func NewPulseSuppressVar(suppress bool) *Variable {
	return &Variable{
		Name:  "PulseSuppress",
		Value: strconv.FormatBool(suppress),
	}
}

// WhiteBalance is a string type to be used to set the AutoWhiteBalance Variable.
type WhiteBalance string

var (
	WhiteBalanceOff          WhiteBalance = "off"
	WhiteBalanceAuto         WhiteBalance = "auto"
	WhiteBalanceSun          WhiteBalance = "sun"
	WhiteBalanceCloud        WhiteBalance = "cloud"
	WhiteBalanceShade        WhiteBalance = "shade"
	WhiteBalanceTungsten     WhiteBalance = "tungsten"
	WhiteBalanceFluorescent  WhiteBalance = "fluorescent"
	WhiteBalanceIncandescent WhiteBalance = "incandescent"
	WhiteBalanceFlash        WhiteBalance = "flash"
	WhiteBalanceHorizon      WhiteBalance = "horizon"
)

// NewAutoWhiteBalanceVar returns a AutoWhiteBalance Variable with the given setting.
func NewAutoWhiteBalanceVar(setting WhiteBalance) *Variable {
	return &Variable{
		Name:  "AutoWhiteBalance",
		Value: string(setting),
	}
}

// NewBitrateVar returns a new Bitrate variable with the given bitrate.
func NewBitrateVar(rate int) *Variable {
	return &Variable{
		Name:  "Bitrate",
		Value: strconv.Itoa(rate),
	}
}

// NewConstrastVar returns a new Constrast variable with the given contrast.
func NewConstrastVar(contrast int) *Variable {
	return &Variable{
		Name:  "Contrast",
		Value: strconv.Itoa(contrast),
	}
}

// NewHeightVar returns a new Height variable with the given height.
func NewHeightVar(height int) *Variable {
	return &Variable{
		Name:  "Height",
		Value: strconv.Itoa(height),
	}
}

// NewISOVar returns a new ISO variable with the given iso.
func NewISOVar(iso int) *Variable {
	return &Variable{
		Name:  "ISO",
		Value: strconv.Itoa(iso),
	}
}

// OutputType is a string type to be used to set the Output Variable.
type OutputType string

var (
	OutputFile OutputType = "File"
	OutputHTTP OutputType = "HTTP"
	OutputRTMP OutputType = "RTMP"
	OutputRTP  OutputType = "RTP"
)

// NewOutputVar returns a new Output variable with the given output type.
func NewOutputVar(output OutputType) *Variable {
	return &Variable{
		Name:  "Output",
		Value: string(output),
	}
}

// NewRTMPURLVar returns a new blank RTMPURL variable.
func NewRTMPURLVar() *Variable {
	return &Variable{
		Name: "RTMPURL",
	}
}

// NewRotationVar returns a new Rotation variable with the given rotation.
func NewRotationVar(rotation int) *Variable {
	return &Variable{
		Name:  "Rotation",
		Value: strconv.Itoa(rotation),
	}
}

// NewSharpnessVar returns a new Sharpness variable with the given sharpness.
func NewSharpnessVar(sharpness int) *Variable {
	return &Variable{
		Name:  "Sharpness",
		Value: strconv.Itoa(sharpness),
	}
}

// NewWidthVar returns a new Width variable with the given width.
func NewWidthVar(width int) *Variable {
	return &Variable{
		Name:  "Width",
		Value: strconv.Itoa(width),
	}
}

// LogLevel is a string type to be used to set the logging Variable.
type LogLevel string

var (
	LoggingDebug   LogLevel = "Debug"
	LoggingInfo    LogLevel = "Info"
	LoggingWarning LogLevel = "Warning"
	LoggingError   LogLevel = "Error"
	LoggingFatal   LogLevel = "Fatal"
)

// NewLoggingVar returns a new logging variable with the given LogLevel type.
func NewLoggingVar(level LogLevel) *Variable {
	return &Variable{
		Name:  "logging",
		Value: string(level),
	}
}

// DeviceMode is a string type to be used to set the mode Variable.
type DeviceMode string

var (
	ModeNormal    DeviceMode = "Normal"
	ModePaused    DeviceMode = "Paused"
	ModeBurst     DeviceMode = "Burst"
	ModeShutdown  DeviceMode = "Shutdown"
	ModeCompleted DeviceMode = "Completed"
)

// NewModeVar returns a new mode variable with the given device mode type.
func NewModeVar(mode DeviceMode) *Variable {
	return &Variable{
		Name:  "mode",
		Value: string(mode),
	}
}

// NewDevice1Actuator returns a new actuator to control device 1.
// The actuator is linked to the Power 1 variable (and Pin).
func NewDevice1Actuator() *ActuatorV2 {
	return actuatorShim(
		"Device 1",
		"Power1",
		PinPower1,
	)
}

// NewDevice2Actuator returns a new actuator to control device 2.
// The actuator is linked to the Power 2 variable (and Pin).
func NewDevice2Actuator() *ActuatorV2 {
	return actuatorShim(
		"Device 2",
		"Power2",
		PinPower2,
	)
}

// NewDevice3Actuator returns a new actuator to control device 3.
// The actuator is linked to the Power 3 variable (and Pin).
func NewDevice3Actuator() *ActuatorV2 {
	return actuatorShim(
		"Device 3",
		"Power3",
		PinPower3,
	)
}
