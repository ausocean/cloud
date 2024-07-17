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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

// logForBroadcast logs a message with the broadcast name and ID.
// This is useful to keep track of logs for different broadcasts.
func logForBroadcast(cfg *BroadcastConfig, msg string, args ...interface{}) {
	log.Println(fmtForBroadcastLog(cfg, msg, args...))
}

func fmtForBroadcastLog(cfg *BroadcastConfig, msg string, args ...interface{}) string {
	idArgs := []interface{}{cfg.Name, cfg.ID}
	idArgs = append(idArgs, args...)
	return fmt.Sprintf("(name: %s, id: %s) "+msg, idArgs...)
}

func try(err error, msg string, log func(string, ...interface{})) bool {
	if err != nil {
		log(msg+": %v", err)
		return false
	}
	return true
}

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
func setActionVars(ctx context.Context, sKey int64, acts string, store datastore.Store, log func(string, ...interface{})) error {
	vars := strings.Split(acts, ",")
	if len(vars) == 0 {
		return errors.New("no var actions to perform")
	}

	for _, v := range vars {
		parts := strings.Split(v, "=")
		if len(parts) != 2 {
			return fmt.Errorf("unexpected actions var format: %s", v)
		}

		err := setVar(ctx, store, parts[0], parts[1], sKey, log)
		if err != nil {
			return fmt.Errorf("could not set action var %s: %w", parts[0], err)
		}
	}
	return nil
}

// setVar sets cloud variables. These variable are only set if they already exist.
func setVar(ctx context.Context, store datastore.Store, name, value string, sKey int64, log func(string, ...interface{})) error {
	log("checking %s variable exists", name)
	_, err := model.GetVariable(ctx, store, sKey, name)
	if err != nil {
		return fmt.Errorf("could not get %s varable: %w", name, err)
	}

	log("%s variable exists, setting to value: %s", name, value)
	err = model.PutVariable(ctx, store, sKey, name, value)
	if err != nil {
		return fmt.Errorf("could not set %s variable: %w", name, err)
	}
	return nil
}

// broadcastByName gets the broadcast configuration with the provided name from
// the datastore. An error is returned if there's no match or for other issues.
func broadcastByName(sKey int64, name string) (*BroadcastConfig, error) {
	// Load config information for any prior broadcasts that have been saved.
	vars, err := model.GetVariablesBySite(context.Background(), settingsStore, sKey, broadcastScope)
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
func updateConfigWithTransaction(ctx context.Context, store Store, skey int64, broadcast string, update func(cfg *BroadcastConfig)) error {
	name := broadcastScope + "." + broadcast
	sep := strings.Index(name, ".")
	if sep >= 0 {
		name = strings.ReplaceAll(name[:sep], ":", "") + name[sep:]
	}
	const typeVariable = "Variable"
	key := store.NameKey(typeVariable, strconv.FormatInt(skey, 10)+"."+name)

	var callBackErr error
	updateConfig := func(ety datastore.Entity) {
		v, ok := ety.(*model.Variable)
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

		update(&cfg)

		d, err := json.Marshal(cfg)
		if err != nil {
			callBackErr = fmt.Errorf("could not marshal JSON for broadcast save: %w", err)
			return
		}

		v.Value = string(d)
		v.Updated = time.Now()
	}

	err := store.Update(ctx, key, updateConfig, &model.Variable{})
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		err = store.Create(ctx, key, &model.Variable{})
		if err != nil {
			return fmt.Errorf("could not create broadcast variable: %w", err)
		}

		// Since the entity doesn't already exist, we need to change the updateConfig function to update
		// a blank config.
		updateConfig = func(ety datastore.Entity) {
			v, ok := ety.(*model.Variable)
			if !ok {
				callBackErr = errors.New("could not cast entity to type Variable")
				return
			}

			cfg := &BroadcastConfig{}
			update(cfg)

			v.Skey = skey
			v.Name = name
			v.Scope = broadcastScope

			d, err := json.Marshal(cfg)
			if err != nil {
				callBackErr = fmt.Errorf("could not marshal JSON for broadcast save: %w", err)
				return
			}

			v.Value = string(d)
			v.Updated = time.Now()
		}

		err = store.Update(ctx, key, updateConfig, &model.Variable{})
		if err != nil {
			return fmt.Errorf("could not update broadcast variable after creation: %w", err)
		}
	} else if err != nil {
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
func broadcastFromVars(broadcasts []model.Variable, name string) (*BroadcastConfig, error) {
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
