/*
NAME
  Ocean Bench admin functions functions.
  Much of this functionality has moved to the dsadmin utility.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2018-2024 the Australian Ocean Lab (AusOcean)

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
  in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

// struct role maps NetReceiver and Ocean Bench permissions.
type role struct {
	Name string
	Perm int64
}

// adminData stores the data served to the admin site page.
type adminData struct {
	Skey      int64
	Site      *model.Site
	SiteUsers []model.User
	Roles     []role
	commonData
}

// utilsData stores the data served to the admin utils page.
type utilsData struct {
	Ma, Sn  string
	Sites   []model.Site
	Devices []model.Device
	Info    map[string]string
	commonData
}

// adminHandler performs various admin tasks.
func adminHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	p, err := getProfile(w, r)
	switch {
	case err != nil && !errors.Is(err, gauth.TokenNotFound):
		log.Printf("authentication error: %v", err)
		fallthrough
	case err != nil:
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	setup(ctx)

	// Adding a new site requires the user to be a super admin.
	if r.URL.Path == "/admin/site/add" {
		if !isSuperAdmin(p.Email) {
			http.Redirect(w, r, "/", http.StatusUnauthorized)
			return
		}
		data := adminData{
			commonData: commonData{
				Pages:   pages("admin"),
				Profile: p,
			},
		}

		switch r.Method {
		case "GET":
			writeTemplate(w, r, "register.html", &data, "")

		case "POST":
			err = addSite(w, r, p)
			if err != nil {
				writeTemplate(w, r, "register.html", &data, err.Error())
			} else {
				http.Redirect(w, r, "/admin", http.StatusFound)
			}

		default:
			http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
		}
		return
	}

	// Require POST method, except for admin landing pages.
	if r.Method != "POST" {
		switch r.URL.Path {
		case "/admin/site", "/admin/broadcast", "/admin/missioncontrol", "/admin/utils":
			// Okay.
		default:
			http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
			return
		}
	}

	// The following tasks all require admin privilege.
	skey, _ := profileData(p)
	if !isAdmin(ctx, skey, p.Email) {
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}

	switch r.URL.Path {
	case "/admin/site/update":
		err = updateSite(w, r, p)

	case "/admin/site/delete":
		err = deleteSite(w, r, p)
		if err == nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

	case "/admin/user/add", "/admin/user/update":
		err = updateUser(w, r, p)

	case "/admin/user/delete":
		err = deleteUser(w, r, p)

	case "/admin/broadcast":
		broadcastHandler(w, r)
		return

	case "/admin/missioncontrol":
		missionControlHandler(w, r)
		return

	case "/admin/utils":
		utilsHandler(w, r, p)
		return

	case "/admin/site":
		err = nil // Just render the admin page.

	default:
		err = errors.New("invalid request: " + r.URL.Path)
	}

	writeAdmin(w, r, p, err)
}

// addSite creates a new site and its first associated admin user.
// Sites are created as private sites initially.
func addSite(w http.ResponseWriter, r *http.Request, p *gauth.Profile) error {
	ctx := r.Context()

	// Create a new site, with just a name.
	sn := r.FormValue("sn")
	if sn == "" {
		return errors.New("empty site name")
	}

	var skey int64
	for {
		// Create a random 31-bit number that is at least 10 digits.
		skey = rand.Int63n((1<<31)-1000000000) + 1000000000
		site := model.Site{Skey: skey, Name: sn, Public: false, Enabled: true}
		err := model.CreateSite(ctx, settingsStore, &site)
		if err == nil {
			break
		}
		if err != datastore.ErrEntityExists {
			return fmt.Errorf("cannot create site: %w", err)
		}
	}

	// Create an admin user for the new site.
	user := model.User{Skey: skey, Email: p.Email, Perm: model.ReadPermission | model.WritePermission | model.AdminPermission, Created: time.Now()}
	err := model.PutUser(ctx, settingsStore, &user)
	if err != nil {
		return fmt.Errorf("cannot create user: %w", err)
	}

	putProfileData(w, r, strconv.FormatInt(skey, 10)+":"+sn)

	return nil
}

// location represents a latitude, longitude, altitude tuple.
type location struct{ Lat, Lng, Alt float64 }

// parseLocation parses a location string or returns an error otherwise.
func parseLocation(s string) (location, error) {
	s = strings.ReplaceAll(s, " ", "")
	ll := strings.Split(s, ",")
	var loc location
	if len(ll) < 2 || len(ll) > 3 {
		return loc, errors.New("expected comma-separated latitude,longitude[,altitude]")
	}
	var err error
	loc.Lat, err = strconv.ParseFloat(ll[0], 64)
	if err != nil || loc.Lat < -90 || loc.Lat > 90 {
		return loc, errors.New("invalid latitude: " + ll[0])
	}
	loc.Lng, err = strconv.ParseFloat(ll[1], 64)
	if err != nil || loc.Lng < -180 || loc.Lng > 180 {
		return loc, errors.New("invalid longitude: " + ll[1])
	}
	if len(ll) == 3 {
		loc.Alt, err = strconv.ParseFloat(ll[2], 64)
		if err != nil {
			return loc, errors.New("invalid altitude: " + ll[2])
		}
	}
	return loc, nil
}

// updateSite updates an existing site.
// Parameter names conform to standard NetReceiver JSON values described at
// https://netreceiver.appspot.com/help/json
func updateSite(w http.ResponseWriter, r *http.Request, p *gauth.Profile) error {
	skey, _ := profileData(p)
	name := r.FormValue("sn")
	if name == "" {
		return errors.New("empty site name")
	}
	desc := r.FormValue("sd")
	org := r.FormValue("org")
	tz, err := strconv.ParseFloat(r.FormValue("tz"), 64)
	if err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	ll, err := parseLocation(r.FormValue("ll"))
	if err != nil {
		return fmt.Errorf("invalid location: %w", err)
	}
	ops := r.FormValue("ops")
	yt := r.FormValue("yt")
	np, err := strconv.ParseInt(r.FormValue("np"), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid notify period: %w", err)
	}
	pb := r.FormValue("pb") != ""
	cf := r.FormValue("cf") != ""
	en := r.FormValue("en") != ""

	ctx := r.Context()
	site, err := model.GetSite(ctx, settingsStore, skey)
	if err != nil {
		return fmt.Errorf("cannot get site: %w", err)
	}

	site.Skey = skey // Immutable!
	site.Name = name
	site.Description = desc
	site.OrgID = org
	site.Timezone = tz
	site.Latitude = ll.Lat
	site.Longitude = ll.Lng
	site.OpsEmail = ops
	site.YouTubeEmail = yt
	site.NotifyPeriod = np
	site.Public = pb
	site.Confirmed = cf
	site.Enabled = en
	err = model.PutSite(ctx, settingsStore, site)
	if err != nil {
		return fmt.Errorf("cannot put site: %w", err)
	}

	return nil
}

// deleteSite deletes the current site and all associated users.
func deleteSite(w http.ResponseWriter, r *http.Request, p *gauth.Profile) error {
	skey, _ := profileData(p)
	ctx := r.Context()

	err := model.DeleteSite(ctx, settingsStore, skey)
	if err != nil {
		return fmt.Errorf("cannot delete site: %w", err)
	}

	users, err := model.GetUsersBySite(ctx, settingsStore, skey)
	if err != nil {
		return fmt.Errorf("cannot get users: %w", err)
	}

	for _, user := range users {
		err = model.DeleteUser(ctx, settingsStore, skey, user.Email)
		if err != nil {
			return fmt.Errorf("cannot delete user: %w", err)
		}
	}

	putProfileData(w, r, "") // Deselect the site.

	return nil
}

// updateUser creates or updates a site user.
func updateUser(w http.ResponseWriter, r *http.Request, p *gauth.Profile) error {
	skey, _ := profileData(p)

	email := r.FormValue("email")
	perm, err := strconv.ParseInt(r.FormValue("perm"), 10, 64)
	if err != nil {
		return fmt.Errorf("cannot parse permission: %w", err)
	}
	user := model.User{Skey: skey, Email: email, Perm: perm, Created: time.Now()}
	err = model.PutUser(r.Context(), settingsStore, &user)
	if err != nil {
		return fmt.Errorf("cannot put user: %w", err)
	}

	return nil
}

// deleteUser deletes a site user.
func deleteUser(w http.ResponseWriter, r *http.Request, p *gauth.Profile) error {
	skey, _ := profileData(p)

	email := r.FormValue("email")
	err := model.DeleteUser(r.Context(), settingsStore, skey, email)
	if err != nil {
		return fmt.Errorf("cannot delete user: %w", err)
	}

	return nil
}

// writeAdmin writes the admin page.
func writeAdmin(w http.ResponseWriter, r *http.Request, p *gauth.Profile, err error) {
	skey, _ := profileData(p)

	data := adminData{
		commonData: commonData{
			Pages:   pages("site"),
			Profile: p,
		},
		Skey: skey,
		Roles: []role{
			{
				Name: "none",
				Perm: 0,
			},
			{
				Name: "read",
				Perm: model.ReadPermission,
			},
			{
				Name: "write",
				Perm: model.ReadPermission | model.WritePermission,
			},
			{
				Name: "admin",
				Perm: model.ReadPermission | model.WritePermission | model.AdminPermission,
			},
		},
	}
	var msg string
	if err != nil {
		msg = err.Error()
	}

	ctx := r.Context()

	data.Site, err = model.GetSite(ctx, settingsStore, skey)
	if err != nil {
		log.Printf("GetSite error: %v", err)
	}
	data.SiteUsers, err = model.GetUsersBySite(ctx, settingsStore, skey)
	if err != nil {
		log.Printf("GetUsersBySite error: %v", err)
	}

	writeTemplate(w, r, "admin.html", &data, msg)
}

// utilsHandler handles admin utils requests.
func utilsHandler(w http.ResponseWriter, r *http.Request, p *gauth.Profile) {
	ctx := r.Context()
	skey, _ := profileData(p)

	var msg string
	devices, err := model.GetDevicesBySite(ctx, settingsStore, skey)
	if err != nil {
		msg = fmt.Sprintf("cannot get devices: %v", err)
	} else {
		sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	}

	sites, err := model.GetAllSites(ctx, settingsStore)
	if err != nil {
		msg = fmt.Sprintf("cannot get sites: %v", err)
	} else {
		sort.Slice(sites, func(i, j int) bool { return sites[i].Name < sites[j].Name })
	}

	data := utilsData{
		commonData: commonData{
			Pages:   pages("utils"),
			Profile: p,
		},
		Devices: devices,
		Sites:   sites,
		Info: map[string]string{
			"Version":     version,
			"Go version":  runtime.Version(),
			"Experiments": os.Getenv("VIDGRIND_EXPERIMENTS"),
		},
	}

	if r.Method == "GET" {
		writeTemplate(w, r, "utils.html", &data, msg)
		return
	}

	err = utilsTaskHandler(w, r, p, &data)
	if err != nil {
		msg = err.Error()
	} else {
		msg = data.Msg
	}
	writeTemplate(w, r, "utils.html", &data, msg)
	return
}

// utilsTaskHandler handles an admin utils task
func utilsTaskHandler(w http.ResponseWriter, r *http.Request, p *gauth.Profile, data *utilsData) error {
	ctx := r.Context()
	skey, _ := profileData(p)

	task := r.FormValue("task")

	// Get device.
	ma := r.FormValue("ma")
	data.Ma = ma
	mac := model.MacEncode(ma)

	dev, err := model.GetDevice(ctx, settingsStore, model.MacEncode(ma))
	if err != nil {
		if err == datastore.ErrNoSuchEntity {
			data.Msg = "device not found"
			return nil
		} else {
			return fmt.Errorf("cannot get device %d: %v", mac, err)
		}
	}

	// Get site.
	var targetSkey int64
	switch task {
	case "find":
		targetSkey = dev.Skey
	case "move":
		sk := r.FormValue("sk")
		targetSkey, err = strconv.ParseInt(sk, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid site key %s", sk)
		}
	default:
		return fmt.Errorf("invalid task: %s", task)
	}

	site, err := model.GetSite(ctx, settingsStore, targetSkey)
	if err != nil {
		if err == datastore.ErrNoSuchEntity {
			data.Msg = "site not found"
			return nil
		} else {
			return fmt.Errorf("cannot get site %d: %v", targetSkey, err)
		}
	}
	data.Sn = site.Name

	if task == "find" {
		return nil
	}

	if targetSkey == skey {
		return fmt.Errorf("target site cannot match source site")
	}

	// Move a device.
	// Check the user is an admin for the target site.
	if !isAdmin(ctx, targetSkey, p.Email) {
		return fmt.Errorf("admin privilege required for target site")
	}

	// Update the device with its new site key.
	dev.Skey = targetSkey
	err = model.PutDevice(ctx, settingsStore, dev)
	if err != nil {
		return fmt.Errorf("cannot update device %d: %v", mac, err)
	}

	// Move the device variables (by re-creating and deleting).
	vars, err := model.GetVariablesBySite(ctx, settingsStore, skey, dev.Hex())
	if err != nil {
		return fmt.Errorf("cannot get variables for device %d: %v", mac, err)
	}
	for _, v := range vars {
		err := model.PutVariable(ctx, settingsStore, targetSkey, v.Name, v.Value)
		if err != nil {
			return fmt.Errorf("cannot put variable %d.%s: %v", targetSkey, v.Name, err)
		}
		err = model.DeleteVariable(ctx, settingsStore, skey, v.Name)
		if err != nil {
			return fmt.Errorf("cannot delete variable %d.%s: %v", skey, v.Name, err)
		}
	}

	data.Msg = fmt.Sprintf("moved device %s (%s) and %d variables to site %s", dev.Name, dev.MAC(), len(vars), site.Name)
	return nil
}

// isSuperAdmin returns true if a user has permission to create new
// sites. Currently, this is limited to users in the domain @localhost
// and @ausocean.org.
func isSuperAdmin(email string) bool {
	at := strings.Index(email, "@")
	if at < 0 {
		return false
	}
	domain := email[at:]
	return domain == "@localhost" || domain == "@ausocean.org"
}

// isAdmin returns true if a user has admin privileges for the given site.
func isAdmin(ctx context.Context, skey int64, email string) bool {
	user, err := model.GetUser(ctx, settingsStore, skey, email)
	if err == nil {
		return user.Perm&model.AdminPermission != 0
	}
	return false
}
