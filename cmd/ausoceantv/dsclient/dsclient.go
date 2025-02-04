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

// Package dsclient initializes the datastore and makes it available to other packages through the use of Get().
package dsclient

import (
	"context"

	"github.com/ausocean/openfish/datastore"
	"github.com/gofiber/fiber/v2/log"
)

var store datastore.Store

// Get returns the datastore global variable.
func Get() datastore.Store {
	return store
}

// Init initializes the datastore global variable and datastore client.
func Init(standalone bool, filestorePath string) error {
	ctx := context.Background()
	var err error
	if standalone {
		log.Info("running in standalone mode")
		store, err = datastore.NewStore(ctx, "file", "vidgrind", filestorePath)
	} else {
		log.Info("running in App Engine mode")
		store, err = datastore.NewStore(ctx, "cloud", "vidgrind", "")
	}
	return err
}
