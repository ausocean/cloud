package main

import (
	"fmt"
	"time"

	"context"

	"github.com/ausocean/cloud/cmd/oceantv/registry"
)

type event interface{ fmt.Stringer }

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

type startFailedEvent struct{}

var _ = registerEvent(startFailedEvent{})

func (e startFailedEvent) String() string { return "startFailedEvent" }

type criticalFailureEvent struct{}

var _ = registerEvent(criticalFailureEvent{})

func (e criticalFailureEvent) String() string { return "criticalFailureEvent" }

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

type hardwareStartFailedEvent struct{ string }

var _ = registerEvent(hardwareStartFailedEvent{})

func (e hardwareStartFailedEvent) String() string { return "hardwareStartFailedEvent" }
func (e hardwareStartFailedEvent) Error() string  { return e.string }

type hardwareStopFailedEvent struct{ string }

var _ = registerEvent(hardwareStopFailedEvent{})

func (e hardwareStopFailedEvent) String() string { return "hardwareStopFailedEvent" }
func (e hardwareStopFailedEvent) Error() string  { return e.string }

type hardwareStartedEvent struct{}

var _ = registerEvent(hardwareStartedEvent{})

func (e hardwareStartedEvent) String() string { return "hardwareStartedEvent" }

type hardwareStoppedEvent struct{}

var _ = registerEvent(hardwareStoppedEvent{})

func (e hardwareStoppedEvent) String() string { return "hardwareStoppedEvent" }

type controllerFailureEvent struct{ string }

var _ = registerEvent(controllerFailureEvent{})

func (e controllerFailureEvent) String() string { return "controllerFailureEvent" }
func (e controllerFailureEvent) Error() string  { return e.string }

type slateResetRequested struct{}

var _ = registerEvent(slateResetRequested{})

func (e slateResetRequested) String() string { return "slateResetRequested" }

type fixFailureEvent struct{}

var _ = registerEvent(fixFailureEvent{})

func (e fixFailureEvent) String() string { return "fixFailureEvent" }

type invalidConfigurationEvent struct{ desc string }

var _ = registerEvent(invalidConfigurationEvent{})

func (e invalidConfigurationEvent) String() string { return "invalidConfigurationEvent" }
func (e invalidConfigurationEvent) Error() string  { return e.desc }

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

// stringToEvent returns an event given its name.
func stringToEvent(name string) event {
	e, err := registry.Get(name)
	if err != nil {
		panic(fmt.Errorf("could not get event for string %s: %w", name, err))
	}
	return e.(event)
}
