package main

import "time"

type hardwareShuttingDown struct {
	stateWithTimeoutFields
}

var _ = register(hardwareShuttingDown{})

func (s hardwareShuttingDown) Name() string { return "hardwareShuttingDown" }

// New implements registry.Newable for creating a fresh value of
// hardwareShuttingDown from an existing value.
func (s hardwareShuttingDown) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareShuttingDown(ctx) }, args...)
}

func newHardwareShuttingDown(ctx *broadcastContext) *hardwareShuttingDown {
	return &hardwareShuttingDown{stateWithTimeoutFields: newStateWithTimeoutFields(ctx)}
}

func (s *hardwareShuttingDown) enter() {
	s.LastEntered = time.Now()
	s.hardware.shutdown(s.broadcastContext)
}

func (s *hardwareShuttingDown) exit() {}
