package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

type hardwareFailure struct {
	*broadcastContext `json:"-"`
	err               error
}

var _ = register(hardwareFailure{})

func newHardwareFailure(ctx *broadcastContext, err error) *hardwareFailure {
	return &hardwareFailure{ctx, err}
}

func (s hardwareFailure) Name() string { return "hardwareFailure" }

func (s hardwareFailure) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{ Err string }{Err: s.err.Error()})
}

func (s *hardwareFailure) UnmarshalJSON(data []byte) error {
	aux := struct{ Err string }{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	s.err = errors.New(aux.Err)
	return nil
}

// New implements registry.Newable for creating a fresh value of
// hardwareFailure from an existing value.
func (s hardwareFailure) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareFailure(ctx, nil) }, args...)
}

func (s *hardwareFailure) enter() {
	notifyMsg := "entering hardware failure state"
	notifyKind := broadcastGeneric
	if s.err != nil {
		if errEvent, ok := s.err.(errorEvent); ok {
			notifyKind = errEvent.Kind()
		}
		notifyMsg = fmt.Sprintf("entering hardware failure state due to: %v", s.err)
	}
	s.logAndNotify(notifyKind, "%s", notifyMsg)
}

func (s *hardwareFailure) exit() {}
