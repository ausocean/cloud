/*
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

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

package hardware

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
)

type RevidCameraClient struct {
	SetActionVars func(ctx context.Context, sKey int64, acts string, store datastore.Store, log func(string, ...interface{})) error
}

type ControllerError string

const (
	None            ControllerError = ""
	LowVoltageAlarm ControllerError = "LowVoltage"
)

func (e ControllerError) Error() string {
	return string(e)
}

func (e ControllerError) Is(target error) bool {
	if target == nil {
		return false
	}
	if t, ok := target.(ControllerError); ok {
		return e == t
	}
	return false
}

func (c *RevidCameraClient) Voltage(ctx *Context) (float64, error) {
	// Get battery voltage sensor, which we'll use to get scale factor and current voltage value.
	sensor, err := model.GetSensorV2(context.Background(), ctx.store, ctx.Cfg.ControllerMAC, ctx.Cfg.BatteryVoltagePin)
	if err != nil {
		return 0, fmt.Errorf("could not get battery voltage sensor (%s.%s): %w", model.MacDecode(ctx.Cfg.ControllerMAC), ctx.Cfg.BatteryVoltagePin, err)
	}

	// Get current battery voltage.
	voltage, err := model.GetSensorValue(context.Background(), ctx.store, sensor)
	switch {
	case errors.Is(err, datastore.ErrNoSuchEntity):
		// We'll get this if the controller is off from low voltage, so just
		// assume we have alarm voltage.
		alarmVoltage, err := c.AlarmVoltage(ctx)
		if err != nil {
			return 0, fmt.Errorf("could not get alarm voltage: %w", err)
		}
		return alarmVoltage, nil
	case err != nil:
		return 0, fmt.Errorf("could not get current battery voltage: %w", err)
	}

	return voltage, nil
}

func (c *RevidCameraClient) AlarmVoltage(ctx *Context) (float64, error) {
	// Get AlarmVoltage variable; if the voltage is above this we expect the controller to be on.
	// If the voltage is below this, we expect the controller to be off.
	controllerMACHex := (&model.Device{Mac: ctx.Cfg.ControllerMAC}).Hex()
	alarmVoltageVar, err := model.GetVariable(context.Background(), ctx.store, ctx.Cfg.SKey, controllerMACHex+".AlarmVoltage")
	if err != nil {
		return 0, fmt.Errorf("could not get alarm voltage variable: %w", err)
	}
	ctx.Log("got AlarmVoltage for %s: %s", controllerMACHex, alarmVoltageVar.Value)

	uncalibratedAlarmVoltage, err := strconv.Atoi(alarmVoltageVar.Value)
	if err != nil {
		return 0, fmt.Errorf("could not convert uncalibrated alarm voltage from string: %w", err)
	}

	// Get battery voltage sensor, which we'll use to get scale factor and current voltage value.
	batteryVoltagePin := ctx.Cfg.BatteryVoltagePin
	if batteryVoltagePin == "" {
		const defaultBatteryVoltagePin = "A4"
		batteryVoltagePin = defaultBatteryVoltagePin
	}
	sensor, err := model.GetSensorV2(context.Background(), ctx.store, ctx.Cfg.ControllerMAC, batteryVoltagePin)
	if err != nil {
		return 0, fmt.Errorf("could not get battery voltage sensor: %w", err)
	}

	// Transform the alarm voltage to the actual voltage.
	alarmVoltage, err := sensor.Transform(float64(uncalibratedAlarmVoltage))
	if err != nil {
		return 0, fmt.Errorf("could not transform alarm voltage: %w", err)
	}

	return alarmVoltage, nil
}

func (c *RevidCameraClient) IsUp(ctx *Context, mac string) (bool, error) {
	deviceIsUp, err := model.DeviceIsUp(context.Background(), ctx.store, mac)
	if err != nil {
		return false, fmt.Errorf("could not get controller status: %w", err)
	}
	return deviceIsUp, nil
}

func (c *RevidCameraClient) Start(ctx *Context) {
	err := extStart(context.Background(), ctx.store, ctx.Cfg, ctx.Log, c.SetActionVars)
	if err != nil {
		ctx.Log("could not start external hardware: %v", err)
		ctx.Bus.Publish(event.HardwareStartFailed{Err: fmt.Errorf("external hardware start actions failed: %w", err)})
		return
	}
}

func (c *RevidCameraClient) Shutdown(ctx *Context) {
	err := extShutdown(context.Background(), ctx.store, ctx.Cfg, ctx.Log, c.SetActionVars)
	if err != nil {
		ctx.Bus.Publish(event.HardwareShutdownFailed{Err: fmt.Errorf("could not perform shutdown actions: %w", err)})
		return
	}
}

func (c *RevidCameraClient) Stop(ctx *Context) {
	err := extStop(context.Background(), ctx.store, ctx.Cfg, ctx.Log, c.SetActionVars)
	if err != nil {
		ctx.Log("could not stop external hardware: %v", err)
		ctx.Bus.Publish(event.HardwareStopFailed{Err: fmt.Errorf("could not perform stop actions: %w", err)})
		return
	}
}

func (c *RevidCameraClient) PublishEventIfStatus(ctx *Context, e event.Event, status bool, mac int64, store datastore.Store, log func(string, ...interface{}), publish func(e event.Event)) {
	if mac == 0 {
		publish(event.InvalidConfiguration{errors.New("camera mac is empty")})
		return
	}
	log("checking status of device with mac: %d", mac)
	alive, err := model.DeviceIsUp(context.Background(), store, model.MacDecode(mac))
	if err != nil {
		log("could not get device status: %v", err)
		return
	}
	log("status from DeviceIsUp check: %v", alive)
	if alive == status {
		publish(e)
		return
	}
}

func (c *RevidCameraClient) Error(ctx *Context) (error, error) {
	controllerMACHex := (&model.Device{Mac: ctx.Cfg.ControllerMAC}).Hex()
	devErr, err := model.GetVariable(context.Background(), ctx.store, ctx.Cfg.SKey, controllerMACHex+".error")
	if err != nil {
		return nil, fmt.Errorf("could not get controller error variable: %w", err)
	}
	return ControllerError(devErr.Value), nil
}

// extStart uses the OnActions in the provided broadcast config to perform
// external streaming hardware startup. In addition, the RTMP key is obtained
// from the broadcast's associated stream object and used to set the devices
// RTMPKey variable.
func extStart(
	ctx context.Context,
	store datastore.Store,
	cfg *broadcast.Config,
	log func(string, ...interface{}),
	setActionVars func(ctx context.Context, sKey int64, acts string, store datastore.Store, log func(string, ...interface{})) error,
) error {
	if cfg.OnActions == "" {
		return nil
	}

	onActions := cfg.OnActions + "," + cfg.RTMPVar + "=" + broadcast.RTMPDestinationAddress + cfg.RTMPKey
	err := setActionVars(ctx, cfg.SKey, onActions, store, log)
	if err != nil {
		return fmt.Errorf("could not set device variables required to start stream: %w", err)
	}

	return nil
}

// ErrNoShutdownActions represents no shutdown actions being registered for the broadcast.
var ErrNoShutdownActions = errors.New("no shutdown actions provided")

// SkipAction is the placeholder used to represent that the action step should be skipped.
const SkipAction = "skip"

func extShutdown(
	ctx context.Context,
	store datastore.Store,
	cfg *broadcast.Config,
	log func(string, ...interface{}),
	setActionVars func(ctx context.Context, sKey int64, acts string, store datastore.Store, log func(string, ...interface{})) error,
) error {
	if cfg.ShutdownActions == SkipAction {
		return broadcast.WarnSkipShutdown
	}
	if cfg.ShutdownActions == "" {
		return ErrNoShutdownActions
	}

	err := setActionVars(ctx, cfg.SKey, cfg.ShutdownActions, store, log)
	if err != nil {
		return fmt.Errorf("could not set device variables to end stream: %w", err)
	}

	return nil
}

// extStop uses the OffActions in the provided broadcast config to perform
// external streaming hardware shutdown.
func extStop(
	ctx context.Context,
	store datastore.Store,
	cfg *broadcast.Config,
	log func(string, ...interface{}),
	setActionVars func(ctx context.Context, sKey int64, acts string, store datastore.Store, log func(string, ...interface{})) error,
) error {
	if cfg.OffActions == "" {
		return nil
	}

	err := setActionVars(ctx, cfg.SKey, cfg.OffActions, store, log)
	if err != nil {
		return fmt.Errorf("could not set device variables to end stream: %w", err)
	}

	return nil
}
