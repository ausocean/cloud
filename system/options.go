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
	"fmt"
	"reflect"

	"github.com/ausocean/cloud/model"
)

// Option represents functional options that can be passed to NewRigSystem.
type Option func(any) error

// variableHolder is an interface for types which can add variables.
type variableHolder interface {
	AddVariables(variables ...*model.Variable)
}

// WithVariables is a functional option that adds the passed variables to the RigSystem.
func WithVariables(variables ...*model.Variable) func(any) error {
	return func(v any) error {
		vh, ok := v.(variableHolder)
		if !ok {
			return fmt.Errorf("%v does not implement variableHolder interface", reflect.TypeOf(v).String())
		}
		vh.AddVariables(variables...)
		return nil
	}
}

// wifiHolder is an interface for types which can add wifi.
type wifiHolder interface {
	SetWifi(ssid, pass string)
}

// WithWifi is a functional option that sets the wifi name and password
// for a device.
func WithWifi(ssid, pass string) func(any) error {
	return func(v any) error {
		wh, ok := v.(wifiHolder)
		if !ok {
			return fmt.Errorf("%v does not implement wifiHolder interface", reflect.TypeOf(v).String())
		}
		if ssid == "" {
			return nil
		}
		wh.SetWifi(ssid, pass)
		return nil
	}
}

// locationHolder is an interface for types which can set their location.
type locationHolder interface {
	SetLocation(lat, long float64)
}

// WithLocation is a functional option which sets the latitude and longitude.
func WithLocation(lat, long float64) func(any) error {
	return func(v any) error {
		lh, ok := v.(locationHolder)
		if !ok {
			return fmt.Errorf("%v does not implement wifiHolder interface", reflect.TypeOf(v).String())
		}
		if lat <= -90 || lat >= 90 || long <= -180 || long >= 180 {
			return model.ErrInvalidLocation
		}
		lh.SetLocation(lat, long)
		return nil
	}
}
