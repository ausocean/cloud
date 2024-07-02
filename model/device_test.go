package model

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ausocean/openfish/datastore"
)

func Test_NewDevice(t *testing.T) {
	ctx := context.Background()

	store, err := datastore.NewStore(ctx, "file", "cloudblue", "")
	if err != nil {
		t.Fatalf("unable to create filestore: %v", err)
	}

	type Want struct {
		dev  Device
		vars map[string]string
		sens []SensorV2
		acts []ActuatorV2
	}

	tests := []struct {
		name string
		mac  string
		opts []DeviceOption
		want Want
	}{{
		name: "no options",
		mac:  "00:11:22:33:44:55",
		want: Want{
			dev: Device{
				Name:          "no options",
				Mac:           MacEncode("00:11:22:33:44:55"),
				ActPeriod:     defaultActPeriod,
				MonitorPeriod: defaultMonPeriod,
				Enabled:       true,
			},
			vars: map[string]string{},
			sens: []SensorV2{},
			acts: []ActuatorV2{},
		},
	},
		{
			name: "controller with defaults",
			mac:  "00:11:22:33:44:66",
			opts: []DeviceOption{WithType(DevTypeController),
				WithInputs(DefaultControllerInputs),
				WithOutputs(DefaultControllerOutputs),
				WithVariables(DefaultControllerVars),
				WithSensors(DefaultControllerSensors),
				WithActuators(DefaultControllerActs),
			},
			want: Want{
				dev: Device{
					Name:          "controller with defaults",
					Mac:           MacEncode("00:11:22:33:44:66"),
					ActPeriod:     defaultActPeriod,
					MonitorPeriod: defaultMonPeriod,
					Enabled:       true,
					Type:          DevTypeController,
					Inputs:        DefaultControllerInputs,
					Outputs:       DefaultControllerOutputs,
				},
				vars: DefaultControllerVars,
				sens: DefaultControllerSensors,
				acts: DefaultControllerActs,
			},
		},
		{
			name: "device with location",
			mac:  "00:11:22:33:44:77",
			opts: []DeviceOption{WithLocation(12.34, 56.78)},
			want: Want{
				dev: Device{
					Name:          "device with location",
					Mac:           MacEncode("00:11:22:33:44:77"),
					ActPeriod:     defaultActPeriod,
					MonitorPeriod: defaultMonPeriod,
					Enabled:       true,
					Latitude:      12.34,
					Longitude:     56.78,
				},
				vars: map[string]string{},
				sens: []SensorV2{},
				acts: []ActuatorV2{},
			},
		},
		{
			name: "controller without defaults",
			mac:  "00:11:22:33:44:88",
			opts: []DeviceOption{WithType(DevTypeController)},
			want: Want{
				dev: Device{
					Name:          "controller without defaults",
					Mac:           MacEncode("00:11:22:33:44:88"),
					ActPeriod:     defaultActPeriod,
					MonitorPeriod: defaultMonPeriod,
					Enabled:       true,
					Type:          DevTypeController,
				},
				vars: map[string]string{},
				sens: []SensorV2{},
				acts: []ActuatorV2{},
			},
		},
		{
			name: "device with custom periods",
			mac:  "00:11:22:33:44:AA",
			opts: []DeviceOption{
				func(ctx context.Context, store datastore.Store, dev *Device) error {
					dev.ActPeriod = 120
					dev.MonitorPeriod = 120
					return nil
				},
			},
			want: Want{
				dev: Device{
					Name:          "device with custom periods",
					Mac:           MacEncode("00:11:22:33:44:AA"),
					ActPeriod:     120,
					MonitorPeriod: 120,
					Enabled:       true,
				},
				vars: map[string]string{},
				sens: []SensorV2{},
				acts: []ActuatorV2{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, err := NewDevice(ctx, store, testSiteKey, tt.name, tt.mac, tt.opts...)
			if err != nil {
				t.Fatalf("test: %s: unable to create new device: %v", tt.name, err)
			}

			if dev.Name != tt.want.dev.Name {
				t.Fatalf("test: %s: got: %s, wanted: %s", tt.name, dev.Name, tt.want.dev.Name)
			}

			if dev.Mac != tt.want.dev.Mac {
				t.Fatalf("test: %s: got: %d, wanted: %d", tt.name, dev.Mac, tt.want.dev.Mac)
			}

			if dev.Type != tt.want.dev.Type {
				t.Fatalf("test: %s: got: %s, wanted: %s", tt.name, dev.Type, tt.want.dev.Type)
			}

			if dev.Latitude != tt.want.dev.Latitude {
				t.Fatalf("test: %s: got: %f, wanted: %f", tt.name, dev.Latitude, tt.want.dev.Latitude)
			}

			if dev.Longitude != tt.want.dev.Longitude {
				t.Fatalf("test: %s: got: %f, wanted: %f", tt.name, dev.Longitude, tt.want.dev.Longitude)
			}

			if dev.Inputs != tt.want.dev.Inputs {
				t.Fatalf("test: %s: got: %s, wanted: %s", tt.name, dev.Inputs, tt.want.dev.Inputs)
			}

			if dev.Outputs != tt.want.dev.Outputs {
				t.Fatalf("test: %s: got: %s, wanted: %s", tt.name, dev.Outputs, tt.want.dev.Outputs)
			}

			// Check variables
			for varName, varVal := range tt.want.vars {
				val, err := GetVariable(ctx, store, testSiteKey, tt.mac+"."+varName)
				if err != nil {
					t.Fatalf("test: %s: unable to get variable: %v", tt.name, err)
				}
				if val.Value != varVal {
					t.Fatalf("test: %s: variable %s: got: %s, wanted: %s", tt.name, varName, val.Value, varVal)
				}
				varName = fmt.Sprintf("%s.%s", strings.ReplaceAll(tt.mac, ":", ""), varName)
				if val.Name != varName {
					t.Fatalf("test: %s: variable %s: got: %s, wanted: %s", tt.name, varName, val.Name, varName)
				}
			}

			// Check sensors
			for _, wantSensor := range tt.want.sens {
				gotSensor, err := GetSensorV2(ctx, store, MacEncode(tt.mac), wantSensor.Pin)
				if err != nil {
					t.Fatalf("test: %s: unable to get sensor: %v", tt.name, err)
				}
				if gotSensor.Args != wantSensor.Args {
					t.Fatalf("test: %s: sensor %s: got: %+v, wanted: %+v", tt.name, wantSensor.Name, gotSensor, wantSensor)
				}
				if gotSensor.Format != wantSensor.Format {
					t.Fatalf("test: %s: sensor %s: got: %+v, wanted: %+v", tt.name, wantSensor.Name, gotSensor, wantSensor)
				}
				if gotSensor.Func != wantSensor.Func {
					t.Fatalf("test: %s: sensor %s: got: %+v, wanted: %+v", tt.name, wantSensor.Name, gotSensor, wantSensor)
				}
				if gotSensor.Name != wantSensor.Name {
					t.Fatalf("test: %s: sensor %s: got: %+v, wanted: %+v", tt.name, wantSensor.Name, gotSensor, wantSensor)
				}
				if gotSensor.Quantity != wantSensor.Quantity {
					t.Fatalf("test: %s: sensor %s: got: %+v, wanted: %+v", tt.name, wantSensor.Name, gotSensor, wantSensor)
				}
				if gotSensor.Units != wantSensor.Units {
					t.Fatalf("test: %s: sensor %s: got: %+v, wanted: %+v", tt.name, wantSensor.Name, gotSensor, wantSensor)
				}
			}

			// Check actuators
			for _, wantActuator := range tt.want.acts {
				gotActuator, err := GetActuatorV2(ctx, store, MacEncode(tt.mac), wantActuator.Pin)
				if err != nil {
					t.Fatalf("test: %s: unable to get actuator: %v", tt.name, err)
				}
				if gotActuator.Var != wantActuator.Var {
					t.Fatalf("test: %s: actuator %s: got: %+v, wanted: %+v", tt.name, wantActuator.Name, gotActuator, wantActuator)
				}
				if gotActuator.Pin != wantActuator.Pin {
					t.Fatalf("test: %s: actuator %s: got: %+v, wanted: %+v", tt.name, wantActuator.Name, gotActuator, wantActuator)
				}
				if gotActuator.Name != wantActuator.Name {
					t.Fatalf("test: %s: actuator %s: got: %+v, wanted: %+v", tt.name, wantActuator.Name, gotActuator, wantActuator)
				}
			}
		})
	}
}
