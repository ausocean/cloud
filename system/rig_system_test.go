package system_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/system"
)

const (
	testSiteKey = 1
	strTestMAC  = "00:11:22:33:44:55"
)

func TestNewRigSystem(t *testing.T) {
	tests := []struct {
		name           string
		skey           int64
		mac            string
		ControllerName string
		options        []system.Option
		wantErr        error
		expectedSystem *system.RigSystem
	}{
		{
			name:           "Valid input without options",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController1",
			options:        nil,
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController1",
					Type:    model.DevTypeController,
					Enabled: true,
				},
			},
		},
		{
			name:           "Invalid MAC without options",
			skey:           testSiteKey,
			mac:            "00:11:22:33",
			ControllerName: "TestController2",
			options:        nil,
			wantErr:        model.ErrInvalidMACAddress,
			expectedSystem: nil,
		},
		{
			name:           "With AlarmNetworkVar",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController3",
			options:        []system.Option{system.WithVariables(model.NewAlarmNetworkVar(3))},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController3",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Variables: []*model.Variable{
					{Name: "AlarmNetwork", Value: "3", Skey: testSiteKey},
				},
			},
		},
		{
			name:           "With BatteryVoltage Sensor",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController4",
			options:        []system.Option{system.WithSensors(model.BatteryVoltageSensor(0.0289))},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController4",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Sensors: []*model.SensorV2{
					{Name: "Battery Voltage", Mac: model.MacEncode(strTestMAC)},
				},
			},
		}, {
			name:           "With AirTemperature Sensor",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController5",
			options:        []system.Option{system.WithSensors(model.AirTemperatureSensor())},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController5",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Sensors: []*model.SensorV2{
					{Name: "Air Temperature", Mac: model.MacEncode(strTestMAC)},
				},
			},
		},
		{
			name:           "With Humidity Sensor",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController6",
			options:        []system.Option{system.WithSensors(model.HumiditySensor())},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController6",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Sensors: []*model.SensorV2{
					{Name: "Humidity", Mac: model.MacEncode(strTestMAC)},
				},
			},
		},
		{
			name:           "With WaterTemperature Sensor",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController7",
			options:        []system.Option{system.WithSensors(model.WaterTemperatureSensor())},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController7",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Sensors: []*model.SensorV2{
					{Name: "Water Temperature", Mac: model.MacEncode(strTestMAC)},
				},
			},
		},
		{
			name:           "With AlarmPeriodVar",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController8",
			options:        []system.Option{system.WithVariables(model.NewAlarmPeriodVar(30 * time.Second))},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController8",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Variables: []*model.Variable{
					{Name: "AlarmPeriod", Value: "30", Skey: testSiteKey},
				},
			},
		},
		{
			name:           "With AlarmRecoveryVoltageVar",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController9",
			options:        []system.Option{system.WithVariables(model.NewAlarmRecoveryVoltageVar(12))},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController9",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Variables: []*model.Variable{
					{Name: "AlarmRecoveryVoltage", Value: "12", Skey: testSiteKey},
				},
			},
		},
		{
			name:           "With Power1Var",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController10",
			options:        []system.Option{system.WithVariables(model.NewPower1Var(true))},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController10",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Variables: []*model.Variable{
					{Name: "Power1", Value: "true", Skey: testSiteKey},
				},
			},
		},
		{
			name:           "With Power2Var",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController11",
			options:        []system.Option{system.WithVariables(model.NewPower2Var(false))},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController11",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Variables: []*model.Variable{
					{Name: "Power2", Value: "false", Skey: testSiteKey},
				},
			},
		},
		{
			name:           "With PulseWidthVar",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController12",
			options:        []system.Option{system.WithVariables(model.NewPulseWidthVar(15 * time.Second))},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController12",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Variables: []*model.Variable{
					{Name: "PulseWidth", Value: "15", Skey: testSiteKey},
				},
			},
		},
		{
			name:           "With AutoRestartVar",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController13",
			options:        []system.Option{system.WithVariables(model.NewAutoRestartVar(5 * time.Minute))},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController13",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Variables: []*model.Variable{
					{Name: "AutoRestart", Value: "300", Skey: testSiteKey},
				},
			},
		},
		{
			name:           "With Device1Actuator",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController14",
			options:        []system.Option{system.WithActuators(model.NewDevice1Actuator())},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController14",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Actuators: []*model.ActuatorV2{
					{Name: "Device 1", Var: "Power1", Pin: string(model.PinPower1), Mac: model.MacEncode(strTestMAC)},
				},
			},
		},
		{
			name:           "With Device2Actuator",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController15",
			options:        []system.Option{system.WithActuators(model.NewDevice2Actuator())},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController15",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Actuators: []*model.ActuatorV2{
					{Name: "Device 2", Var: "Power2", Pin: string(model.PinPower2), Mac: model.MacEncode(strTestMAC)},
				},
			},
		},
		{
			name:           "With Device3Actuator",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController16",
			options:        []system.Option{system.WithActuators(model.NewDevice3Actuator())},
			wantErr:        nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController16",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Actuators: []*model.ActuatorV2{
					{Name: "Device 3", Var: "Power3", Pin: string(model.PinPower3), Mac: model.MacEncode(strTestMAC)},
				},
			},
		},
		{
			name:           "With All Sensors and Variables",
			skey:           testSiteKey,
			mac:            strTestMAC,
			ControllerName: "TestController17",
			options: []system.Option{
				system.WithSensors(
					model.BatteryVoltageSensor(0.0289),
					model.AnalogValueSensor(),
					model.AirTemperatureSensor(),
					model.HumiditySensor(),
					model.WaterTemperatureSensor(),
				),
				system.WithVariables(
					model.NewAlarmNetworkVar(3),
					model.NewAlarmPeriodVar(30*time.Second),
					model.NewAlarmRecoveryVoltageVar(12),
					model.NewAlarmVoltageVar(10),
					model.NewAutoRestartVar(5*time.Minute),
					model.NewPower1Var(true),
					model.NewPower2Var(false),
					model.NewPower3Var(true),
					model.NewPulsesVar(100),
					model.NewPulseWidthVar(15*time.Second),
					model.NewPulseCycleVar(60*time.Second),
					model.NewPulseSuppressVar(true),
				),
				system.WithActuators(
					model.NewDevice1Actuator(),
					model.NewDevice2Actuator(),
					model.NewDevice3Actuator(),
				),
			},
			wantErr: nil,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode(strTestMAC),
					Name:    "TestController17",
					Type:    model.DevTypeController,
					Enabled: true,
				},
				Sensors: []*model.SensorV2{
					{Name: "Battery Voltage", Mac: model.MacEncode(strTestMAC)},
					{Name: "Analog Value", Mac: model.MacEncode(strTestMAC)},
					{Name: "Air Temperature", Mac: model.MacEncode(strTestMAC)},
					{Name: "Humidity", Mac: model.MacEncode(strTestMAC)},
					{Name: "Water Temperature", Mac: model.MacEncode(strTestMAC)},
				},
				Variables: []*model.Variable{
					{Name: "AlarmNetwork", Value: "3", Skey: testSiteKey},
					{Name: "AlarmPeriod", Value: "30", Skey: testSiteKey},
					{Name: "AlarmRecoveryVoltage", Value: "12", Skey: testSiteKey},
					{Name: "AlarmVoltage", Value: "10", Skey: testSiteKey},
					{Name: "AutoRestart", Value: "300", Skey: testSiteKey},
					{Name: "Power1", Value: "true", Skey: testSiteKey},
					{Name: "Power2", Value: "false", Skey: testSiteKey},
					{Name: "Power3", Value: "true", Skey: testSiteKey},
					{Name: "Pulses", Value: "100", Skey: testSiteKey},
					{Name: "PulseWidth", Value: "15", Skey: testSiteKey},
					{Name: "PulseCycle", Value: "60", Skey: testSiteKey},
					{Name: "PulseSuppress", Value: "true", Skey: testSiteKey},
				},
				Actuators: []*model.ActuatorV2{
					{Name: "Device 1", Var: "Power1", Pin: string(model.PinPower1), Mac: model.MacEncode(strTestMAC)},
					{Name: "Device 2", Var: "Power2", Pin: string(model.PinPower2), Mac: model.MacEncode(strTestMAC)},
					{Name: "Device 3", Var: "Power3", Pin: string(model.PinPower3), Mac: model.MacEncode(strTestMAC)},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := system.NewRigSystem(tt.skey, tt.mac, tt.ControllerName, tt.options...)
			if err != tt.wantErr {
				t.Fatalf("unexpected error from NewRigSystem, wanted: %v, got: %v", tt.wantErr, err)
			}
			if tt.wantErr != nil {
				// The RigSystem should be nil, and we don't need to compare other fields.
				if got != nil {
					t.Fatalf("got non-nil RigSystem: %v", *got)
				}
				return
			}

			// Check Controller.
			if mismatch := mismatchingFieldValues(got.Controller, tt.expectedSystem.Controller, "Skey", "Mac", "Name", "Type"); len(mismatch) != 0 {
				vwanted := reflect.ValueOf(tt.expectedSystem.Controller)
				vgot := reflect.ValueOf(got.Controller)
				for _, fieldName := range mismatch {
					t.Errorf("mismatch in %s field, wanted: %v, got: %v", fieldName, vwanted.FieldByName(fieldName), vgot.FieldByName(fieldName))
				}
				t.FailNow()
			}

			// Check Variables.
			if len(got.Variables) > 0 {
				t.Log(*got.Variables[0])
			}
			if len(got.Variables) != len(tt.expectedSystem.Variables) {
				t.Fatalf("mismatch in number of variables, wanted: %d, got: %d", len(got.Variables), len(tt.expectedSystem.Variables))
			}
			for i, gotPtr := range got.Variables {
				got := *gotPtr
				wanted := *tt.expectedSystem.Variables[i]
				if mismatch := mismatchingFieldValues(got, wanted, "Skey", "Name", "Value"); len(mismatch) != 0 {
					vwanted := reflect.ValueOf(wanted)
					vgot := reflect.ValueOf(got)
					for _, fieldName := range mismatch {
						t.Errorf("mismatch in %s field, wanted: %v, got: %v", fieldName, vwanted.FieldByName(fieldName), vgot.FieldByName(fieldName))
					}
					t.FailNow()
				}
			}

			// Check Sensors.
			if len(got.Sensors) != len(tt.expectedSystem.Sensors) {
				t.Fatalf("mismatch in number of sensors, wanted: %d, got: %d", len(got.Sensors), len(tt.expectedSystem.Sensors))
			}
			for i, gotPtr := range got.Sensors {
				got := *gotPtr
				wanted := *tt.expectedSystem.Sensors[i]
				if mismatch := mismatchingFieldValues(got, wanted, "Mac", "Name"); len(mismatch) != 0 {
					vwanted := reflect.ValueOf(wanted)
					vgot := reflect.ValueOf(got)
					for _, fieldName := range mismatch {
						t.Errorf("mismatch in %s field, wanted: %v, got: %v", fieldName, vwanted.FieldByName(fieldName), vgot.FieldByName(fieldName))
					}
					t.FailNow()
				}
			}

			// Check Actuators.
			if len(got.Actuators) != len(tt.expectedSystem.Actuators) {
				t.Fatalf("mismatch in number of actuators, wanted: %d, got: %d", len(got.Actuators), len(tt.expectedSystem.Actuators))
			}
			for i, gotPtr := range got.Actuators {
				got := *gotPtr
				wanted := *tt.expectedSystem.Actuators[i]
				if mismatch := mismatchingFieldValues(got, wanted, "Mac", "Name", "Var", "Pin"); len(mismatch) != 0 {
					vwanted := reflect.ValueOf(wanted)
					vgot := reflect.ValueOf(got)
					for _, fieldName := range mismatch {
						t.Errorf("mismatch in %s field, wanted: %v, got: %v", fieldName, vwanted.FieldByName(fieldName), vgot.FieldByName(fieldName))
					}
					t.FailNow()
				}
			}

		})
	}
}

func mismatchingFieldValues(a, b interface{}, fieldNames ...string) []string {
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)

	var badFields []string

	// Get the field by name
	for _, fieldName := range fieldNames {
		fa := va.FieldByName(fieldName)
		fb := vb.FieldByName(fieldName)
		// Compare the fields
		if !reflect.DeepEqual(fa.Interface(), fb.Interface()) {
			badFields = append(badFields, fieldName)
		}
	}

	return badFields
}
