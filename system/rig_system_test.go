package system_test

import (
	"reflect"
	"testing"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/system"
)

const testSiteKey = 1

func TestNewRigSystem(t *testing.T) {
	tests := []struct {
		name           string
		skey           int64
		mac            string
		ControllerName string
		options        []system.Option
		wantErr        bool
		expectedSystem *system.RigSystem
	}{
		{
			name:           "Valid input without options",
			skey:           testSiteKey,
			mac:            "00:11:22:33:44:55",
			ControllerName: "TestController",
			options:        nil,
			wantErr:        false,
			expectedSystem: &system.RigSystem{
				Controller: model.Device{
					Skey:    testSiteKey,
					Mac:     model.MacEncode("00:11:22:33:44:55"),
					Name:    "TestController",
					Type:    model.DevTypeController,
					Enabled: true,
				},
			},
		},
		{
			name:           "Invalid MAC without options",
			skey:           testSiteKey,
			mac:            "00:11:22:33:44",
			ControllerName: "TestController",
			options:        nil,
			wantErr:        true,
			expectedSystem: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := system.NewRigSystem(tt.skey, tt.mac, tt.ControllerName, tt.options...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRigSystem() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Controller.Skey != tt.skey {
					t.Errorf("Expected Skey %d, got %d", tt.skey, got.Controller.Skey)
				}
				if got.Controller.Mac != model.MacEncode(tt.mac) {
					t.Errorf("Expected Mac %d, got %d", model.MacEncode(tt.mac), got.Controller.Mac)
				}
				if got.Controller.Name != tt.ControllerName {
					t.Errorf("Expected Name %s, got %s", tt.ControllerName, got.Controller.Name)
				}
				for i, variable := range got.Variables {
					if !reflect.DeepEqual(*variable, *(tt.expectedSystem.Variables[i])) {
						t.Errorf("Expected Variables %v, got %v", *tt.expectedSystem.Variables[i], *variable)
					}
				}
				if !reflect.DeepEqual(got.Sensors, tt.expectedSystem.Sensors) {
					t.Errorf("Expected Sensors %v, got %v", tt.expectedSystem.Sensors, got.Sensors)
				}
				if !reflect.DeepEqual(got.Actuators, tt.expectedSystem.Actuators) {
					t.Errorf("Expected Actuators %v, got %v", tt.expectedSystem.Actuators, got.Actuators)
				}
				if !reflect.DeepEqual(got.Peripherals, tt.expectedSystem.Peripherals) {
					t.Errorf("Expected Peripherals %v, got %v", tt.expectedSystem.Peripherals, got.Peripherals)
				}
			}
		})
	}
}
