/*
DESCRIPTION
  utils.go provides useful utilities and helper functions.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Russell Stanley <russell@ausocean.org>

LICENSE
  Copyright (C) 2021-2023 the Australian Ocean Lab (AusOcean)

  This file is part of VidGrind. VidGrind is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  VidGrind is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with NetReceiver in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/ausocean/iotsvc/iotds"
	"golang.org/x/net/context"
)

// reportError writes an error message to the logs and template.
func reportError(w http.ResponseWriter, r *http.Request, d broadcastRequest, f string, args ...interface{}) {
	msg := fmt.Sprintf(f, args...)
	log.Println(msg)
	writeTemplate(w, r, "broadcast.html", &d, msg)
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

// addDurationToCronSpec is a helper to add a duration less than and not equal to
// 60 minutes to a cron spec.
func addDurationToCronSpec(spec string, dur time.Duration) (string, error) {
	timeStr, err := cronSpecToTime(spec)
	if err != nil {
		return "", fmt.Errorf("could not convert cron spec to time str: %w", err)
	}
	const layoutStr = "15:04"
	t, err := time.Parse(layoutStr, timeStr)
	if err != nil {
		return "", fmt.Errorf("could not parse cam on time: %w", err)
	}
	timeStr = t.Add(dur).Format(layoutStr)
	return timeToCronSpec(timeStr)
}

// registerCallCron registers a cron with action type "call". This will add the
// provided key and function to the cron scheduler's func map, for calling
// according to the cron spec with the provided data.
func registerCallCron(id, spec, key string, cfg *BroadcastConfig, skey int64, store iotds.Store) error {
	bytes, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not marshal broadcast config: %w", err)
	}
	c := iotds.Cron{
		Skey:    skey,
		ID:      id,
		Action:  "call",
		Var:     key, // This func will be called with the cron data as the argument.
		Data:    string(bytes),
		Enabled: true,
	}

	// Parse the time based on the site's timezone.
	site, err := iotds.GetSite(context.Background(), store, skey)
	if err != nil {
		return fmt.Errorf("could not get site: %w", err)
	}
	err = c.ParseTime(spec, site.Timezone)
	if err != nil {
		return fmt.Errorf("could not parse time: %w", err)
	}

	// Add the cron to the scheduler and the database.
	err = iotds.PutCron(context.Background(), store, &c)
	if err != nil {
		return fmt.Errorf("could not put cron in datastore: %w", err)
	}

	err = cronScheduler.Set(&c)
	if err != nil {
		return fmt.Errorf("could not schedule cron: %w", err)
	}

	return nil
}

// cronSpecToTime converts a cron specification string to a time in a 24 hr time
// format (hh:mm).
func cronSpecToTime(cronSpec string) (string, error) {
	fields := strings.Fields(cronSpec)
	if len(fields) != 5 {
		return "", fmt.Errorf("wrong number of fields in cron spec: %s", cronSpec)
	}

	hour, err := strconv.Atoi(fields[1])
	if err != nil {
		return "", fmt.Errorf("invalid hour field: %s", cronSpec)
	}

	minute, err := strconv.Atoi(fields[0])
	if err != nil {
		return "", fmt.Errorf("invalid minute field: %s", cronSpec)
	}

	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return "", fmt.Errorf("cron spec field out of range: %s", cronSpec)
	}

	return fmt.Sprintf("%02d:%02d", hour, minute), nil
}

func timeToCronSpec(timeStr string) (string, error) {
	parsedTime, err := time.Parse("15:04", timeStr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d %d * * *", parsedTime.Minute(), parsedTime.Hour()), nil
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

// isTimeStr returns true if the provided string is a valid 24hr time in format
// hh:mm.
func isTimeStr(str string) bool {
	split := strings.Split(str, ":")
	if len(split) != 2 {
		return false
	}
	hour, err := strconv.Atoi(split[0])
	if err != nil {
		return false
	}
	if 0 > hour || hour > 23 {
		return false
	}
	min, err := strconv.Atoi(split[1])
	if err != nil {
		return false
	}
	if 0 > min || min > 59 {
		return false
	}
	return true
}

// isCronSpec returns true if the provided string is a valid cron spec.
func isCronSpec(str string) bool {
	fields := strings.Fields(str)
	if len(fields) != 5 {
		return false
	}
	for i, field := range fields {
		if !isValidCronField(i, field) {
			return false
		}
	}
	return true
}

// isValidCronField returns true if the provided cron spec field with the provided
// index is valid. The cron spec field refers to one of the 5 space separated fields
// in a cron spec.
func isValidCronField(index int, field string) bool {
	if field == "*" {
		return true
	}
	values := strings.Split(field, "/")
	if len(values) > 2 {
		return false
	}
	if !isValidCronValue(values[0], index) {
		return false
	}
	if len(values) == 2 && !func(step string) bool {
		val, err := strconv.Atoi(step)
		if err != nil {
			return false
		}
		return val > 0
	}(values[1]) {
		return false
	}
	return true
}

// isValidCronValue returns true if the provided cron spec field value with the
// provided index is valid.
func isValidCronValue(value string, index int) bool {
	if strings.Contains(value, ",") {
		parts := strings.Split(value, ",")
		for _, part := range parts {
			if !isValidCronValue(part, index) {
				return false
			}
		}
		return true
	}

	if strings.Contains(value, "-") {
		parts := strings.Split(value, "-")
		if len(parts) != 2 {
			return false
		}
		start, err1 := strconv.Atoi(parts[0])
		end, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return false
		}
		if start > end {
			return false
		}
		return isValidCronRange(index, start, end)
	}

	val, err := strconv.Atoi(value)
	if err != nil {
		return false
	}
	return isValidCronRange(index, val, val)
}

// isValidCronRange returns true if the provided range is valid for the provided
// cron spec field index.
func isValidCronRange(index int, start, end int) bool {
	switch index {
	case 0:
		return start >= 0 && end <= 59
	case 1:
		return start >= 0 && end <= 23
	case 2:
		return start >= 1 && end <= 31
	case 3:
		return start >= 1 && end <= 12
	case 4:
		return start >= 0 && end <= 7
	default:
		return false
	}
}

// hourMinMoreThan returns true if the first time is later than the second time,
// comparing only the hour and minutes of the individual times. We're assuming
// the times are in 24 hour format.
func hourMinMoreThan(a, b time.Time) bool {
	h1, m1 := a.Hour(), a.Minute()
	h2, m2 := b.Hour(), b.Minute()
	return h1 > h2 || (h1 == h2 && m1 > m2)
}

func trimDescriptionChars(desc string) string {
	if len(desc) > 80 {
		return desc[:80]
	}
	return desc
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

var logConfigs = false

func provideConfig(cfg *BroadcastConfig) string {
	if logConfigs {
		return fmt.Sprintf("%v", trimDescriptionFromConfig(cfg))
	}
	return fmt.Sprintf("(config logging disabled) Events: %v, HardwareState: %v", cfg.Events, cfg.HardwareState)
}
