package main

type hardwareOff struct{}

var _ = register(hardwareOff{})

func (s hardwareOff) Name() string { return "hardwareOff" }

// New implements registry.Newable for creating a fresh value of
// hardwareOff from an existing value.
func (s hardwareOff) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareOff() }, args...)
}

func newHardwareOff() *hardwareOff { return &hardwareOff{} }

func (s *hardwareOff) enter()      {}

func (s *hardwareOff) exit()       {}
