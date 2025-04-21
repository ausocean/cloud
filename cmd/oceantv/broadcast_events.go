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

func marshalEvent(event event) []byte {
	alias := struct {
		Name string
		Err  string
	}{Name: event.String()}

	if errEvent, ok := event.(error); ok {
		// Trim event name from error string, this format is expected.
		after, found := strings.CutPrefix(errEvent.Error(), "("+event.String()+") ")
		if !found {
			panic(fmt.Sprintf("event Error() does not have correct format, want: (<event name>) <error message>, got: %s", errEvent.Error()))
		}

		alias.Err = after
	}

	eventData, err := json.Marshal(alias)
	if err != nil {
		panic(fmt.Sprintf("could not marshal event: %v", err))
	}

	return eventData
}

func unmarshalEvent(eventData string) event {
	alias := struct {
		Name string
		Err  error
	}{}

	err := json.Unmarshal([]byte(eventData), &alias)
	if err != nil {
		panic(fmt.Sprintf("could not unmarshal event: %v", err))
	}

	event := stringToEvent(alias.Name, alias.Err)

	if _, ok := event.(error); alias.Err != nil && !ok {
		panic(fmt.Sprintf("have error data for event: %s, but event is not an error: %T", alias.Name, event))
	}

	if _, ok := event.(registry.Newable); alias.Err != nil && !ok {
		panic("have error data for event but can't New with data because not Newable")
	}

	return event
}
