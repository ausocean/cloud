/*
DESCRIPTION
  utils.go provides useful utilities and helper functions.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Russell Stanley <russell@ausocean.org>

LICENSE
  Copyright (C) 2021-2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/ausocean/iotsvc/iotds"
	"golang.org/x/net/context"
)

// removeDate removes a date from within a string that matches dd/mm/yyyy or mm/dd/yyyy.
func removeDate(s string) string {
	const dateRegex = "[0-3][0-9]/[0-3][0-9]/(?:[0-9][0-9])?[0-9][0-9]"
	r := regexp.MustCompile(dateRegex)
	return r.ReplaceAllString(s, "")
}

// setActionVars sets vars based on the provided string of "actions" in acts,
// to accomplish setup/shutdown of a device(s) for streaming.
// acts is of form: <device.varname>=<value>,<device.varname>=<value>. For example,
// if we need to turn on a camera and set its mode to normal:
// ESP.CamPower=true,Camera.mode=Normal.
func setActionVars(ctx context.Context, sKey int64, acts string, store iotds.Store) error {
	vars := strings.Split(acts, ",")
	if len(vars) == 0 {
		return errors.New("no var actions to perform")
	}

	for _, v := range vars {
		parts := strings.Split(v, "=")
		if len(parts) != 2 {
			return fmt.Errorf("unexpected actions var format: %s", v)
		}

		err := setVar(ctx, store, parts[0], parts[1], sKey)
		if err != nil {
			return fmt.Errorf("could not set action var %s: %w", parts[0], err)
		}
	}
	return nil
}

// setVar sets cloud variables. These variable are only set if they already exist.
func setVar(ctx context.Context, store iotds.Store, name, value string, sKey int64) error {
	log.Printf("checking %s variable exists", name)
	_, err := iotds.GetVariable(ctx, store, sKey, name)
	if err != nil {
		return fmt.Errorf("could not get %s varable: %w", name, err)
	}

	log.Printf("%s variable exists, setting to value: %s", name, value)
	err = iotds.PutVariable(ctx, store, sKey, name, value)
	if err != nil {
		return fmt.Errorf("could not set %s variable: %w", name, err)
	}
	return nil
}

// broadcastByName gets the broadcast configuration with the provided name from
// the datastore. An error is returned if there's no match or for other issues.
func broadcastByName(sKey int64, name string) (*BroadcastConfig, error) {
	// Load config information for any prior broadcasts that have been saved.
	vars, err := iotds.GetVariablesBySite(context.Background(), settingsStore, sKey, broadcastScope)
	if err != nil {
		return nil, fmt.Errorf("could not get broadcast variables by site: %w", err)
	}
	cfg, err := broadcastFromVars(vars, name)
	if err != nil {
		return nil, fmt.Errorf("could not get the broadcast (%s) from the broadcast vars: %w", name, err)
	}
	return cfg, nil
}

// TODO: document this.
func updateConfigWithTransaction(ctx context.Context, store Store, skey int64, broadcast string, update func(cfg *BroadcastConfig) error) error {
	name := broadcastScope + "." + broadcast
	sep := strings.Index(name, ".")
	if sep >= 0 {
		name = strings.ReplaceAll(name[:sep], ":", "") + name[sep:]
	}
	const typeVariable = "Variable"
	key := store.NameKey(typeVariable, strconv.FormatInt(skey, 10)+"."+name)

	var callBackErr error
	updateConfig := func(ety iotds.Entity) {
		v, ok := ety.(*iotds.Variable)
		if !ok {
			callBackErr = errors.New("could not cast entity to type Variable")
			return
		}

		var cfg BroadcastConfig
		err := json.Unmarshal([]byte(v.Value), &cfg)
		if err != nil {
			callBackErr = fmt.Errorf("could not unmarshal selected broadcast config: %v", err)
			return
		}

		err = update(&cfg)
		if err != nil {
			callBackErr = fmt.Errorf("error from broadcast update callback: %w", err)
			return
		}

		d, err := json.Marshal(cfg)
		if err != nil {
			callBackErr = fmt.Errorf("could not marshal JSON for broadcast save: %w", err)
			return
		}

		v.Value = string(d)
		v.Updated = time.Now()
	}

	err := store.Update(ctx, key, updateConfig, &iotds.Variable{})
	if err != nil {
		return fmt.Errorf("could not update variable: %w", err)
	}

	if callBackErr != nil {
		return fmt.Errorf("error from broadcast update callback: %w", callBackErr)
	}

	return nil
}

type ErrBroadcastNotFound struct{ name string }

func (e ErrBroadcastNotFound) Error() string {
	return fmt.Sprintf("broadcast with name %s doesn't exist", e.name)
}

func (e ErrBroadcastNotFound) Is(target error) bool {
	_, ok := target.(ErrBroadcastNotFound)
	return ok
}

// broadcastFromVars searches a slice of broadcast variables for a broadcast
// config with the provided name and returns if found, otherwise an error is
// returned.
func broadcastFromVars(broadcasts []iotds.Variable, name string) (*BroadcastConfig, error) {
	for _, v := range broadcasts {
		if name == v.Name || name == strings.TrimPrefix(v.Name, broadcastScope+".") {
			var cfg BroadcastConfig
			err := json.Unmarshal([]byte(v.Value), &cfg)
			if err != nil {
				return nil, fmt.Errorf("could not unmarshal selected broadcast config: %v", err)
			}
			return &cfg, nil
		}
	}
	return nil, ErrBroadcastNotFound{name}
}

// getDeviceStatus gets the status of a device given its MAC address.
// The status is determined by checking the uptime variable of the
// device. If the uptime is less than twice the monitor period, the
// device is considered to be sending data and the function returns
// true, otherwise false is returned.
func getDeviceStatus(ctx context.Context, mac int64, store Store) (bool, error) {
	dev, err := iotds.GetDevice(ctx, store, mac)
	if err != nil {
		return false, fmt.Errorf("could not get device: %w", err)
	}
	v, err := iotds.GetVariable(ctx, store, dev.Skey, "_"+dev.Hex()+".uptime")
	if err != nil {
		return false, fmt.Errorf("could not get uptime variable: %w", err)
	}
	if time.Since(v.Updated) < time.Duration(2*int(dev.MonitorPeriod))*time.Second {
		return true, nil
	}
	return false, nil
}

var logConfigs = false

func provideConfig(cfg *BroadcastConfig) string {
	if logConfigs {
		return fmt.Sprintf("%v", trimDescriptionFromConfig(cfg))
	}
	return fmt.Sprintf("(config logging disabled) Events: %v, HardwareState: %v", cfg.Events, cfg.HardwareState)
}

func trimDescriptionFromConfig(cfg *BroadcastConfig) string {
	trimmedConfig := *cfg
	cfg.Description = trimDescriptionChars(trimmedConfig.Description)
	trimmedData, err := json.Marshal(trimmedConfig)
	if err != nil {
		return ""
	}
	return string(trimmedData)
}

func trimDescriptionChars(desc string) string {
	if len(desc) > 80 {
		return desc[:80]
	}
	return desc
}
