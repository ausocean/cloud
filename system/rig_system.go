package system

import (
	"context"
	"fmt"
	"strings"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

// RigSystem represents a controller device which has associated
// variables, sensors, acuators, and peripherals.
type RigSystem struct {
	Controller  model.Device
	Variables   []*model.Variable
	Sensors     []*model.SensorV2
	Actuators   []*model.ActuatorV2
	Peripherals []*model.Device
}

// Option represents functional options that can be passed to NewRigSystem.
type Option func(*RigSystem) error

// WithVariables is a functional option that adds the passed variables to the RigSystem.
func WithVariables(variables ...*model.Variable) func(*RigSystem) error {
	return func(sys *RigSystem) error {
		sys.Variables = append(sys.Variables, variables...)
		return nil
	}
}

// WithSensors is a functional option that adds the passed sensors to the RigSystem.
func WithSensors(sensors ...*model.SensorV2) func(*RigSystem) error {
	return func(sys *RigSystem) error {
		sys.Sensors = append(sys.Sensors, sensors...)
		return nil
	}
}

// WithActuators is a functional option that adds the passed actuators to the RigSystem.
func WithActuators(actuators ...*model.ActuatorV2) func(*RigSystem) error {
	return func(sys *RigSystem) error {
		sys.Actuators = append(sys.Actuators, actuators...)
		return nil
	}
}

// WithPeripherals is a functional option that adds the passed peripherals to the RigSystem.
func WithPeripherals(peripherals ...*model.Device) func(*RigSystem) error {
	return func(sys *RigSystem) error {
		sys.Peripherals = append(sys.Peripherals, peripherals...)
		return nil
	}
}

// NewRigSystem returns a new RigSystem with the given options. It is the callers
// responsibility to put the components into the datastore.
//
// MAC and name refer to the MAC Address and name of the Controller which will be the heart of
// the RigSystem.
func NewRigSystem(skey int64, MAC, name string, options ...Option) (*RigSystem, error) {
	if model.MacEncode(MAC) == 0 {
		return nil, model.ErrInvalidMACAddress
	}

	sys := &RigSystem{
		Controller: model.Device{
			Skey:    skey,
			Mac:     model.MacEncode(MAC),
			Name:    name,
			Type:    model.DevTypeController,
			Enabled: true,
		},
	}

	// Apply functional options.
	for i, opt := range options {
		err := opt(sys)
		if err != nil {
			return nil, fmt.Errorf("failed to apply option[%d]: %w", i, err)
		}
	}

	// Append site/device information to actuators, sensors, and variables.
	for _, variable := range sys.Variables {
		variable.Skey = skey
		variable.Scope = strings.ReplaceAll(sys.Controller.MAC(), ":", "")
	}
	for _, sensor := range sys.Sensors {
		sensor.Mac = sys.Controller.Mac
	}
	for _, actuator := range sys.Actuators {
		actuator.Mac = sys.Controller.Mac
	}

	return sys, nil
}

// Put puts a RigSystem and all of its components into the datastore.
func PutRigSystem(ctx context.Context, store datastore.Store, system *RigSystem) error {
	// Put the Controller.
	err := model.PutDevice(ctx, store, &system.Controller)
	if err != nil {
		return fmt.Errorf("unable to put system controller: %w", err)
	}

	// Put all variables.
	for _, v := range system.Variables {
		err = model.PutVariable(ctx, store, v.Skey, v.Name, v.Value)
		if err != nil {
			return fmt.Errorf("unable to put variable with name: %s, err: %w", v.Name, err)
		}
	}

	// Put all sensors.
	for _, s := range system.Sensors {
		err = model.PutSensorV2(ctx, store, s)
		if err != nil {
			return fmt.Errorf("unable to put sensor with name: %s, err: %w", s.Name, err)
		}
	}

	// Put all actuators.
	for _, a := range system.Actuators {
		err = model.PutActuatorV2(ctx, store, a)
		if err != nil {
			return fmt.Errorf("unable to put actuator with name: %s, err: %w", a.Name, err)
		}
	}

	// Put all Peripherals
	for _, p := range system.Peripherals {
		err = model.PutDevice(ctx, store, p)
		if err != nil {
			return fmt.Errorf("unable to put peripheral with name: %s, err: %w", p.Name, err)
		}
	}

	return nil
}
