package main

type hardwareOn struct{}

var _ = register(hardwareOn{})

func (s hardwareOn) Name() string { return "hardwareOn" }

// New implements registry.Newable for creating a fresh value of
// hardwareOn from an existing value.
func (s hardwareOn) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareOn() }, args...)
}

func newHardwareOn() *hardwareOn { return &hardwareOn{} }
func (s *hardwareOn) enter()     {}
func (s *hardwareOn) exit()      {}
