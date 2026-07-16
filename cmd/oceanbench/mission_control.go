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
	"github.com/ausocean/cloud/gauth"
	"github.com/gofiber/fiber/v2"
)

// missionControlHandler handles mission control page requests.
func missionControlHandler(c *fiber.Ctx, profile *gauth.Profile) error {
	data := monitorData{commonData: commonData{Pages: pages("mission control"), Profile: profile}}
	writeTemplate(c, "mission-control.html", &data, "")
	return nil
}
