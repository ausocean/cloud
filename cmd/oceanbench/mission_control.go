/*
DESCRIPTION
  Ocean Bench site mission control handling.

AUTHORS
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2022-2026 the Australian Ocean Lab (AusOcean)

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
	"errors"
	"log"

	"github.com/ausocean/cloud/gauth"
	"github.com/gofiber/fiber/v2"
)

// missionControlHandler handles mission control page requests.
func missionControlHandler(c *fiber.Ctx) error {
	logRequest(c)

	p, err := getProfile(c)
	switch {
	case err != nil && !errors.Is(err, gauth.TokenNotFound):
		log.Printf("authentication error: %v", err)
		fallthrough
	case err != nil:
		return c.Redirect("/", fiber.StatusUnauthorized)
	}

	if !isSuperAdmin(p.Email) {
		return c.Redirect("/", fiber.StatusUnauthorized)
	}

	data := monitorData{commonData: commonData{Pages: pages(c, "mission control"), Profile: p}}
	writeTemplate(c, "mission-control.html", &data, "")
	return nil
}
