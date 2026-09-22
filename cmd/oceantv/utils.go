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
	"strings"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/model"
)

func try(err error, msg string, log func(string, ...interface{})) bool {
	if err != nil {
		log(msg+": %v", err)
		return false
	}
	return true
}

// setActionVars sets vars based on the provided ordered action vars in acts,
// to accomplish setup/shutdown of a device(s) for streaming. Each ActionVar
// holds a variable name and the value to set it to, for example:
// {Name: "ESP.CamPower", Value: "true"}. Actions are performed in order.
func setActionVars(ctx Ctx, sKey int64, acts []ActionVar, store Store, log func(string, ...interface{})) error {
	if len(acts) == 0 {
		return errors.New("no var actions to perform")
	}

	for _, v := range acts {
		if v.Name == "" {
			return errors.New("unexpected actions var with empty name")
		}

		err := setVar(ctx, store, v.Name, v.Value, sKey, log)
		if err != nil {
			return fmt.Errorf("could not set action var %s: %w", v.Name, err)
		}
	}
	return nil
}

// setVar sets cloud variables. These variable are only set if they already exist.
func setVar(ctx Ctx, store Store, name, value string, sKey int64, log func(string, ...interface{})) error {
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
func broadcastByName(sKey int64, name string) (*Cfg, error) {
	// Load config information for any prior broadcasts that have been saved.
	vars, err := model.GetVariablesBySite(context.Background(), store, sKey, broadcast.Scope)
	if err != nil {
		return nil, fmt.Errorf("could not get broadcast variables by site: %w", err)
	}
	cfg, err := broadcastFromVars(vars, name)
	if err != nil {
		return nil, fmt.Errorf("could not get the broadcast (%s) from the broadcast vars: %w", name, err)
	}
	return cfg, nil
}

// broadcastFromVars searches a slice of broadcast variables for a broadcast
// config with the provided name and returns if found, otherwise an error is
// returned.
func broadcastFromVars(broadcasts []model.Variable, name string) (*Cfg, error) {
	for _, v := range broadcasts {
		if name == v.Name || name == strings.TrimPrefix(v.Name, broadcast.Scope+".") {
			var cfg Cfg
			err := json.Unmarshal([]byte(v.Value), &cfg)
			if err != nil {
				return nil, fmt.Errorf("could not unmarshal selected broadcast config: %v", err)
			}
			return &cfg, nil
		}
	}
	return nil, broadcast.ErrBroadcastNotFound{name}
}

var logConfigs = false

func provideConfig(cfg *Cfg) string {
	if logConfigs {
		return fmt.Sprintf("%v", trimDescriptionFromConfig(cfg))
	}
	return fmt.Sprintf("(config logging disabled) Events: %v, HardwareState: %v", cfg.Events, cfg.HardwareState)
}

func trimDescriptionFromConfig(cfg *Cfg) string {
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
