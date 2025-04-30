package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"context"

	"github.com/ausocean/cloud/cmd/oceantv/registry"
	"github.com/ausocean/cloud/notify"
)

type event interface{ fmt.Stringer }

type errorEvent interface {
	event
	error
	registry.Newable
	Kind() notify.Kind
	Unwrap() error
}

// registerEvent registers an event in the registry.
// It returns a struct{} to allow us to register the event in a var declaration in
// the global scope.
func registerEvent(e event) struct{} {
	err := registry.Register(e)
	if err != nil {
		panic(err)
	}
	return struct{}{}
}

type timeEvent struct{ time.Time }

var _ = registerEvent(timeEvent{})

func (e timeEvent) String() string { return "timeEvent" }

type finishEvent struct{}

var _ = registerEvent(finishEvent{})

func (e finishEvent) String() string { return "finishEvent" }

type finishedEvent struct{}

var _ = registerEvent(finishedEvent{})

func (e finishedEvent) String() string { return "finishedEvent" }

type startEvent struct{}

var _ = registerEvent(startEvent{})

func (e startEvent) String() string { return "startEvent" }

type startedEvent struct{}

var _ = registerEvent(startedEvent{})

func (e startedEvent) String() string { return "startedEvent" }

type startFailedEvent struct{ error }

var _ = registerEvent(startFailedEvent{})

func (e startFailedEvent) String() string { return "startFailedEvent" }
func (e startFailedEvent) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}

func (e startFailedEvent) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return startFailedEvent{err}, nil
}

// Kind implements the errorEvent interface.
func (e startFailedEvent) Kind() notify.Kind {
	if errEvent, ok := e.error.(errorEvent); ok {
		return errEvent.Kind()
	}

	if unwrapped := unwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcastGeneric
}

func (e startFailedEvent) Unwrap() error { return e.error }

func (e startFailedEvent) Is(target error) bool {
	if _, ok := target.(startFailedEvent); ok {
		return true
	}
	return errors.Is(e.error, target)
}

// criticalFailureEvent is really a non recoverable start failure event.
type criticalFailureEvent struct{ error }

var _ = registerEvent(criticalFailureEvent{})

func (e criticalFailureEvent) String() string { return "criticalFailureEvent" }
func (e criticalFailureEvent) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}
func (e criticalFailureEvent) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return criticalFailureEvent{err}, nil
}

// Kind implements the errorEvent interface.
func (e criticalFailureEvent) Kind() notify.Kind {
	if errEvent, ok := e.error.(errorEvent); ok {
		return errEvent.Kind()
	}

	if unwrapped := unwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcastGeneric
}

func (e criticalFailureEvent) Unwrap() error { return e.error }

func (e criticalFailureEvent) Is(target error) bool {
	if _, ok := target.(criticalFailureEvent); ok {
		return true
	}
	return errors.Is(e.error, target)
}

type healthCheckDueEvent struct{}

var _ = registerEvent(healthCheckDueEvent{})

func (e healthCheckDueEvent) String() string { return "healthCheckDueEvent" }

type statusCheckDueEvent struct{}

var _ = registerEvent(statusCheckDueEvent{})

func (e statusCheckDueEvent) String() string { return "statusCheckDueEvent" }

type chatMessageDueEvent struct{}

var _ = registerEvent(chatMessageDueEvent{})

func (e chatMessageDueEvent) String() string { return "chatMessageDueEvent" }

type badHealthEvent struct{}

var _ = registerEvent(badHealthEvent{})

func (e badHealthEvent) String() string { return "badHealthEvent" }

type goodHealthEvent struct{}

var _ = registerEvent(goodHealthEvent{})

func (e goodHealthEvent) String() string { return "goodHealthEvent" }

type hardwareStartRequestEvent struct{}

var _ = registerEvent(hardwareStartRequestEvent{})

func (e hardwareStartRequestEvent) String() string { return "hardwareStartRequestEvent" }

type hardwareStopRequestEvent struct{}

var _ = registerEvent(hardwareStopRequestEvent{})

func (e hardwareStopRequestEvent) String() string { return "hardwareStopRequestEvent" }

type hardwareResetRequestEvent struct{}

var _ = registerEvent(hardwareResetRequestEvent{})

func (e hardwareResetRequestEvent) String() string { return "hardwareResetRequestEvent" }

type hardwareStartFailedEvent struct{ error }

var _ = registerEvent(hardwareStartFailedEvent{})

func (e hardwareStartFailedEvent) String() string { return "hardwareStartFailedEvent" }
func (e hardwareStartFailedEvent) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}
func (e hardwareStartFailedEvent) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return hardwareStartFailedEvent{err}, nil
}
func (e hardwareStartFailedEvent) Kind() notify.Kind {
	if errEvent, ok := e.error.(errorEvent); ok {
		return errEvent.Kind()
	}

	if unwrapped := unwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcastHardware
}

func (e hardwareStartFailedEvent) Unwrap() error { return e.error }

func (e hardwareStartFailedEvent) Is(target error) bool {
	if _, ok := target.(hardwareStartFailedEvent); ok {
		return true
	}
	return errors.Is(e.error, target)
}

type hardwareStopFailedEvent struct{ error }

var _ = registerEvent(hardwareStopFailedEvent{})

func (e hardwareStopFailedEvent) String() string { return "hardwareStopFailedEvent" }
func (e hardwareStopFailedEvent) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}
func (e hardwareStopFailedEvent) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return hardwareStopFailedEvent{err}, nil
}
func (e hardwareStopFailedEvent) Kind() notify.Kind {
	if errEvent, ok := e.error.(errorEvent); ok {
		return errEvent.Kind()
	}

	if unwrapped := unwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcastHardware
}

func (e hardwareStopFailedEvent) Unwrap() error { return e.error }

func (e hardwareStopFailedEvent) Is(target error) bool {
	if _, ok := target.(hardwareStartFailedEvent); ok {
		return true
	}
	return errors.Is(e.error, target)
}

type hardwareStartedEvent struct{}

var _ = registerEvent(hardwareStartedEvent{})

func (e hardwareStartedEvent) String() string { return "hardwareStartedEvent" }

type hardwareStoppedEvent struct{}

var _ = registerEvent(hardwareStoppedEvent{})

func (e hardwareStoppedEvent) String() string { return "hardwareStoppedEvent" }

type controllerFailureEvent struct{ error }

var _ = registerEvent(controllerFailureEvent{})

func (e controllerFailureEvent) String() string { return "controllerFailureEvent" }
func (e controllerFailureEvent) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}
func (e controllerFailureEvent) New(args ...any) (any, error) {
	return controllerFailureEvent{args[0].(error)}, nil
}
func (e controllerFailureEvent) Kind() notify.Kind {
	if errEvent, ok := e.error.(errorEvent); ok {
		return errEvent.Kind()
	}

	if unwrapped := unwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcastHardware
}

func (e controllerFailureEvent) Unwrap() error { return e.error }

func (e controllerFailureEvent) Is(target error) bool {
	if _, ok := target.(controllerFailureEvent); ok {
		return true
	}
	return errors.Is(e.error, target)
}

type slateResetRequested struct{}

var _ = registerEvent(slateResetRequested{})

func (e slateResetRequested) String() string { return "slateResetRequested" }

type fixFailureEvent struct{}

var _ = registerEvent(fixFailureEvent{})

func (e fixFailureEvent) String() string { return "fixFailureEvent" }

type invalidConfigurationEvent struct{ error }

var _ = registerEvent(invalidConfigurationEvent{})

func (e invalidConfigurationEvent) String() string { return "invalidConfigurationEvent" }
func (e invalidConfigurationEvent) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}
func (e invalidConfigurationEvent) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return invalidConfigurationEvent{err}, nil
}
func (e invalidConfigurationEvent) Kind() notify.Kind {
	if errEvent, ok := e.error.(errorEvent); ok {
		return errEvent.Kind()
	}

	if unwrapped := unwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcastConfiguration
}

func (e invalidConfigurationEvent) Unwrap() error { return e.error }

func (e invalidConfigurationEvent) Is(target error) bool {
	if _, ok := target.(invalidConfigurationEvent); ok {
		return true
	}
	return errors.Is(e.error, target)
}

type lowVoltageEvent struct{}

var _ = registerEvent(lowVoltageEvent{})

func (e lowVoltageEvent) String() string { return "lowVoltageEvent" }

type voltageRecoveredEvent struct{}

var _ = registerEvent(voltageRecoveredEvent{})

func (e voltageRecoveredEvent) String() string { return "voltageRecoveredEvent" }

type handler func(event) error

type eventBus interface {
	subscribe(handler handler)
	publish(event event)
}

// basicEventBus is a simple event bus that stores events when the context is
// cancelled.
type basicEventBus struct {
	ctx        context.Context
	handlers   []handler
	storeEvent func(event event)
	log        func(string, ...interface{})
}

// newBasicEventBus creates a new basicEventBus.
// The context must be cancellable.
// The storeEventAfterCancel function is called on publish when the context
// is cancelled.
func newBasicEventBus(ctx context.Context, storeEventAfterCancel func(event event), log func(string, ...interface{})) *basicEventBus {
	return &basicEventBus{storeEvent: storeEventAfterCancel, ctx: ctx, log: log}
}

func (bus *basicEventBus) subscribe(handler handler) { bus.handlers = append(bus.handlers, handler) }

func (bus *basicEventBus) publish(event event) {
	bus.log("publishing event: %s", event.String())
	doneChan := bus.ctx.Done()
	if doneChan == nil {
		panic("context must be cancellable")
	}

	select {
	case <-doneChan:
		bus.storeEvent(event)
		return
	default:
	}

	for _, handler := range bus.handlers {
		err := handler(event)
		if err != nil {
			bus.log("error handling event: %s: %v", event.String(), err)
		}
	}
}

// unwrapErrEvent recursively unwraps the error and returns the last errorEvent
// found in the chain.
func unwrapErrEvent(err error, last errorEvent) errorEvent {
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		if unwrappedErrEvent, ok := unwrapped.(errorEvent); ok {
			last = unwrappedErrEvent
		}
		return unwrapErrEvent(unwrapped, last)
	}
	return last
}

// stringToEvent returns an event given its name.
func stringToEvent(name string, args ...interface{}) event {
	e, err := registry.Get(name, args...)
	if err != nil {
		panic(fmt.Errorf("could not get event for string %s: %w", name, err))
	}
	return e.(event)
}

type serializedEvent struct {
	Type    string           `json:"type"`
	Message string           `json:"message,omitempty"`
	Cause   *serializedEvent `json:"cause,omitempty"`
}

func encodeSerializedEvent(v any) *serializedEvent {
	if v == nil {
		return nil
	}

	switch e := v.(type) {
	case errorEvent:
		return &serializedEvent{
			Type:    e.String(),
			Message: e.Error(), // Optional, useful for logging/debugging
			Cause:   encodeSerializedEvent(e.Unwrap())}

	case event:
		// Non-error event — no cause
		return &serializedEvent{
			Type: e.String(),
		}

	case error:
		// Standard error — possibly wrapped
		return &serializedEvent{
			Type:    "generic",
			Message: e.Error(),
			Cause:   encodeSerializedEvent(errors.Unwrap(e)), // recurse
		}

	default:
		panic(fmt.Errorf("unsupported event type: %T", v))
	}
}

func unmarshalEvent(data []byte) event {
	ser := &serializedEvent{}
	if err := json.Unmarshal(data, ser); err != nil {
		panic(fmt.Errorf("could not unmarshal event: %w", err))
	}
	return decodeSerializedEvent(ser).(event)
}

// This is to help us deal with the fact that after marshalling and unmarshalling
// errors.Is does not work as we would like. This is because for a basic error from
// errors.New() all that is checked through errors.Is is that the pointer is the same.
// So after unmarshalling we get a new pointer and errors.Is will not work.
// This is a workaround to make it work for the generic case. We just consider the
// error to be a match if the string is the same. This is not ideal but it is better than
// nothing.
type lenientComparisonError struct{ string }

func (e lenientComparisonError) Error() string        { return e.string }
func (e lenientComparisonError) Is(target error) bool { return e.Error() == target.Error() }

func decodeSerializedEvent(ev *serializedEvent) any {
	if ev == nil {
		return nil
	}

	if ev.Type == "generic" {
		if ev.Cause != nil {
			decoded := decodeSerializedEvent(ev.Cause).(error)
			return fmt.Errorf("%s%w", strings.ReplaceAll(ev.Message, decoded.Error(), ""), decoded)
		}
		return lenientComparisonError{ev.Message}
	}

	if ev.Cause != nil {
		return stringToEvent(ev.Type, decodeSerializedEvent(ev.Cause))
	}
	return stringToEvent(ev.Type)
}

func marshalEvent(e event) []byte {
	ser := encodeSerializedEvent(e)
	if ser == nil {
		panic("trying to marshal nil event")
	}
	data, err := json.Marshal(ser)
	if err != nil {
		panic(fmt.Errorf("could not marshal event: %w", err))
	}
	return data
}
