/*
DESCRIPTION
  Datastore entity registrations.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2023 the Australian Ocean Lab (AusOcean).

  This file is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
  or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
  License for more details.

  You should have received a copy of the GNU General Public License in
  gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package model

import (
	"github.com/ausocean/openfish/datastore"
)

// RegisterEntities is a convenience function that registers all of
// the datastore entities in one go.
func RegisterEntities() {
	datastore.RegisterEntity(typeActuator, func() datastore.Entity { return new(Actuator) })
	datastore.RegisterEntity(typeActuatorV2, func() datastore.Entity { return new(ActuatorV2) })
	datastore.RegisterEntity(typeCredential, func() datastore.Entity { return new(Credential) })
	datastore.RegisterEntity(typeCron, func() datastore.Entity { return new(Cron) })
	datastore.RegisterEntity(typeDevice, func() datastore.Entity { return new(Device) })
	datastore.RegisterEntity(typeMedia, func() datastore.Entity { return new(Media) })
	datastore.RegisterEntity(typeMtsMedia, func() datastore.Entity { return new(MtsMedia) })
	datastore.RegisterEntity(typeScalar, func() datastore.Entity { return new(Scalar) })
	datastore.RegisterEntity(typeSensor, func() datastore.Entity { return new(Sensor) })
	datastore.RegisterEntity(typeSensorV2, func() datastore.Entity { return new(SensorV2) })
	datastore.RegisterEntity(typeSite, func() datastore.Entity { return new(Site) })
	datastore.RegisterEntity(typeText, func() datastore.Entity { return new(Text) })
	datastore.RegisterEntity(typeUser, func() datastore.Entity { return new(User) })
	datastore.RegisterEntity(typeVariable, func() datastore.Entity { return new(Variable) })
	datastore.RegisterEntity(typeFeed, func() datastore.Entity { return new(Feed) })
	datastore.RegisterEntity(typeSubscriber, func() datastore.Entity { return new(Subscriber) })
	datastore.RegisterEntity(typeSubscription, func() datastore.Entity { return new(Subscription) })
	datastore.RegisterEntity(TypeSubscriberRegion, func() datastore.Entity { return new(SubscriberRegion) })
}
