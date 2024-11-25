/*
AUTHORS
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This file is part of AusOcean TV. AusOcean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  AusOcean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with AusOcean TV in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

// Package api provides API handlers for the AusOcean TV API.
package api

import (
	"github.com/ausocean/cloud/cmd/ausoceantv/dsclient"
	"github.com/gofiber/fiber/v2"
)

func CreateFeed(ctx *fiber.Ctx) error {

	store := dsclient.Get()

	store.Put()

	return ctx.JSON(Feed{ID: &id})
}
