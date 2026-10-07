/*
DESCRIPTION
  broadcast_manager.go provides the BroadcastManager interface and
  implementations i.e. OceanBroadcastManager.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2023 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package main

import "github.com/ausocean/cloud/cmd/oceantv/manager"

// newOceanBroadcastManager attaches cron lifecycle management to OceanTV saves.
func newOceanBroadcastManager(hst Hst, cfg *Cfg, db Store, log func(string, ...interface{}), opts ...manager.Option) *manager.OceanBroadcast {
	options := make([]manager.Option, 0, len(opts)+1)
	if broadcastCrons != nil {
		options = append(options, manager.WithCronManager(broadcastCrons))
	}
	options = append(options, opts...)
	return manager.NewOceanBroadcast(hst, cfg, db, log, setVar, broadcastByName, options...)
}
