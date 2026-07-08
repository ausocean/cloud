package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/registry"
	"github.com/ausocean/cloud/notify"
)

// Event represents an event in an event driven state machine
type Event interface{ fmt.Stringer }

// Error is an Event which represents that an error has occurred.
type Error interface {
	Event
	error
	registry.Newable
	Kind() notify.Kind
	Unwrap() error
}

// registerEvent registers an event in the registry.
// It returns a struct{} to allow us to register the event in a var declaration in
// the global scope.
func registerEvent(e Event) struct{} {
	err := registry.Register(e)
	if err != nil {
		panic(err)
	}
	return struct{}{}
}

// Time represents time passing to "tick" the state machine periodically.
type Time struct{ time.Time }

var _ = registerEvent(Time{})

// String implements the Event interface.
func (e Time) String() string { return "timeEvent" }

// Finish represents that a broadcast is due to finish.
type Finish struct{}

var _ = registerEvent(Finish{})

// String implements the Event interface.
func (e Finish) String() string { return "finishEvent" }

// Finished represents that a broadcast has finished.
type Finished struct{}

var _ = registerEvent(Finished{})

// String implements the Event interface.
func (e Finished) String() string { return "finishedEvent" }

// Start represents that a broadcat is due to start.
type Start struct{}

var _ = registerEvent(Start{})

// String implements the Event interface.
func (e Start) String() string { return "startEvent" }

// Started represents that a broadcast has started.
type Started struct{}

var _ = registerEvent(Started{})

// String implements the Event interface.
func (e Started) String() string { return "startedEvent" }

// StartFailed represents that a broadcast has failed to start.
type StartFailed struct{ error }

var _ = registerEvent(StartFailed{})

// String implements the Event interface.
func (e StartFailed) String() string { return "startFailedEvent" }
func (e StartFailed) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}

// New implements the Error interface.
func (e StartFailed) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return StartFailed{err}, nil
}

// Kind implements the Error interface.
func (e StartFailed) Kind() notify.Kind {
	if errEvent, ok := e.error.(Error); ok {
		return errEvent.Kind()
	}

	if unwrapped := UnwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcast.KindGeneric
}

// Unwrap implements the Error interface.
func (e StartFailed) Unwrap() error { return e.error }

// Is implements the Error interface.
func (e StartFailed) Is(target error) bool {
	if _, ok := target.(StartFailed); ok {
		return true
	}
	return errors.Is(e.error, target)
}

// CriticalFailure is really a non recoverable start failure event.
type CriticalFailure struct{ error }

var _ = registerEvent(CriticalFailure{})

// String implements the Event interface.
func (e CriticalFailure) String() string { return "criticalFailureEvent" }
func (e CriticalFailure) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}

// New implements the Error interface.
func (e CriticalFailure) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return CriticalFailure{err}, nil
}

// Kind implements the Error interface.
func (e CriticalFailure) Kind() notify.Kind {
	if errEvent, ok := e.error.(Error); ok {
		return errEvent.Kind()
	}

	if unwrapped := UnwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcast.KindGeneric
}

// Unwrap implements the Error interface.
func (e CriticalFailure) Unwrap() error { return e.error }

// Is implements the Error interface.
func (e CriticalFailure) Is(target error) bool {
	if _, ok := target.(CriticalFailure); ok {
		return true
	}
	return errors.Is(e.error, target)
}

// HealthCheckDue represents that a broadcast is due for a health check.
type HealthCheckDue struct{}

var _ = registerEvent(HealthCheckDue{})

// String implements the Event interface.
func (e HealthCheckDue) String() string { return "healthCheckDueEvent" }

// StatusCheckDue represents that a broadcast is due for a status check.
type StatusCheckDue struct{}

var _ = registerEvent(StatusCheckDue{})

// String implements the Event interface.
func (e StatusCheckDue) String() string { return "statusCheckDueEvent" }

// ChatMessageDue represents that a broadcast is due to send a chat message.
type ChatMessageDue struct{}

var _ = registerEvent(ChatMessageDue{})

// String implements the Event interface.
func (e ChatMessageDue) String() string { return "chatMessageDueEvent" }

// BadHealth represents a broadcast has received a bad health indication.
type BadHealth struct{}

var _ = registerEvent(BadHealth{})

// String implements the Event interface.
func (e BadHealth) String() string { return "badHealthEvent" }

// GoodHealth represents that a broadcast has received a good health indication.
type GoodHealth struct{}

var _ = registerEvent(GoodHealth{})

// String implements the Event interface.
func (e GoodHealth) String() string { return "goodHealthEvent" }

// HardwareStartRequest represents that the hardware has been requested to start.
type HardwareStartRequest struct{}

var _ = registerEvent(HardwareStartRequest{})

// String implements the Event interface.
func (e HardwareStartRequest) String() string { return "hardwareStartRequestEvent" }

// HardwareStopRequest represents that the hardware has been requested to stop.
type HardwareStopRequest struct{}

var _ = registerEvent(HardwareStopRequest{})

// String implements the Event interface.
func (e HardwareStopRequest) String() string { return "hardwareStopRequestEvent" }

// HardwareResetRequest represents that the hardware has been requested to reset.
type HardwareResetRequest struct{}

var _ = registerEvent(HardwareResetRequest{})

// String implements the Event interface.
func (e HardwareResetRequest) String() string { return "hardwareResetRequestEvent" }

// HardwareStartFailed represents that the hardware failed to start.
type HardwareStartFailed struct{ error }

var _ = registerEvent(HardwareStartFailed{})

// String implements the Event interface.
func (e HardwareStartFailed) String() string { return "hardwareStartFailedEvent" }
func (e HardwareStartFailed) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}

// New implements the Error interface.
func (e HardwareStartFailed) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return HardwareStartFailed{err}, nil
}

// Kind implements the Error interface.
func (e HardwareStartFailed) Kind() notify.Kind {
	if errEvent, ok := e.error.(Error); ok {
		return errEvent.Kind()
	}

	if unwrapped := UnwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcast.KindHardware
}

// Unwrap implements the Error interface.
func (e HardwareStartFailed) Unwrap() error { return e.error }

// Is implements the Error interface.
func (e HardwareStartFailed) Is(target error) bool {
	if _, ok := target.(HardwareStartFailed); ok {
		return true
	}
	return errors.Is(e.error, target)
}

// HardwareStopFailed represents that the hardware has failed to stop.
type HardwareStopFailed struct{ error }

var _ = registerEvent(HardwareStopFailed{})

// String implements the Event interface.
func (e HardwareStopFailed) String() string { return "hardwareStopFailedEvent" }
func (e HardwareStopFailed) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}

// New implements the Error interface.
func (e HardwareStopFailed) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return HardwareStopFailed{err}, nil
}

// Kind implements the Error interface.
func (e HardwareStopFailed) Kind() notify.Kind {
	if errEvent, ok := e.error.(Error); ok {
		return errEvent.Kind()
	}

	if unwrapped := UnwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcast.KindHardware
}

// Unwrap implements the Error interface.
func (e HardwareStopFailed) Unwrap() error { return e.error }

// Is implements the Error interface.
func (e HardwareStopFailed) Is(target error) bool {
	if _, ok := target.(HardwareStartFailed); ok {
		return true
	}
	return errors.Is(e.error, target)
}

// HardwareStarted represents that the hardware has started.
type HardwareStarted struct{}

var _ = registerEvent(HardwareStarted{})

// String implements the Event interface.
func (e HardwareStarted) String() string { return "hardwareStartedEvent" }

// HardwareStopped represents that the hardware has stopped.
type HardwareStopped struct{}

var _ = registerEvent(HardwareStopped{})

// String implements the Event interface.
func (e HardwareStopped) String() string { return "hardwareStoppedEvent" }

// ControllerFailure represents that the controller has experienced a failure.
type ControllerFailure struct{ error }

var _ = registerEvent(ControllerFailure{})

// String implements the Event interface.
func (e ControllerFailure) String() string { return "controllerFailureEvent" }
func (e ControllerFailure) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}

// New implements the Error interface.
func (e ControllerFailure) New(args ...any) (any, error) {
	return ControllerFailure{args[0].(error)}, nil
}

// Kind implements the Error interface.
func (e ControllerFailure) Kind() notify.Kind {
	if errEvent, ok := e.error.(Error); ok {
		return errEvent.Kind()
	}

	if unwrapped := UnwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcast.KindHardware
}

// Unwrap implements the Error interface.
func (e ControllerFailure) Unwrap() error { return e.error }

// Is implements the Error interface.
func (e ControllerFailure) Is(target error) bool {
	if _, ok := target.(ControllerFailure); ok {
		return true
	}
	return errors.Is(e.error, target)
}

// SlateResetRequested represents that there has been a request to reset the slate.
type SlateResetRequested struct{}

var _ = registerEvent(SlateResetRequested{})

// String implements the Event interface.
func (e SlateResetRequested) String() string { return "slateResetRequested" }

// FixFailure represents that an attempted state fix has failed.
type FixFailure struct{ error }

var _ = registerEvent(FixFailure{})

// String implements the Event interface.
func (e FixFailure) String() string { return "fixFailureEvent" }
func (e FixFailure) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}

// New implements the Error interface.
func (e FixFailure) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return FixFailure{err}, nil
}

// Kind implements the Error interface.
func (e FixFailure) Kind() notify.Kind {
	if errEvent, ok := e.error.(Error); ok {
		return errEvent.Kind()
	}

	if unwrapped := UnwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcast.KindGeneric
}

// Unwrap implements the Error interface.
func (e FixFailure) Unwrap() error { return e.error }

// Is implements the Error interface.
func (e FixFailure) Is(target error) bool {
	if _, ok := target.(FixFailure); ok {
		return true
	}
	return errors.Is(e.error, target)
}

// InvalidConfiguration represents that a broadcast has an invalid configuration.
type InvalidConfiguration struct{ error }

var _ = registerEvent(InvalidConfiguration{})

// String implements the Event interface.
func (e InvalidConfiguration) String() string { return "invalidConfigurationEvent" }
func (e InvalidConfiguration) Error() string {
	if e.error == nil {
		return "(" + e.String() + ") <nil>"
	}
	return "(" + e.String() + ") " + e.error.Error()
}

// New implements the Error interface.
func (e InvalidConfiguration) New(args ...any) (any, error) {
	var err error = nil
	if len(args) != 0 {
		err = args[0].(error)
	}
	return InvalidConfiguration{err}, nil
}

// Kind implements the Error interface.
func (e InvalidConfiguration) Kind() notify.Kind {
	if errEvent, ok := e.error.(Error); ok {
		return errEvent.Kind()
	}

	if unwrapped := UnwrapErrEvent(e.error, nil); unwrapped != nil {
		return unwrapped.Kind()
	}

	return broadcast.KindConfiguration
}

// Unwrap implements the Error interface.
func (e InvalidConfiguration) Unwrap() error { return e.error }

// Is implements the Error interface.
func (e InvalidConfiguration) Is(target error) bool {
	if _, ok := target.(InvalidConfiguration); ok {
		return true
	}
	return errors.Is(e.error, target)
}

// LowVoltage represents that the hardware is reporting a voltage
// below the configured threshold.
type LowVoltage struct{}

var _ = registerEvent(LowVoltage{})

// String implements the Event interface.
func (e LowVoltage) String() string { return "lowVoltageEvent" }

// VoltageRecovered represents that the voltage has been reported above
// the configured voltage recovery threshold.
type VoltageRecovered struct{}

var _ = registerEvent(VoltageRecovered{})

// String implements the Event interface.
func (e VoltageRecovered) String() string { return "voltageRecoveredEvent" }

// Handler is a function to handle an event.
type Handler func(Event) error

// EventBus is an interface to be used to handle events fired and handled by
// a state machine.
type EventBus interface {
	subscribe(handler Handler)
	publish(event Event)
}

// BasicEventBus is a simple event bus that stores events when the context is
// cancelled.
type BasicEventBus struct {
	ctx        context.Context
	handlers   []Handler
	storeEvent func(event Event)
	log        func(string, ...interface{})
}

// NewBasicEventBus creates a new basicEventBus.
// The context must be cancellable.
// The storeAfterCancel function is called on publish when the context
// is cancelled.
func NewBasicEventBus(ctx context.Context, storeEventAfterCancel func(event Event), log func(string, ...interface{})) *BasicEventBus {
	return &BasicEventBus{storeEvent: storeEventAfterCancel, ctx: ctx, log: log}
}

func (bus *BasicEventBus) subscribe(handler Handler) { bus.handlers = append(bus.handlers, handler) }

func (bus *BasicEventBus) publish(event Event) {
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

// UnwrapErrEvent recursively unwraps the error and returns the last errorEvent
// found in the chain.
func UnwrapErrEvent(err error, last Error) Error {
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		if unwrappedErrEvent, ok := unwrapped.(Error); ok {
			last = unwrappedErrEvent
		}
		return UnwrapErrEvent(unwrapped, last)
	}
	return last
}

// StringToEvent returns an event given its name.
func StringToEvent(name string, args ...interface{}) Event {
	e, err := registry.Get(name, args...)
	if err != nil {
		panic(fmt.Errorf("could not get event for string %s: %w", name, err))
	}
	return e.(Event)
}

type Serialized struct {
	Type    string      `json:"type"`
	Message string      `json:"message,omitempty"`
	Cause   *Serialized `json:"cause,omitempty"`
}

func EncodeSerializedEvent(v any) *Serialized {
	if v == nil {
		return nil
	}

	switch e := v.(type) {
	case Error:
		return &Serialized{
			Type:    e.String(),
			Message: e.Error(), // Optional, useful for logging/debugging
			Cause:   EncodeSerializedEvent(e.Unwrap())}

	case Event:
		// Non-error event — no cause
		return &Serialized{
			Type: e.String(),
		}

	case error:
		// Standard error — possibly wrapped
		return &Serialized{
			Type:    "generic",
			Message: e.Error(),
			Cause:   EncodeSerializedEvent(errors.Unwrap(e)), // recurse
		}

	default:
		panic(fmt.Errorf("unsupported event type: %T", v))
	}
}

func UnmarshalEvent(data []byte) Event {
	ser := &Serialized{}
	if err := json.Unmarshal(data, ser); err != nil {
		panic(fmt.Errorf("could not unmarshal event: %w", err))
	}
	return DecodeSerializedEvent(ser).(Event)
}

// This is to help us deal with the fact that after marshalling and unmarshalling
// errors.Is does not work as we would like. This is because for a basic error from
// errors.New() all that is checked through errors.Is is that the pointer is the same.
// So after unmarshalling we get a new pointer and errors.Is will not work.
// This is a workaround to make it work for the generic case. We just consider the
// error to be a match if the string is the same. This is not ideal but it is better than
// nothing.
type LenientComparisonError struct{ string }

func (e LenientComparisonError) Error() string { return e.string }

// Is implements the Error interface.
func (e LenientComparisonError) Is(target error) bool { return e.Error() == target.Error() }

func DecodeSerializedEvent(ev *Serialized) any {
	if ev == nil {
		return nil
	}

	if ev.Type == "generic" {
		if ev.Cause != nil {
			decoded := DecodeSerializedEvent(ev.Cause).(error)
			return fmt.Errorf("%s%w", strings.ReplaceAll(ev.Message, decoded.Error(), ""), decoded)
		}
		return LenientComparisonError{ev.Message}
	}

	if ev.Cause != nil {
		return StringToEvent(ev.Type, DecodeSerializedEvent(ev.Cause))
	}
	return StringToEvent(ev.Type)
}

func MarshalEvent(e Event) []byte {
	ser := EncodeSerializedEvent(e)
	if ser == nil {
		panic("trying to marshal nil event")
	}
	data, err := json.Marshal(ser)
	if err != nil {
		panic(fmt.Errorf("could not marshal event: %w", err))
	}
	return data
}
