/*
AUTHORS
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

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

package notification

import "github.com/ausocean/cloud/notify"

const (
	KindGeneric       notify.Kind = "broadcast-generic"       // Problems where cause is unknown or un-categorized.
	KindForwarder     notify.Kind = "broadcast-forwarder"     // Problems related to our forwarding service i.e. can't stream slate.
	KindHardware      notify.Kind = "broadcast-hardware"      // Problems related to streaming hardware i.e. controllers and cameras.
	KindNetwork       notify.Kind = "broadcast-network"       // Problems related to bad bandwidth, generally indicated by bad health events.
	KindSoftware      notify.Kind = "broadcast-software"      // Problems related to the functioning of our broadcast software.
	KindConfiguration notify.Kind = "broadcast-configuration" // Problems related to the configuration of the broadcast.
	KindService       notify.Kind = "broadcast-service"       // Problems related to the broadcast service e.g. YouTube API issues.
)
