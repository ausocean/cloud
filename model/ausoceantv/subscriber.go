/*
AUTHORS
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean).

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
  or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
  License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package model

import "time"

// Subscriber is an entity in the datastore representing a user who subscribes to AusOcean.TV.
type Subscriber struct {
	ID              string    // AusOcean assigned Subscriber ID.
	AccountID       string    // Google Account ID. NB: May not be necessary.
	Email           string    // Subscriber's email address.
	GivenName       string    // Subscriber's given name.
	FamilyName      string    // Subscriber's family name.
	Area            []string  // Subscriber’s area(s) of interest.
	DemographicInfo string    // Optional demographic info about the subscriber, e.g., their postcode.
	PaymentInfo     string    // Info required to use a payments platform.
	Created         time.Time // Time the subscriber entity was created.
}
