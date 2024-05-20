/*
DESCRIPTION
  Datastore device type and functions.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2019-2023 the Australian Ocean Lab (AusOcean).

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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/openfish/datastore"
)

// typeDevice is the name of the datastore device type.
const typeDevice = "Device"

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
	case 0:
		return "ok"
	case 1:
		return "update"
	case 2:
		return "reboot"
	case 3:
		return "debug"
	case 4:
		return "upgrade"
	case 5:
		return "alarm"
	case 6:
		return "test"
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

// Retain these for the sensor/actuator migration then remove.
func (dev *Device) InputListWithID() []string {
	ins := strings.ReplaceAll(dev.Inputs, " ", "")
	split := strings.Split(ins, ",")
	var out []string
	for _, s := range split {
		out = append(out, dev.Name+"."+s)
	}
	return out
}

func (dev *Device) OutputListWithID() []string {
	outs := strings.ReplaceAll(dev.Outputs, " ", "")
	split := strings.Split(outs, ",")
	var out []string
	for _, s := range split {
		out = append(out, dev.Name+"."+s)
	}
	return out
}
