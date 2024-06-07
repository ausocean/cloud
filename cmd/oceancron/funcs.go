/*
AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Cron. Ocean Cron is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean Cron is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with Ocean Cron in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ausocean/cloud/model"
)

// cronFuncs contains our cron extension functions, which are defined below.
var cronFuncs = map[string]func(int64, string) error{
	"check": check,
}

// Device health statuses.
const (
	healthStatusGood    = "good"
	healthStatusBad     = "bad"
	healthStatusUnknown = "unknown"
)

// check is a built-in function for site and/or device checking. If
// mac is specified, it checks just that device. Otherwise, it checks
// all devices for the given site. If any device is not healthy, a
// "site" notification is sent.
func check(skey int64, mac string) error {
	ctx := context.Background()

	name, err := model.GetSiteName(ctx, settingsStore, skey)
	if err != nil {
		return fmt.Errorf("getting site %d failed with error: %v", skey, err)
	}
	devices, err := model.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		return fmt.Errorf("getting devices for site %d failed with error: %v", skey, err)
	}

	type deviceStatus struct {
		name   string
		status string
	}
	h := make(map[string]deviceStatus)
	healthy := true
	mac = strings.ToUpper(mac)

	for _, dev := range devices {
		if mac == "" || mac == dev.MAC() {
			status := checkDevice(ctx, dev)
			if status != healthStatusGood {
				healthy = false
			}
			h[dev.MAC()] = deviceStatus{name: dev.Name, status: status}
		}
	}

	if !healthy {
		var msg string
		if mac != "" {
			msg = fmt.Sprintf("Site %s has unhealthy device: %s (%s)", name, h[mac].name, mac)
		} else {
			msg = fmt.Sprintf("Site %s has unhealthy device(s):", name)
			for k, v := range h {
				msg += fmt.Sprintf("\n%s (%s): %s ", v.name, k, v.status)
			}
		}
		log.Print(msg)
		err := notifier.Send(ctx, skey, "site", msg)
		if err != nil {
			return err
		}
	}

	return nil
}

// checkDevice returns the status of a device, which is determined by
// whether or not the device has responded within that two monitor
// periods.
func checkDevice(ctx context.Context, dev model.Device) string {
	v, err := model.GetVariable(ctx, settingsStore, dev.Skey, "_"+dev.Hex()+".uptime")
	if err != nil {
		return healthStatusUnknown
	}
	if time.Since(v.Updated) < time.Duration(2*dev.MonitorPeriod)*time.Second {
		return healthStatusGood
	}
	return healthStatusBad
}
