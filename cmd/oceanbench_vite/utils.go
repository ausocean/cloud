/*
DESCRIPTION
  utils.go provides useful utilities and helper functions.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Russell Stanley <russell@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2021-2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Bench. Ocean Bench is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean Bench is distributed in the hope that it will be useful,
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
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/model"
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

// timeToCronSpec converts a 24 hour time to a cron spec.
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

// toJSON
func toJSON(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		// Handle the error by returning a string representation of the error.
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(bytes)
}
