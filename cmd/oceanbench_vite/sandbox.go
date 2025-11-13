/*
AUTHORS
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2019-2025 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Bench. Ocean Bench is free software: you can
  redistribute it and/or modify it under the terms of the GNU General
  Public License as published by the Free Software Foundation, either
  version 3 of the License, or (at your option) any later version.

  This software is distributed in the hope that it will be useful, but
  WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
  General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt.  If not, see
  http://www.gnu.org/licenses/.
*/

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/system"
)

type sandboxData struct {
	Devices []model.Device
	Mac     string
	Device  model.Device
	commonData
}

func sandboxHandler(w http.ResponseWriter, r *http.Request) {
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	skey, _ := profileData(profile)

	data := sandboxData{
		commonData: commonData{
			Pages:   pages("home"),
			Profile: profile,
		},
	}

	if skey != model.SandboxSkey {
		writeTemplate(w, r, "sandbox.html", &data, "Must be on Sandbox Site.")
		return
	}

	ctx := context.Background()
	data.Devices, err = model.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		writeTemplate(w, r, "sandbox.html", &data, "could not get devices by site")
		return
	}

	ma := r.FormValue("ma")
	if !model.IsMacAddress(ma) && ma != "" {
		writeTemplate(w, r, "sandbox.html", &data, "invalid mac address")
		return
	}

	for _, d := range data.Devices {
		if d.MAC() == ma {
			data.Device = d
			break
		}
	}

	writeTemplate(w, r, "sandbox.html", &data, "")
	return
}

// configDevicesHandler handles configuration of new devices.
//
// Form Fields:
//
//	dn = Name of the new device
//	ma = MAC address
//	dt = device type
//	wi = comma seperated WiFi name and password (optional)
//	ll = comma seperated latitude and longitude (optional)
//	sk = target site key for the new device
func configDevicesHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	ctx := context.Background()
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}

	if r.Method == http.MethodGet {
		writeConfigure(w, r, profile)
		return
	}

	// Parse the form values.
	dn := r.FormValue("dn")
	ma := r.FormValue("ma")
	dt := r.FormValue("dt")
	wifi := r.FormValue("wi")
	ll := r.FormValue("ll")
	sk := r.FormValue("sk")
	r.ParseForm()

	dev, err := model.GetDevice(ctx, settingsStore, model.MacEncode(ma))
	if err != nil {
		writeError(w, fmt.Errorf("unable to get device by mac: %w", err))
		return
	}

	// Parse location.
	var lat, long float64
	if len(strings.Split(ll, ",")) == 2 {
		lat, err = strconv.ParseFloat(strings.Split(ll, ",")[0], 64)
		if err != nil {
			writeError(w, fmt.Errorf("unable to parse lat float64 from: %s, err: %w", strings.Split(ll, ",")[0], err))
			return
		}
		long, err = strconv.ParseFloat(strings.Split(ll, ",")[1], 64)
		if err != nil {
			writeError(w, fmt.Errorf("unable to parse long float64 from: %s, err: %w", strings.Split(ll, ",")[1], err))
			return
		}
	}

	// Parse Wifi.
	var ssid, pass string
	if wifiSplit := strings.Split(wifi, ","); len(wifiSplit) == 2 {
		ssid = wifiSplit[0]
		pass = wifiSplit[1]
	}

	if !model.IsMacAddress(ma) {
		writeError(w, model.ErrInvalidMACAddress)
		return
	}

	var isValidType bool
	for _, t := range devTypes {
		if dt == t {
			isValidType = true
			break
		}
	}
	if !isValidType {
		writeError(w, model.ErrInvalidDevType)
		return
	}
	skey, err := strconv.ParseInt(sk, 10, 64)
	if err != nil {
		writeError(w, fmt.Errorf("could not parse site key: %w", err))
		return
	}

	// Create the device.
	switch dt {
	case model.DevTypeController:
		// Create a controller with all default values defined in rig_system.go.
		sys, err := system.NewRigSystem(skey, dev.Dkey, ma, dn,
			system.WithRigSystemDefaults(),
			system.WithWifi(ssid, pass),
			system.WithLocation(lat, long),
		)
		if err != nil {
			writeError(w, err)
			return
		}

		err = system.PutRigSystem(ctx, settingsStore, sys)
		if err != nil {
			writeError(w, fmt.Errorf("unable to put rig system: %w", err))
			return
		}
	case model.DevTypeCamera:
		camSys, err := system.NewCamera(skey, dev.Dkey, dn, ma, system.WithCameraDefaults())
		if err != nil {
			writeError(w, err)
			return
		}

		err = system.PutCameraSystem(ctx, settingsStore, camSys)
		if err != nil {
			writeError(w, fmt.Errorf("unable to put camera system: %w", err))
			return
		}

	default:
		writeError(w, errNotImplemented)
		return
	}
	site, err := model.GetSite(ctx, settingsStore, int64(skey))
	if err != nil {
		writeError(w, fmt.Errorf("failed to get site: %v", err))
		return
	}
	profile.Data = fmt.Sprintf("%d:%s", skey, site.Name)
	err = putProfileData(w, r, profile.Data)
	if err != nil {
		writeError(w, fmt.Errorf("failed to put profile data: %w", err))
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/set/devices?ma=%s", ma), http.StatusSeeOther)
}

type configureData struct {
	MAC      string
	DevTypes []string
	Sites    []model.Site
	commonData
}

func writeConfigure(w http.ResponseWriter, r *http.Request, profile *gauth.Profile) {
	data := configureData{
		commonData: commonData{
			Pages: pages("devices"),
		}}
	ctx := r.Context()
	var err error

	data.Sites, err = model.GetAllSites(ctx, settingsStore)
	if err != nil {
		writeTemplate(w, r, "configure.html", &data, fmt.Sprintf("could not get all sites: %v", err.Error()))
		return
	}

	// Parse form values.
	data.MAC = r.FormValue("ma")
	if data.MAC == "" {
		// TODO: Allow creation of new device from Sandbox page.
		http.Redirect(w, r, "/admin/sandbox", http.StatusFound)
		return
	}
	data.DevTypes = devTypes
	r.ParseForm()

	writeTemplate(w, r, "configure.html", &data, "")
}
