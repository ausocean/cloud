/*
DESCRIPTION
  VidGrind cron handling.

AUTHORS
  Dan Kortschak <dan@ausocean.org>
  Saxon Nelson-Milton <saxon@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2021-2024 the Australian Ocean Lab (AusOcean)

  This file is part of VidGrind. VidGrind is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  VidGrind is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with NetReceiver in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"bitbucket.org/ausocean/iotsvc/iotds"
)

// proxyScheduler is a cron client that forwards requests to a cron service, such as Ocean Cron.
type proxyScheduler struct {
	url string
}

// Set simply forwards a cron schedule request to a service running a
// cron scheduler. The caller is required to perform relevant cron
// datastore operations _before_ this calling this method, otherwise
// changes will not be visible to the remote service.
// TODO: Sign requests using JWT.
func (ps *proxyScheduler) Set(cron *iotds.Cron) error {
	log.Printf("setting cron: %v", cron.ID)

	// Create a new HTTP client.
	clt := &http.Client{}

	// Create a new request.
	var op string
	if cron.Enabled {
		op = "set"
	} else {
		op = "unset"
	}
	url := ps.url + "/cron/" + op + "/" + strconv.Itoa(int(cron.Skey)) + "/" + cron.ID
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating cron request: %w", err)
	}

	// Send the request.
	resp, err := clt.Do(req)
	if err != nil {
		return fmt.Errorf("error sending cron request: %w", err)
	}

	// Check the response.
	if resp.StatusCode != http.StatusOK {
		return errors.New("cron request failed with status code: " + http.StatusText(resp.StatusCode))
	}

	log.Printf("/cron/%s/%s/%s OK", op, strconv.Itoa(int(cron.Skey)), cron.ID)
	return nil
}
