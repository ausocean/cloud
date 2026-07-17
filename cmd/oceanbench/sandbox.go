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
	"github.com/gofiber/fiber/v2"
)

type sandboxData struct {
	Devices []model.Device
	Mac     string
	Device  model.Device
	commonData
}

func sandboxHandler(c *fiber.Ctx) error {
	profile, err := getProfile(c)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		return c.Redirect("/", fiber.StatusUnauthorized)
	}
	skey, _ := requestSiteData(c, profile)

	data := sandboxData{
		commonData: commonData{
			Pages:   pages(c, "home"),
			Profile: profile,
		},
	}

	if skey != model.SandboxSkey {
		writeTemplate(c, "sandbox.html", &data, "Must be on Sandbox Site.")
		return nil
	}

	ctx := context.Background()
	data.Devices, err = model.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		writeTemplate(c, "sandbox.html", &data, "could not get devices by site")
		return err
	}

	ma := c.FormValue("ma")
	if !model.IsMacAddress(ma) && ma != "" {
		writeTemplate(c, "sandbox.html", &data, "invalid mac address")
		return nil
	}

	for _, d := range data.Devices {
		if d.MAC() == ma {
			data.Device = d
			break
		}
	}

	writeTemplate(c, "sandbox.html", &data, "")
	return nil
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
func configDevicesHandler(c *fiber.Ctx) error {
	logRequest(c)
	ctx := context.Background()
	profile, err := getProfile(c)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		return c.Redirect("/", fiber.StatusUnauthorized)
	}

	if c.Method() == http.MethodGet {
		return writeConfigure(c, profile)
	}

	// Parse the form values.
	dn := c.FormValue("dn")
	ma := c.FormValue("ma")
	dt := c.FormValue("dt")
	wifi := c.FormValue("wi")
	ll := c.FormValue("ll")
	sk := c.FormValue("sk")

	dev, err := model.GetDevice(ctx, settingsStore, model.MacEncode(ma))
	if err != nil {
		writeError(c, fmt.Errorf("unable to get device by mac: %w", err))
		return err
	}

	// Parse location.
	var lat, long float64
	if len(strings.Split(ll, ",")) == 2 {
		lat, err = strconv.ParseFloat(strings.Split(ll, ",")[0], 64)
		if err != nil {
			writeError(c, fmt.Errorf("unable to parse lat float64 from: %s, err: %w", strings.Split(ll, ",")[0], err))
			return err
		}
		long, err = strconv.ParseFloat(strings.Split(ll, ",")[1], 64)
		if err != nil {
			writeError(c, fmt.Errorf("unable to parse long float64 from: %s, err: %w", strings.Split(ll, ",")[1], err))
			return err
		}
	}

	// Parse Wifi.
	var ssid, pass string
	if wifiSplit := strings.Split(wifi, ","); len(wifiSplit) == 2 {
		ssid = wifiSplit[0]
		pass = wifiSplit[1]
	}

	if !model.IsMacAddress(ma) {
		writeError(c, model.ErrInvalidMACAddress)
		return model.ErrInvalidMACAddress
	}

	var isValidType bool
	for _, t := range devTypes {
		if dt == t {
			isValidType = true
			break
		}
	}
	if !isValidType {
		writeError(c, model.ErrInvalidDevType)
		return model.ErrInvalidDevType
	}
	skey, err := strconv.ParseInt(sk, 10, 64)
	if err != nil {
		writeError(c, fmt.Errorf("could not parse site key: %w", err))
		return err
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
			writeError(c, err)
			return err
		}

		err = system.PutRigSystem(ctx, settingsStore, sys)
		if err != nil {
			writeError(c, fmt.Errorf("unable to put rig system: %w", err))
			return err
		}
	case model.DevTypeCamera:
		camSys, err := system.NewCamera(skey, dev.Dkey, dn, ma, system.WithCameraDefaults())
		if err != nil {
			writeError(c, err)
			return err
		}

		err = system.PutCameraSystem(ctx, settingsStore, camSys)
		if err != nil {
			writeError(c, fmt.Errorf("unable to put camera system: %w", err))
			return err
		}

	default:
		writeError(c, errNotImplemented)
		return errNotImplemented
	}
	site, err := model.GetSite(ctx, settingsStore, int64(skey))
	if err != nil {
		writeError(c, fmt.Errorf("failed to get site: %v", err))
		return err
	}
	profile.Data = fmt.Sprintf("%d:%s", skey, site.Name)
	err = putProfileData(c, profile.Data)
	if err != nil {
		writeError(c, fmt.Errorf("failed to put profile data: %w", err))
		return err
	}

	return c.Redirect(fmt.Sprintf("/set/devices?ma=%s", ma), fiber.StatusSeeOther)
}

type configureData struct {
	MAC      string
	DevTypes []string
	Sites    []model.Site
	commonData
}

func writeConfigure(c *fiber.Ctx, profile *gauth.Profile) error {
	data := configureData{
		commonData: commonData{
			Pages: pages(c, "devices"),
		}}
	ctx := c.UserContext()
	var err error

	data.Sites, err = model.GetAllSites(ctx, settingsStore)
	if err != nil {
		writeTemplate(c, "configure.html", &data, fmt.Sprintf("could not get all sites: %v", err.Error()))
		return err
	}

	// Parse form values.
	data.MAC = c.FormValue("ma")
	if data.MAC == "" {
		// TODO: Allow creation of new device from Sandbox page.
		return c.Redirect("/fiber/sandbox", fiber.StatusFound)
	}
	data.DevTypes = devTypes

	writeTemplate(c, "configure.html", &data, "")
	return nil
}
