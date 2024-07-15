/*
AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean).

  This file is free software: you can redistribute it and/or modify it
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

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ausocean/openfish/datastore"
)

// This file contain definitions of legacty and temporary entities
// required for datastore migrations.

// Site entities.
const (
	typeSiteV1 = "SiteV1"
	typeSiteV2 = "SiteV2"
	typeSiteV3 = "SiteV3"
)

// SiteV1 represents a NetReceiver site (deprecated).
//
// Comments indicate the original lower case names.
// Properties not migrated: user_phone, notify_phone, country
type SiteV1 struct {
	Skey         int64     // Was skey
	Name         string    // Was name
	OwnerEmail   string    // Was user_email
	Latitude     float64   // Was latitude
	Longitude    float64   // Was longtitude
	Timezone     float64   // Was tz_offset
	NotifyPeriod int64     // Was notify_period
	Enabled      bool      // was enabled
	Confirmed    bool      // Was confirmed
	Premium      bool      // Was premium
	Public       bool      // Was public
	Created      time.Time // Was created
}

// Load implements datastore.LoadSaver.Load for SiteV1 and maps the
// old lowcase name property names to the new titlecase names.
func (site *SiteV1) Load(ps []datastore.Property) error {
	for _, p := range ps {
		var ok bool
		switch p.Name {
		case "skey":
			site.Skey, ok = p.Value.(int64)
		case "name":
			site.Name, ok = p.Value.(string)
		case "user_email":
			site.OwnerEmail, ok = p.Value.(string)
		case "latitude":
			site.Latitude, ok = p.Value.(float64)
		case "longitude":
			site.Longitude, ok = p.Value.(float64)
		case "tz_offset":
			site.Timezone, ok = p.Value.(float64)
		case "notify_period":
			site.NotifyPeriod, ok = p.Value.(int64)
		case "enabled":
			site.Enabled, ok = p.Value.(bool)
		case "confirmed":
			site.Confirmed, ok = p.Value.(bool)
		case "premium":
			site.Premium, ok = p.Value.(bool)
		case "public":
			site.Public, ok = p.Value.(bool)
		case "created":
			site.Created, ok = p.Value.(time.Time)
		default:
			continue
		}
		if !ok {
			return errors.New("Unexpected type for Site." + p.Name)
		}
	}
	return nil
}

func (site *SiteV1) Save() ([]datastore.Property, error) {
	return nil, datastore.ErrUnimplemented
}

func (site *SiteV1) Encode() []byte {
	return []byte{}
}

func (site *SiteV1) Decode(b []byte) error {
	return datastore.ErrUnimplemented
}

func (site *SiteV1) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

func (site *SiteV1) GetCache() datastore.Cache {
	return nil
}

// SiteV2 represents a site migrated from V1.
type SiteV2 struct {
	Skey         int64
	Name         string
	OwnerEmail   string
	Latitude     float64
	Longitude    float64
	Timezone     float64
	NotifyPeriod int64
	Enabled      bool
	Confirmed    bool
	Premium      bool
	Public       bool
	Created      time.Time
}

func (site *SiteV2) Encode() []byte {
	return []byte{}
}

func (site *SiteV2) Decode(b []byte) error {
	return datastore.ErrUnimplemented
}

func (site *SiteV2) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

func (site *SiteV2) GetCache() datastore.Cache {
	return nil
}

// SiteV3 represents a site migrated from V2.
type SiteV3 struct {
	Skey         int64
	Name         string
	Description  string
	OrgID        string
	OwnerEmail   string
	OpsEmail     string
	YouTubeEmail string
	Latitude     float64
	Longitude    float64
	Timezone     float64
	NotifyPeriod int64
	Enabled      bool
	Confirmed    bool
	Premium      bool
	Public       bool
	Subscribed   time.Time
	Created      time.Time
}

func (site *SiteV3) Encode() []byte {
	return []byte{}
}

func (site *SiteV3) Decode(b []byte) error {
	return datastore.ErrUnimplemented
}

func (site *SiteV3) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

func (site *SiteV3) GetCache() datastore.Cache {
	return nil
}

// Device entities.
const (
	typeDeviceV1 = "DeviceV1"
	typeDeviceV2 = "DeviceV2"
)

// DeviceV1 represents a V1 device.
type DeviceV1 struct {
	Skey          int64             // Site key.
	Dkey          int64             // Device key.
	Mac           int64             // Encoded MAC address (immutable).
	Did           string            // Device name.
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

// Encode serializes a device into JSON.
func (dev *DeviceV1) Encode() []byte {
	bytes, _ := json.Marshal(dev)
	return bytes
}

// Decode deserializes a device from JSON.
func (dev *DeviceV1) Decode(b []byte) error {
	return json.Unmarshal(b, dev)
}

func (dev *DeviceV1) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

func (dev *DeviceV1) GetCache() datastore.Cache {
	return nil
}

func (dev *DeviceV1) Hex() string {
	return fmt.Sprintf("%012x", dev.Mac)
}

// DeviceV2 is identical to Device except Did is renamed Name.
type DeviceV2 struct {
	Mac           int64             // Encoded MAC address (immutable).
	Name          string            // Device name.
	Skey          int64             // Site key.
	Dkey          int64             // Device key.
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

// Encode serializes a device into JSON.
func (dev *DeviceV2) Encode() []byte {
	bytes, _ := json.Marshal(dev)
	return bytes
}

// Decode deserializes a device from JSON.
func (dev *DeviceV2) Decode(b []byte) error {
	return json.Unmarshal(b, dev)
}

func (dev *DeviceV2) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

func (dev *DeviceV2) GetCache() datastore.Cache {
	return nil
}

// Cron entities.
const (
	typeCronV1 = "CronV1"
	typeCronV2 = "CronV2"
)

// CronV1 represents an old (NetReceiver) Cron (deprecated).
type CronV1 struct {
	Skey    int64     // Site key.
	ID      string    // Cron ID.
	Time    time.Time // Cron time.
	TOD     string    // Symbolic time of day, e.g., "Sunset", or repeating time "*30".
	Repeat  bool      // True if repeating time.
	Minutes int64     // Minutes since start of UTC day or repeat minutes.
	Action  string    // Action to be performed
	Var     string    // Action variable (if any).
	Data    string    // Action data (if any).
	Enabled bool      // True if enabled, false otherwise.
}

// Load implements datastore.LoadSaver.Load for CronV1 and maps the
// old lowcase name property names to the new titlecase names.
func (c *CronV1) Load(ps []datastore.Property) error {
	for _, p := range ps {
		var ok bool
		switch p.Name {
		case "skey":
			c.Skey, ok = p.Value.(int64)
		case "cid":
			c.ID, ok = p.Value.(string)
		case "time":
			c.Time, ok = p.Value.(time.Time)
		case "tod":
			c.TOD, ok = p.Value.(string)
			if !ok {
				c.TOD = ""
				ok = true
			}
		case "repeat":
			c.Repeat, ok = p.Value.(bool)
		case "minutes":
			c.Minutes, ok = p.Value.(int64)
		case "action":
			c.Action, ok = p.Value.(string)
		case "var":
			c.Var, ok = p.Value.(string)
		case "data":
			c.Data, ok = p.Value.(string)
		case "enabled":
			c.Enabled, ok = p.Value.(bool)
		default:
			continue
		}
		if !ok {
			return errors.New("Unexpected type for Cron." + p.Name)
		}
	}
	return nil
}

func (c *CronV1) Save() ([]datastore.Property, error) {
	return nil, datastore.ErrUnimplemented
}

func (c *CronV1) Encode() []byte {
	return []byte{}
}

func (c *CronV1) Decode(b []byte) error {
	return datastore.ErrUnimplemented
}

func (c *CronV1) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

func (c *CronV1) GetCache() datastore.Cache {
	return nil
}

// CronV2 represents the latet version of a cron.
type CronV2 struct {
	Skey    int64     // Site key.
	ID      string    // Cron ID.
	Time    time.Time // Cron time.
	TOD     string    // Symbolic time of day, e.g., "Sunset", or repeating time "*30".
	Repeat  bool      // True if repeating time.
	Minutes int64     // Minutes since start of UTC day or repeat minutes.
	Action  string    // Action to be performed
	Var     string    // Action variable (if any).
	Data    string    `datastore:",noindex"` // Action data (if any).
	Enabled bool      // True if enabled, false otherwise.
}

func (c *CronV2) Save() ([]datastore.Property, error) {
	return nil, datastore.ErrUnimplemented
}

func (c *CronV2) Encode() []byte {
	return []byte{}
}

func (c *CronV2) Decode(b []byte) error {
	return datastore.ErrUnimplemented
}

func (c *CronV2) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

func (c *CronV2) GetCache() datastore.Cache {
	return nil
}
