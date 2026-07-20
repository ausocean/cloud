/*
DESCRIPTION
  device-log.go provides a handler for logs.

AUTHOR
  Deborah Baker <deborah@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

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
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/gofiber/fiber/v2"
)

// setLogHandler creates a new log for the given device MAC and sitekey. The request parameters are:
//
//	sk: site key
//	ma: device MAC address (encoded as int64)
//	ld: log data.
func setLogHandler(c *fiber.Ctx) error {
	ctx := context.Background()

	// Validate the user is logged in.
	profile, err := getProfile(c)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		c.Status(http.StatusUnauthorized)
		return nil
	}

	// Parse the fields to be put into the log.
	skStr := c.FormValue("sk")
	maStr := c.FormValue("ma")
	ld := c.FormValue("lg")

	// Convert the site key and device MAC to int64.
	sk, err := strconv.ParseInt(skStr, 10, 64)
	if err != nil {
		log.Printf("failed to parse site key: %v", err)
		c.Status(fiber.StatusBadRequest)
		return err
	}
	ma, err := strconv.ParseInt(maStr, 10, 64)
	if err != nil {
		log.Printf("failed to parse device MAC: %v", err)
		c.Status(fiber.StatusBadRequest)
		return err
	}

	// Check the user has at least admin permissions for the site they are trying to create a log for.
	user, err := model.GetUser(ctx, settingsStore, sk, profile.Email)
	if errors.Is(err, datastore.ErrNoSuchEntity) || (err == nil && user.Perm&model.AdminPermission == 0) {
		log.Println("user does not have admin permissions")
		c.Status(fiber.StatusUnauthorized)
		return err
	} else if err != nil {
		log.Printf("failed to get permission for user: %v", err)
		c.Status(fiber.StatusInternalServerError)
		return err
	}

	// Put the new Log into the datastore.
	err = model.PutLog(ctx, settingsStore, &model.Log{Skey: sk, DeviceMAC: ma, Note: ld})

	// Return any errors from putting the log.
	if err != nil {
		log.Printf("failed to put Log: %v", err)
		c.Status(fiber.StatusInternalServerError)
		return err
	}

	// Redirect to the log page.
	return c.Redirect("/logs", fiber.StatusSeeOther)
}

// logPageHandler handles requests for the log page.
func logPageHandler(c *fiber.Ctx) error {
	logRequest(c)
	profile, err := getProfile(c)
	switch {
	case err != nil && !errors.Is(err, gauth.TokenNotFound):
		log.Printf("authentication error: %v", err)
		fallthrough
	case err != nil:
		return c.Redirect("/", fiber.StatusUnauthorized)
	}

	skey, err := getCurrentSkey(c, profile)
	if err != nil {
		log.Printf("unable to get current skey, redirecting: %v", err)
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	// Ensure user has write permission for the site.
	ctx := c.UserContext()
	user, err := model.GetUser(ctx, settingsStore, skey, profile.Email)
	if errors.Is(err, datastore.ErrNoSuchEntity) || user.Perm&model.WritePermission == 0 {
		log.Println("user does not have write permissions")
		return c.Redirect("/", fiber.StatusUnauthorized)
	} else if err != nil {
		log.Printf("failed to get permission for user: %v", err)
		return c.Redirect("/", fiber.StatusInternalServerError)
	}

	expectedPath := fmt.Sprintf("/%d/logs", skey)
	if c.Path() != expectedPath {
		// Redirect all invalid URLs to the base URL.
		return c.Redirect(expectedPath, fiber.StatusFound)
	}

	data := adminData{
		commonData: commonData{
			Pages:   pages(c, "logs"),
			Profile: profile,
		},
		Skey: skey,
	}

	return writeTemplate(c, "log.html", &data, "")
}
