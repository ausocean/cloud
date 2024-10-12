package main

import (
	"fmt"
	"time"

	"context"
)

type event interface{ fmt.Stringer }

type timeEvent struct{ time.Time }

func (e timeEvent) String() string { return "timeEvent" }

type finishEvent struct{}

func (e finishEvent) String() string { return "finishEvent" }

type startEvent struct{}

func (e startEvent) String() string { return "startEvent" }

type startedEvent struct{}

func (e startedEvent) String() string { return "startedEvent" }

type startFailedEvent struct{}

func (e startFailedEvent) String() string { return "startFailedEvent" }

type healthCheckDueEvent struct{}

func (e healthCheckDueEvent) String() string { return "healthCheckDueEvent" }

type statusCheckDueEvent struct{}

func (e statusCheckDueEvent) String() string { return "statusCheckDueEvent" }

type chatMessageDueEvent struct{}

func (e chatMessageDueEvent) String() string { return "chatMessageDueEvent" }

type badHealthEvent struct{}

func (e badHealthEvent) String() string { return "badHealthEvent" }

type goodHealthEvent struct{}

func (e goodHealthEvent) String() string { return "goodHealthEvent" }

type hardwareStartRequestEvent struct{}

func (e hardwareStartRequestEvent) String() string { return "hardwareStartRequestEvent" }

type hardwareStopRequestEvent struct{}

func (e hardwareStopRequestEvent) String() string { return "hardwareStopRequestEvent" }

type hardwareResetRequestEvent struct{}

func (e hardwareResetRequestEvent) String() string { return "hardwareResetRequestEvent" }

type hardwareStartFailedEvent struct{}

func (e hardwareStartFailedEvent) String() string { return "hardwareStartFailedEvent" }

type hardwareStopFailedEvent struct{}

func (e hardwareStopFailedEvent) String() string { return "hardwareStopFailedEvent" }

type hardwareStartedEvent struct{}

func (e hardwareStartedEvent) String() string { return "hardwareStartedEvent" }

type hardwareStoppedEvent struct{}

func (e hardwareStoppedEvent) String() string { return "hardwareStoppedEvent" }

type controllerFailureEvent struct{}

func (e controllerFailureEvent) String() string { return "controllerFailureEvent" }

type slateResetRequested struct{}

func (e slateResetRequested) String() string { return "slateResetRequested" }

type fixFailureEvent struct{}

func (e fixFailureEvent) String() string { return "fixFailureEvent" }

type invalidConfigurationEvent struct{ desc string }

func (e invalidConfigurationEvent) String() string { return "invalidConfigurationEvent" }
func (e invalidConfigurationEvent) Error() string  { return e.desc }

type lowVoltageEvent struct{}

func (e lowVoltageEvent) String() string { return "lowVoltageEvent" }

type voltageRecoveredEvent struct{}

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
func stringToEvent(name string) (event, error) {
	eventMap := map[string]event{
		"timeEvent":                 timeEvent{},
		"finishEvent":               finishEvent{},
		"startEvent":                startEvent{},
		"startedEvent":              startedEvent{},
		"startFailedEvent":          startFailedEvent{},
		"healthCheckDueEvent":       healthCheckDueEvent{},
		"statusCheckDueEvent":       statusCheckDueEvent{},
		"chatMessageDueEvent":       chatMessageDueEvent{},
		"badHealthEvent":            badHealthEvent{},
		"goodHealthEvent":           goodHealthEvent{},
		"hardwareStartRequestEvent": hardwareStartRequestEvent{},
		"hardwareStopRequestEvent":  hardwareStopRequestEvent{},
		"hardwareResetRequestEvent": hardwareResetRequestEvent{},
		"hardwareStartFailedEvent":  hardwareStartFailedEvent{},
		"hardwareStopFailedEvent":   hardwareStopFailedEvent{},
		"hardwareStartedEvent":      hardwareStartedEvent{},
		"hardwareStoppedEvent":      hardwareStoppedEvent{},
		"controllerFailureEvent":    controllerFailureEvent{},
		"slateResetRequested":       slateResetRequested{},
		"fixFailureEvent":           fixFailureEvent{},
		"invalidConfigurationEvent": invalidConfigurationEvent{},
		"lowVoltageEvent":           lowVoltageEvent{},
		"voltageRecoveredEvent":     voltageRecoveredEvent{},
	}

	event, ok := eventMap[name]
	if !ok {
		panic(fmt.Sprintf("unknown event: %s", name))
	}
	return event, nil
}
