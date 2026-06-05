package main

import "time"

type hardwarePoweringOff struct {
	stateWithTimeoutFields
}

var _ = register(hardwarePoweringOff{})

func (s hardwarePoweringOff) Name() string { return "hardwarePoweringOff" }

// New implements registry.Newable for creating a fresh value of
// hardwarePoweringOff from an existing value.
func (s hardwarePoweringOff) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwarePoweringOff(ctx) }, args...)
}

func newHardwarePoweringOff(ctx *broadcastContext) *hardwarePoweringOff {
	return &hardwarePoweringOff{stateWithTimeoutFields: newStateWithTimeoutFields(ctx)}
}

func (s *hardwarePoweringOff) enter() {
	s.LastEntered = time.Now()
	s.hardware.stop(s.broadcastContext)
}

func (s *hardwarePoweringOff) exit() {}
