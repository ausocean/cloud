/*
DESCRIPTION
  Datastore device type and functions.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2019-2024 the Australian Ocean Lab (AusOcean).

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
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/openfish/datastore"
)

// typeDevice is the name of the datastore device type.
const typeDevice = "Device"

// Device state statuses.
const (
	DeviceStatusOK = iota
	DeviceStatusUpdate
	DeviceStatusReboot
	DeviceStatusDebug
	DeviceStatusUpgrade
	DeviceStatusAlarm
	DeviceStatusTest
	DeviceStatusShutdown
)

var (
	ErrDeviceNotEnabled   = errors.New("device not enabled")
	ErrDeviceNotFound     = errors.New("device not found")
	ErrMissingDeviceKey   = errors.New("missing device key")
	ErrMalformedDeviceKey = errors.New("malformed device key")
	ErrInvalidDeviceKey   = errors.New("invalid device key")
	ErrInvalidMACAddress  = errors.New("invalid MAC address")
)

// Device represents a cloud device. The encoded MAC address
// serves as the datastore ID key. See MacEncode.
type Device struct {
	Skey          int64             // Site key.
	Dkey          int64             // Device key.
	Mac           int64             // Encoded MAC address (immutable).
	Name          string            // Device name.
	Inputs        string            // Input pins.
	Outputs       string            // Output pins.
	Wifi          string            // Wifi credentials, if any.
	MonitorPeriod int64             // Monitor period (s).
	ActPeriod     int64             // Actuation period (s)
	Type          string            // Client type.
	Version       string            // Client version.
	Protocol      string            // Client protocol.
	Status        int64             // Status code.
	Latitude      float64           // Device latitude.
	Longitude     float64           // Device longtitude.
	Enabled       bool              // True if enabled, false otherwise.
	Updated       time.Time         // Date/time last updated.
	other         map[string]string // Other, non-persistent data.
}

// Device types.
const (
	DevTypeController = "Controller"
	DevTypeCamera     = "Camera"
	DevTypeHydrophone = "Hydrophone"
	DevTypeSpeaker    = "Speaker"
	DevTypeAligner    = "Aligner"
	DevTypeTest       = "Test"
)

// Consts containing defaults for all device configs.
const (
	defaultActPeriod int64 = 60
	defaultMonPeriod int64 = 60
)

// Consts containing defaults for a controller config.
const (
	DefaultControllerInputs  string = "A0,X50,X51,X60,X10"
	DefaultControllerOutputs string = "D14,D15,D16" // Peripheral Power.
)

// ControllerDefaultVars is a map which relates controller variable names to their default values
var DefaultControllerVars = map[string]string{
	"AlarmNetwork":         "10",
	"AlarmPeriod":          "5",     // 5 seconds.
	"AlarmRecoveryVoltage": "840",   // 25.4545 V.
	"AlarmVoltage":         "825",   // 25 V.
	"AutoRestart":          "600",   // 10 minutes.
	"Power1":               "false", // initialise to OFF.
	"Power2":               "false", // initialise to OFF.
	"Power3":               "false", // initialise to OFF.
	"Pulses":               "3",     // 3 pulses per cycle.
	"PulseWidth":           "2",     // 1s ON, 1s OFF.
	"PulseCycle":           "30",    // 30s between flashes.
	"PulseSuppress":        "false",
}

// ControllerDefaultSensors contains a slice of the default sensors for a controller device.
var DefaultControllerSensors = []SensorV2{
	{Name: "Battery Voltage", Pin: "A0", Quantity: "DCV", Func: "scale", Args: "0.0289", Units: "V", Format: "round1"},
	{Name: "Analog Value", Pin: "X10", Quantity: "OTH", Func: "none", Format: "round1"},
	{Name: "Air Temperature", Pin: "X50", Quantity: "MWH", Func: "linear", Args: "0.1,-273.15", Units: "C", Format: "round1"},
	{Name: "Humidity", Pin: "X51", Quantity: "MMB", Func: "scale", Args: "0.1", Units: "%", Format: "round2"},
	{Name: "Water Temperature", Pin: "X60", Quantity: "MTW", Func: "linear", Args: "0.1,-273.15", Units: "C", Format: "round1"},
}

// ControllerDefaultActs contains a slice of the default actuators for a controller device.
var DefaultControllerActs = []ActuatorV2{
	{Name: "Device 1", Var: "Power1", Pin: "D16"},
	{Name: "Device 2", Var: "Power2", Pin: "D14"},
	{Name: "Device 3", Var: "Power3", Pin: "D15"},
}

// DeviceOption is a functional option to be supplied to NewDevice.
type DeviceOption func(context.Context, datastore.Store, *Device) error

// WithInputs creates a device with the given inputs.
func WithInputs(inputs string) DeviceOption {
	return func(ctx context.Context, store datastore.Store, dev *Device) error {
		dev.Inputs = inputs
		return nil
	}
}

// WithOutputs creates a device with the given outputs.
func WithOutputs(outputs string) DeviceOption {
	return func(ctx context.Context, store datastore.Store, dev *Device) error {
		dev.Outputs = outputs
		return nil
	}
}

// WithVariables creates the given variables for the device.
func WithVariables(variables map[string]string) DeviceOption {
	return func(ctx context.Context, store datastore.Store, dev *Device) error {
		// Put all variables into the datastore.
		for varName, varVal := range variables {
			err := PutVariable(ctx, store, dev.Skey, dev.MAC()+"."+varName, varVal)
			if err != nil {
				return fmt.Errorf("unable to put variable: %w", err)
			}
		}
		return nil
	}
}

// WithSensors creates the given sensors for the device.
func WithSensors(sensors []SensorV2) DeviceOption {
	return func(ctx context.Context, store datastore.Store, dev *Device) error {
		// Put all sensors into the datastore.
		for _, sensor := range sensors {
			sensor.Mac = dev.Mac
			err := PutSensorV2(ctx, store, &sensor)
			if err != nil {
				return fmt.Errorf("unable to put sensor %s: %w", sensor.Name, err)
			}
		}
		return nil
	}
}

// WithActuators creates the given actuators for the device.
func WithActuators(acts []ActuatorV2) DeviceOption {
	return func(ctx context.Context, store datastore.Store, dev *Device) error {
		// Put all actuators into the datastore.
		for _, act := range acts {
			act.Mac = dev.Mac
			err := PutActuatorV2(ctx, store, &act)
			if err != nil {
				return fmt.Errorf("unable to put actuator %s: %w", act.Name, err)
			}
		}
		return nil
	}
}

// WithType sets the given type of the new device. If no type is passed, or the type is not a known type
// this will create a device without any default settings (only MAC and name), and an empty device type.
func WithType(deviceType string) DeviceOption {
	return func(ctx context.Context, store datastore.Store, dev *Device) error {
		devTypes := []string{
			DevTypeController,
			DevTypeCamera,
			DevTypeHydrophone,
			DevTypeSpeaker,
			DevTypeAligner,
			DevTypeTest,
		}

		// Check for a valid type.
		if !slices.Contains[[]string, string](devTypes, deviceType) {
			deviceType = ""
		}
		dev.Type = deviceType
		return nil
	}
}

// WithWifi sets the WiFi SSID and password in a CSV format.
func WithWifi(SSID, password string) DeviceOption {
	return func(ctx context.Context, store datastore.Store, dev *Device) error {
		dev.Wifi = fmt.Sprintf("%s,%s", SSID, password)
		return nil
	}
}

// WithLocation sets the latitude and longitude of the device.
func WithLocation(lat, long float64) DeviceOption {
	return func(ctx context.Context, store datastore.Store, dev *Device) error {
		dev.Longitude = long
		dev.Latitude = lat
		return nil
	}
}

// NewDevice creates a new device with the given parameters, and options.
//
// NOTE: Options are applied in order and any order dependent options should be
// passed in order of execution.
func NewDevice(ctx context.Context, store datastore.Store, skey int64, name, MAC string, options ...DeviceOption) (*Device, error) {
	// Create a device with common config parameters.
	dev := &Device{
		Skey:          skey,
		Name:          name,
		Mac:           MacEncode(MAC),
		ActPeriod:     defaultActPeriod,
		MonitorPeriod: defaultMonPeriod,
		Enabled:       true,
	}

	// Apply the functional options.
	var err error
	for i, opt := range options {
		err = opt(ctx, store, dev)
		if err != nil {
			return nil, fmt.Errorf("unable to apply option[%d]: %w", i, err)
		}
	}

	// Put the device into the datastore.
	err = PutDevice(ctx, store, dev)
	if err != nil {
		return nil, fmt.Errorf("unable to put device into settings store: %w", err)
	}

	return dev, nil
}

// Encode serializes a Device into tab-separated values.
func (dev *Device) Encode() []byte {
	return []byte(fmt.Sprintf("%d\t%d\t%d\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s\t%s\t%f\t%f\t%t\t%d",
		dev.Skey, dev.Dkey, dev.Mac, dev.Name, dev.Inputs, dev.Outputs, dev.Wifi, dev.MonitorPeriod, dev.ActPeriod, dev.Status, dev.Type, dev.Version, dev.Protocol, dev.Latitude, dev.Longitude, dev.Enabled, dev.Updated.Unix()))
}

// Decode deserializes a Device from tab-separated values.
func (dev *Device) Decode(b []byte) error {
	p := strings.Split(string(b), "\t")
	if len(p) != 17 {
		return datastore.ErrDecoding
	}
	var err error
	dev.Skey, err = strconv.ParseInt(p[0], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	dev.Dkey, err = strconv.ParseInt(p[1], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	dev.Mac, err = strconv.ParseInt(p[2], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	dev.Name = p[3]
	dev.Inputs = p[4]
	dev.Outputs = p[5]
	dev.Wifi = p[6]
	dev.MonitorPeriod, err = strconv.ParseInt(p[7], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	dev.ActPeriod, err = strconv.ParseInt(p[8], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	dev.Status, err = strconv.ParseInt(p[9], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	dev.Type = p[10]
	dev.Version = p[11]
	dev.Protocol = p[12]
	dev.Latitude, err = strconv.ParseFloat(p[13], 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	dev.Longitude, err = strconv.ParseFloat(p[14], 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	dev.Enabled, err = strconv.ParseBool(p[15])
	if err != nil {
		return datastore.ErrDecoding
	}
	ts, err := strconv.ParseInt(p[16], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	dev.Updated = time.Unix(ts, 0)
	return nil
}

// Copy copies a device to dst, or returns a copy of the device when dst is nil.
func (dev *Device) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var d *Device
	if dst == nil {
		d = new(Device)
	} else {
		var ok bool
		d, ok = dst.(*Device)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*d = *dev
	return d, nil
}

var devCache datastore.Cache = datastore.NewEntityCache()

// GetCache returns the device cache.
func (dev *Device) GetCache() datastore.Cache {
	return nil
}

// Return the MAC address as a formated string, i.e., a wrapper for MacDecode(dev.Mac).
func (dev *Device) MAC() string {
	return MacDecode(dev.Mac)
}

// Return the MAC address as a hexadecimal string, i.e., essentially a MAC address without the colons.
func (dev *Device) Hex() string {
	return fmt.Sprintf("%012x", dev.Mac)
}

// Return the device status as text.
func (dev *Device) StatusText() string {
	switch dev.Status {
	case DeviceStatusOK:
		return "ok"
	case DeviceStatusUpdate:
		return "update"
	case DeviceStatusReboot:
		return "reboot"
	case DeviceStatusDebug:
		return "debug"
	case DeviceStatusUpgrade:
		return "upgrade"
	case DeviceStatusAlarm:
		return "alarm"
	case DeviceStatusTest:
		return "test"
	case DeviceStatusShutdown:
		return "shutdown"
	default:
		return "unknown"
	}
}

// Return other data which is not persistent.
func (dev *Device) Other(key string) string {
	return dev.other[key]
}

// Set other data which is not persistent.
func (dev *Device) SetOther(key, value string) {
	if dev.other == nil {
		dev.other = make(map[string]string)
	}
	dev.other[key] = value
}

// InputList returns device inputs as a list.
func (dev *Device) InputList() []string {
	return strings.Split(dev.Inputs, ",")
}

// OutputList returns device outputs as a list.
func (dev *Device) OutputList() []string {
	return strings.Split(dev.Outputs, ",")
}

// PutDevice creates or updates a device.
func PutDevice(ctx context.Context, store datastore.Store, dev *Device) error {
	dev.Updated = time.Now()
	key := store.IDKey(typeDevice, dev.Mac)
	_, err := store.Put(ctx, key, dev)
	return err
}

// GetDevice returns a Device by its integer ID (which is the encoded
// MAC address).
func GetDevice(ctx context.Context, store datastore.Store, mac int64) (*Device, error) {
	key := store.IDKey(typeDevice, mac)
	dev := new(Device)
	err := store.Get(ctx, key, dev)
	if err != nil {
		return nil, err
	}
	return dev, nil
}

// GetDevicesBySite returns all the devices for a given site.
func GetDevicesBySite(ctx context.Context, store datastore.Store, skey int64) ([]Device, error) {
	// FileStore queries must be handled specially.
	_, filestore := store.(*datastore.FileStore)
	if filestore {
		return getDevicesBySiteFromFileStore(ctx, store, skey)
	}

	q := store.NewQuery(typeDevice, false)
	q.Filter("Skey =", skey)
	q.Order("Name")
	var devs []Device
	_, err := store.GetAll(ctx, q, &devs)
	return devs, err
}

// getDevicesBySiteFromFileStore retrieves devices from a FileStore.
// Since FileStore does not index devides by skey, this requires
// retrieving all of the devices then filtering out the ones that
// don't match.
func getDevicesBySiteFromFileStore(ctx context.Context, store datastore.Store, skey int64) ([]Device, error) {
	q := store.NewQuery(typeDevice, false)
	var devices []Device
	_, err := store.GetAll(ctx, q, &devices)
	if err != nil {
		return nil, err
	}

	// Filter out devices that don't match the given skey.
	for i := len(devices) - 1; i >= 0; i -= 1 {
		if devices[i].Skey != skey {
			if i == len(devices)-1 {
				devices = devices[:i]
			} else {
				devices = append(devices[:i], devices[i+1:]...)
			}
		}
	}

	// Order by device ID.
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].Name < devices[j].Name
	})
	return devices, nil
}

// DeleteDevice deletes a device.
func DeleteDevice(ctx context.Context, store datastore.Store, mac int64) error {
	key := store.IDKey(typeDevice, mac)
	return store.DeleteMulti(ctx, []*datastore.Key{key})
}

// MacEncode encodes a MAC address string, optionally colon-separated,
// as a network-endian int64, e.g., "00:00:00:00:00:01" and
// "000000000001" are both encoded as 1. Returns 0 for an invalid
// MAC address.
func MacEncode(mac string) int64 {
	mac = strings.ReplaceAll(mac, ":", "")
	if len(mac) != 12 {
		return 0
	}
	var enc int64
	var shift uint
	for i := 10; i >= 0; i -= 2 {
		d1 := hexToDec(mac[i])
		d2 := hexToDec(mac[i+1])
		if d1 == -1 || d2 == -1 {
			return 0
		}
		n := (d1 << 4) + d2
		enc += n << shift
		shift += 8
	}
	return enc
}

// hexToDec returns the decimal integer corresponding to a hex byte, or -1 if there is none.
func hexToDec(hex byte) int64 {
	switch hex := int64(hex | ' '); {
	case '0' <= hex && hex <= '9':
		return hex - '0'
	case 'a' <= hex && hex <= 'f':
		return hex - ('a' - 10)
	default:
		return -1
	}
}

// MacDecode decodes a network-endian int64 as a colon-separated MAC
// address string, e.g., 1 is decoded as "00:00:00:00:00:01". Zero is
// an invalid encoding and the empty string is returned instead of
// 00:00:00:00:00:00.
func MacDecode(enc int64) string {
	if enc == 0 {
		return ""
	}
	const hexDigits = "0123456789ABCDEF"
	bytes := make([]byte, 17)
	for i := 5; i >= 0; i-- {
		bytes[i*3] = hexDigits[(enc>>4)&0xF]
		bytes[i*3+1] = hexDigits[enc&0xF]
		if i != 5 {
			bytes[i*3+2] = byte(':')
		}
		enc = enc >> 8
	}
	return string(bytes)
}

// CheckDevice returns a device if the supplied MAC address is valid,
// the device key (supplied as a string) is correct and the device is enabled, else an error.
func CheckDevice(ctx context.Context, store datastore.Store, mac string, dk string) (*Device, error) {
	if !IsMacAddress(mac) {
		return nil, ErrInvalidMACAddress
	}

	dev, err := GetDevice(ctx, store, MacEncode(mac))
	if err != nil {
		return nil, err
	}
	if dk == "" {
		return dev, ErrMissingDeviceKey
	}
	dkey, err := strconv.ParseInt(dk, 10, 64)
	if err != nil {
		return dev, ErrMalformedDeviceKey
	}
	if dev.Dkey != dkey {
		return dev, ErrInvalidDeviceKey
	}
	if !dev.Enabled {
		return dev, ErrDeviceNotEnabled
	}
	return dev, nil
}

// IsMacAddress returns true if mac is a valid IPv4 MAC address,
// optionally colon-separated, false otherwise. False is returned for
// "00:00:00:00:00:00".
func IsMacAddress(mac string) bool {
	mac = strings.ReplaceAll(mac, ":", "")
	if len(mac) != 12 || mac == "000000000000" {
		return false
	}
	for i := 10; i >= 0; i -= 2 {
		if hexToDec(mac[i]) == -1 {
			return false
		}
		if hexToDec(mac[i+1]) == -1 {
			return false
		}
	}
	return true
}

// DeviceIsUp returns true if the device exists and is up, or false
// otherwise. Up status is determined by checking the uptime variable
// associated with the device. The device is considered to be up if
// the uptime has been updated within the last two monitor periods.
func DeviceIsUp(ctx context.Context, store datastore.Store, mac string) (bool, error) {
	dev, err := GetDevice(ctx, store, MacEncode(mac))
	if err != nil {
		return false, fmt.Errorf("could not get device: %w", err)
	}
	v, err := GetVariable(ctx, store, dev.Skey, "_"+dev.Hex()+".uptime")
	if err != nil {
		return false, fmt.Errorf("could not get uptime variable: %w", err)
	}
	if time.Since(v.Updated) < time.Duration(2*int(dev.MonitorPeriod))*time.Second {
		return true, nil
	}
	return false, nil
}
