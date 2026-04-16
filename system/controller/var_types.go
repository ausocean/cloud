package controller

import "github.com/ausocean/cloud/model"

// VarTypes returns the variable types for the controller.
func VarTypes() []model.VarType {
	return []model.VarType{
		{Name: "LogLevel", Type: model.VarTypeUint},
		{Name: "Pulses", Type: model.VarTypeUint},
		{Name: "PulseWidth", Type: model.VarTypeUint},
		{Name: "PulseDutyCycle", Type: model.VarTypeUint},
		{Name: "PulseCycle", Type: model.VarTypeUint},
		{Name: "AutoRestart", Type: model.VarTypeUint},
		{Name: "AlarmPeriod", Type: model.VarTypeUint},
		{Name: "AlarmNetwork", Type: model.VarTypeUint},
		{Name: "AlarmVoltage", Type: model.VarTypeUint},
		{Name: "AlarmRecoveryVoltage", Type: model.VarTypeUint},
		{Name: "PeakVoltage", Type: model.VarTypeUint},
		{Name: "HeartbeatPeriod", Type: model.VarTypeUint},
		{Name: "Power0", Type: model.VarTypeBool},
		{Name: "Power1", Type: model.VarTypeBool},
		{Name: "Power2", Type: model.VarTypeBool},
		{Name: "Power3", Type: model.VarTypeBool},
		{Name: "PulseSuppress", Type: model.VarTypeBool},
	}
}
