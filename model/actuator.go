/*
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2020-2023 the Australian Ocean Lab (AusOcean).

  This is free software: you can redistribute it and/or modify it
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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ausocean/openfish/datastore"
)

// Actuator errors.
var (
	ErrWrongNumParts = errors.New("wrong number of values for actuator")
	ErrSKeyParse     = errors.New("cannot parse site key for actuator")
)

const (
	typeActuator   = "Actuator" // Actuator datastore type.
	typeActuatorV2 = "ActuatorV2"
	numParts       = 4 // Number of parts for actuator in TSV.
)

// Pins used by actuators.
const (
	PinPower1 Pin = "D16"
	PinPower2 Pin = "D14"
	PinPower3 Pin = "D15"
)

// Actuator represents an actuator datastore type to link a device output with
// a variable such that that variable value changes the physical device output value.
// NB: This type is deprecated. See ActuatorV2.
type Actuator struct {
	SKey int64  `datastore:"Skey"` // Site key.
	AID  string `datastore:"Aid"`  // Actuator ID.
	Var  string // The variable whose value is applied to the pin.
	Pin  string // The device pin actuated represented as <DeviceID>.<Pin>.
}

// Encode serializes an Actuator into tab-separated values.
func (a *Actuator) Encode() []byte {
	return []byte(fmt.Sprintf("%d\t%s\t%s\t%s", a.SKey, a.AID, a.Var, a.Pin))
}

// Decode deserializes an Actuator from tab-separated values.
func (a *Actuator) Decode(b []byte) error {
	p := strings.Split(string(b), "\t")
	if len(p) != numParts {
		return ErrWrongNumParts
	}
	var err error
	a.SKey, err = strconv.ParseInt(p[0], 10, 64)
	if err != nil {
		return ErrSKeyParse
	}
	a.AID = p[1]
	a.Var = p[2]
	a.Pin = p[3]
	return nil
}

// Copy is not currently implemented.
func (a *Actuator) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

// GetCache returns nil, indicating no caching.
func (a *Actuator) GetCache() datastore.Cache {
	return nil
}

// GetActuatorByPin gets an actuator for device did and with pin.
// NB: Filter fields must be datastore field names!
func GetActuatorByPin(ctx context.Context, store datastore.Store, skey int64, did string, pin string) ([]Actuator, error) {
	q := store.NewQuery(typeActuator, false, "Skey", "Aid", "Pin")
	q.Filter("Skey =", skey)
	q.Filter("Aid =", nil)
	q.Filter("Pin =", did+"."+pin)

	// TODO: do we check for multiple Actuators ? and if so what do we do when there is?
	var acts []Actuator
	_, err := store.GetAll(ctx, q, &acts)
	return acts, err
}

// GetActuatorsBySite returns the actuators associated with a site optionally
// filtered by device. Provide "" for no filtering.
func GetActuatorsBySite(ctx context.Context, store datastore.Store, sKey int64, devID string) ([]Actuator, error) {
	q := store.NewQuery(typeActuator, false, "Skey", "Aid", "Pin")
	q.Filter("Skey =", sKey)
	q.Order("Aid")
	var all, toOut []Actuator
	_, err := store.GetAll(ctx, q, &all)
	if err != nil {
		return nil, err
	}
	if devID == "" {
		return all, nil
	}

	for _, a := range all {
		if strings.Split(a.Pin, ".")[0] == devID {
			toOut = append(toOut, a)
		}
	}
	return toOut, nil
}

// PutActuator puts an actuator in the datastore.
func PutActuator(ctx context.Context, store datastore.Store, act *Actuator) error {
	pin := ""
	_, filestore := store.(*datastore.FileStore)
	if filestore {
		pin = "." + act.Pin // Include pin as part of key name for filestore.
	}
	k := store.NameKey(typeActuator, strconv.FormatInt(act.SKey, 10)+"."+act.AID+pin)
	_, err := store.Put(ctx, k, act)
	return err
}

// DeleteActuator deletes and actuator from the datastore given its actuator ID.
func DeleteActuator(ctx context.Context, store datastore.Store, aid string) error {
	q := store.NewQuery(typeActuator, true, "Aid")
	q.Filter("Aid =", aid)
	keys, err := store.GetAll(ctx, q, nil)
	if err != nil {
		return fmt.Errorf("get all error: %v", err)
	}
	return store.DeleteMulti(ctx, keys)
}

// ActuatorV2 defines a version 2 actuator. An actuator sends the
// value of a variable to a device's output pin. The key is the MAC
// address concatenated with the pin. Version 2 actuators do not have
// a site key, but are linked to a site indirectly via their
// device. The variable name is relative to the device, not a full
// variable name.
type ActuatorV2 struct {
	Name string // Name of actuator (mutable).
	Mac  int64  // MAC address of associated device (immutable).
	Pin  string // Pin of associated device (immutable).
	Var  string // Relative name of device variable (mutable).
}

// Encode encodes an actuator as JSON.
func (a *ActuatorV2) Encode() []byte {
	bytes, _ := json.Marshal(a)
	return bytes
}

// Decode decodes an actuator from JSON.
func (a *ActuatorV2) Decode(b []byte) error {
	return json.Unmarshal(b, a)
}

// Copy copies an actuator to dst, or returns a copy of the acuator when dst is nil.
func (a *ActuatorV2) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var a2 *ActuatorV2
	if dst == nil {
		a2 = new(ActuatorV2)
	} else {
		var ok bool
		a2, ok = dst.(*ActuatorV2)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*a2 = *a
	return a2, nil
}

var actuatorCache datastore.Cache = datastore.NewEntityCache()

// GetCache returns the actuator cache.
func (s *ActuatorV2) GetCache() datastore.Cache {
	return actuatorCache
}

// PutActuatorV2 creates/updates an actuator.
func PutActuatorV2(ctx context.Context, store datastore.Store, act *ActuatorV2) error {
	k := store.NameKey(typeActuatorV2, strconv.FormatInt(act.Mac, 10)+"."+act.Pin)
	_, err := store.Put(ctx, k, act)
	return err
}

// GetActuatorV2 gets an actuator.
func GetActuatorV2(ctx context.Context, store datastore.Store, mac int64, pin string) (*ActuatorV2, error) {
	k := store.NameKey(typeActuatorV2, strconv.FormatInt(mac, 10)+"."+pin)
	act := new(ActuatorV2)
	err := store.Get(ctx, k, act)
	if err != nil {
		return nil, err
	}
	return act, nil
}

// GetActuatorsV2 gets all actuators for a device.
func GetActuatorsV2(ctx context.Context, store datastore.Store, mac int64) ([]ActuatorV2, error) {
	q := store.NewQuery(typeActuatorV2, false, "Mac", "Pin")
	q.Filter("Mac =", mac)
	var acts []ActuatorV2
	_, err := store.GetAll(ctx, q, &acts)
	return acts, err
}

// DeleteActuatorV2 deletes an actuator.
func DeleteActuatorV2(ctx context.Context, store datastore.Store, mac int64, pin string) error {
	k := store.NameKey(typeActuatorV2, strconv.FormatInt(mac, 10)+"."+pin)
	return store.DeleteMulti(ctx, []*datastore.Key{k})
}

// actuatorShim is a thin shim between typed values and an actuator.
func actuatorShim(name, variable string, pin Pin) ActuatorV2 {
	return ActuatorV2{Name: name, Var: variable, Pin: string(pin)}
}
