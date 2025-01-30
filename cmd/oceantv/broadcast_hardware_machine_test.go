package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/ausocean/cloud/notify"
)

func TestGetHardwareStateStorage(t *testing.T) {
	tests := []struct {
		name         string
		initialState state
	}{
		{"test hardware off", newHardwareOff()},
		{"test hardware on", newHardwareOn()},
		{"test hardware starting", newHardwareStarting(&broadcastContext{camera: &dummyHardwareManager{}, logOutput: t.Log, notifier: newMockNotifier()})},
		{"test hardware stopping", newHardwareStopping(minimalMockBroadcastContext(t))},
		{"test hardware restarting", newHardwareRestarting(minimalMockBroadcastContext(t))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateStr := hardwareStateToString(tt.initialState)
			gotState := getHardwareState(&broadcastContext{cfg: &BroadcastConfig{HardwareState: stateStr}, logOutput: t.Log, notifier: newMockNotifier()})
			if reflect.TypeOf(gotState) != reflect.TypeOf(tt.initialState) {
				t.Errorf("expected state %v, got %v", tt.initialState, gotState)
			}
		})
	}
}

func TestHandleHardwareStoppedEvent(t *testing.T) {
	bCtx := standardMockBroadcastContext(t, false)

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
	}{
		{
			desc:          "hardwareStopping transitions to hardwareOff",
			initialState:  newHardwareStopping(bCtx),
			expectedState: newHardwareOff(),
		},
		{
			desc:          "hardwareStarting transitions to hardwareOff",
			initialState:  newHardwareStarting(bCtx),
			expectedState: newHardwareOff(),
		},
		{
			desc:          "hardwareOn transitions to hardwareOff",
			initialState:  newHardwareOn(),
			expectedState: newHardwareOff(),
		},
		{
			desc:          "hardwareRestarting remains hardwareRestarting",
			initialState:  newHardwareRestarting(bCtx),
			expectedState: newHardwareRestarting(bCtx), // Assuming this state remains unchanged.
		},
		// Add other states and their transitions as needed.
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &BroadcastConfig{}
			bCtx.man = newDummyManager(t, bCtx.cfg)
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)
			bus.publish(hardwareStoppedEvent{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling stopped event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleHardwareStopFailedEvent(t *testing.T) {
	bCtx := standardMockBroadcastContext(t, false)

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
	}{
		{
			desc:          "hardwareStopping transitions to hardwareFailure",
			initialState:  newHardwareStopping(bCtx),
			expectedState: &hardwareFailure{},
		},
		{
			desc:          "hardwareRestarting transitions to hardwareFailure",
			initialState:  newHardwareRestarting(bCtx),
			expectedState: &hardwareFailure{},
		},
		{
			desc:          "hardwareStarting stays as hardwareStarting",
			initialState:  newHardwareStarting(bCtx),
			expectedState: &hardwareStarting{}, // Assuming this state remains unchanged.
		},
		// Add other states and their transitions as needed.
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &BroadcastConfig{}
			bCtx.man = newDummyManager(t, bCtx.cfg)
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)
			bus.publish(hardwareStopFailedEvent{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling stop failed event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleHardwareStartFailedEvent(t *testing.T) {
	bCtx := standardMockBroadcastContext(t, false)

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
	}{
		{
			desc:          "hardwareStarting transitions to hardwareFailure",
			initialState:  newHardwareStarting(bCtx),
			expectedState: &hardwareFailure{},
		},
		{
			desc:          "hardwareRestarting transitions to hardwareFailure",
			initialState:  newHardwareRestarting(bCtx),
			expectedState: &hardwareFailure{},
		},
		// For other states that don't match the above, you might want a generic test to ensure
		// they don't transition. Here's an example for hardwareOn:
		{
			desc:          "hardwareOn remains hardwareOn",
			initialState:  newHardwareOn(),
			expectedState: newHardwareOn(), // This assumes it remains unchanged.
		},
		// Add other states and their transitions as needed.
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &BroadcastConfig{}
			bCtx.man = newDummyManager(t, bCtx.cfg)
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)
			bus.publish(hardwareStartFailedEvent{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling start failed event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleHardwareStartedEvent(t *testing.T) {
	bCtx := standardMockBroadcastContext(t, false)

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
	}{
		{
			desc:          "hardwareStarting transitions to hardwareOn",
			initialState:  newHardwareStarting(bCtx),
			expectedState: newHardwareOn(),
		},
		{
			desc:          "hardwareRestarting transitions to hardwareOn",
			initialState:  newHardwareRestarting(bCtx),
			expectedState: newHardwareOn(),
		},
		// For other states that don't match the above, you might want a generic test to ensure
		// they don't transition. Here's an example for hardwareOff:
		{
			desc:          "hardwareOff remains hardwareOff",
			initialState:  newHardwareOff(),
			expectedState: newHardwareOff(), // This assumes it remains unchanged.
		},
		// Add other states and their transitions as needed.
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &BroadcastConfig{}
			bCtx.man = newDummyManager(t, bCtx.cfg)
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)
			bus.publish(hardwareStartedEvent{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling started event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

func TestHandleHardwareResetRequestEvent(t *testing.T) {
	bCtx := standardMockBroadcastContext(t, true)

	tests := []struct {
		desc          string
		initialState  state
		expectedState state
	}{
		{
			desc:          "hardwareOn transitions to hardwareRestarting",
			initialState:  newHardwareOn(),
			expectedState: newHardwareRestarting(bCtx),
		},
		{
			desc:          "hardwareOff transitions to hardwareStarting",
			initialState:  newHardwareOff(),
			expectedState: newHardwareStarting(bCtx),
		},
		{
			desc:          "hardwareRestarting remains hardwareRestarting",
			initialState:  newHardwareRestarting(bCtx),
			expectedState: newHardwareRestarting(bCtx), // This assumes it remains unchanged.
		},
		{
			desc:          "hardwareStarting remains hardwareStarting",
			initialState:  newHardwareStarting(bCtx),
			expectedState: newHardwareStarting(bCtx), // This assumes it remains unchanged.
		},
		{
			desc:          "hardwareStopping remains hardwareStopping",
			initialState:  newHardwareStopping(bCtx),
			expectedState: newHardwareStopping(bCtx), // This assumes it remains unchanged.
		},
		// Add other states and their transitions as needed.
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, _ := context.WithCancel(context.Background())
			bus := newBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &BroadcastConfig{}
			bCtx.man = newDummyManager(t, bCtx.cfg)
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.subscribe(sm.handleEvent)
			bus.publish(hardwareResetRequestEvent{})

			// Assuming you have a stateToString function
			if stateToString(sm.currentState) != stateToString(tt.expectedState) {
				t.Errorf("unexpected state after handling reset request event: got %v, want %v",
					stateToString(sm.currentState), stateToString(tt.expectedState))
			}
		})
	}
}

type hardwareSystem struct {
	ctx *broadcastContext
	hsm *hardwareStateMachine
	log func(string, ...interface{})
}

var hardwareSys hardwareSystem

func (hs *hardwareSystem) tick() error {
	for _, event := range hs.ctx.cfg.Events {
		e := stringToEvent(event)
		hs.log("publishing stored event: %s", e.String())
		hs.ctx.bus.publish(e)
	}

	// Remove stored events we just published from the config.
	err := hs.ctx.man.Save(nil, func(_cfg *BroadcastConfig) { _cfg.Events = nil })
	if err != nil {
		return fmt.Errorf("could not clear config events: %w", err)
	}

	hs.ctx.bus.publish(timeEvent{time.Now()})
	return nil
}

type hardwareSystemOption func(*hardwareSystem) error

func (h hardwareSystem) withBroadcastManager(bm BroadcastManager) hardwareSystemOption {
	return func(bs *hardwareSystem) error {
		bs.ctx.man = bm
		return nil
	}
}

func (h hardwareSystem) withBroadcastService(bs BroadcastService) hardwareSystemOption {
	return func(b *hardwareSystem) error {
		b.ctx.svc = bs
		return nil
	}
}

func (h hardwareSystem) withForwardingService(fs ForwardingService) hardwareSystemOption {
	return func(bs *hardwareSystem) error {
		bs.ctx.fwd = fs
		return nil
	}
}

func (h hardwareSystem) withHardwareManager(hm hardwareManager) hardwareSystemOption {
	return func(bs *hardwareSystem) error {
		bs.ctx.camera = hm
		return nil
	}
}

func (h hardwareSystem) withEventBus(bus eventBus) hardwareSystemOption {
	return func(bs *hardwareSystem) error {
		bs.ctx.bus = bus
		bus.subscribe(bs.hsm.handleEvent)
		return nil
	}
}

func (h hardwareSystem) withNotifier(n notify.Notifier) hardwareSystemOption {
	return func(bs *hardwareSystem) error {
		bs.ctx.notifier = n
		return nil
	}
}

func newHardwareOnlySystem(ctx context.Context, store Store, cfg *BroadcastConfig, logOutput func(v ...any), options ...hardwareSystemOption) (*hardwareSystem, error) {
	if ctx.Done() == nil {
		return nil, errors.New("context must be cancellable")
	}

	// Handy log wrapper that shims with interfaces that like the
	// classic func(string, ...interface{}) signature.
	// This can be used by a lot of the components here.
	log := func(msg string, args ...interface{}) {
		logForBroadcast(cfg, logOutput, msg, args...)
	}

	var man BroadcastManager

	// This will get called in the case that events are published to
	// the event bus but our context is cancelled. This might happen if a routine
	// is used to do a broadcast start and this function returns. We'll save them
	// to the config and then load them next time we perform checks.
	storeEventsAfterCtx := func(event event) {
		log("storing event after cancel: %s", event.String())
		try(
			man.Save(nil, func(_cfg *BroadcastConfig) {
				_cfg.Events = append(_cfg.Events, event.String())
			}),
			"could not update config with callback",
			log,
		)
	}

	bus := newBasicEventBus(ctx, storeEventsAfterCtx, log)

	// This context will be used by the state machines for access to our bits and bobs.
	broadcastContext := &broadcastContext{cfg, man, store, nil, nil, bus, nil, logOutput, nil}

	// The hardware state machine will be responsible for the external camera hardware
	// state.
	hsm := newHardwareStateMachine(broadcastContext)
	bus.subscribe(hsm.handleEvent)

	sys := &hardwareSystem{broadcastContext, hsm, log}

	// Apply any options to the system.
	for _, opt := range options {
		err := opt(sys)
		if err != nil {
			return nil, fmt.Errorf("could not apply option to broadcast system: %w", err)
		}
	}

	return sys, nil
}

// TestHardwareStopAndRestart tests the hardware state machine handling of stop and reset
// requests when shutdown actions are available and not available.
func TestHardwareStopAndRestart(t *testing.T) {
	const testSiteKey = 7845764367

	_ = func(n int) []event {
		var events []event
		for i := 0; i < n; i++ {
			events = append(events, timeEvent{})
		}
		return events
	}

	tests := []struct {
		desc               string
		cfg                func(*BroadcastConfig)
		finalHardwareState state
		initialEvent       event
		hardwareMan        hardwareManager
		newBroadcastMan    func(*testing.T, *BroadcastConfig) BroadcastManager

		// Leave unset to use default max ticks.
		// Some tests may require more ticks to reach the final state.
		requiredTicks int

		expectedEvents []event
		expectedLogs   []string
		expectedNotify map[int64]map[notify.Kind][]string
	}{
		{
			desc: "normal hardware stop, without shutdown actions",
			cfg: func(c *BroadcastConfig) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOn"
				c.ControllerMAC = 1
				c.CameraMac = 2
			},
			finalHardwareState: &hardwareOff{},
			initialEvent:       hardwareStopRequestEvent{},
			hardwareMan:        newDummyHardwareManager(withInitialCameraState(true)),
			newBroadcastMan: func(t *testing.T, c *BroadcastConfig) BroadcastManager {
				return newDummyManager(t, c)
			},
			expectedEvents: []event{hardwareStopRequestEvent{},
				hardwareShutdownFailedEvent{},
				timeEvent{},
				timeEvent{},
				hardwareStoppedEvent{},
			},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
		{
			desc: "normal hardware stop, with shutdown actions",
			cfg: func(c *BroadcastConfig) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOn"
				c.ControllerMAC = 1
				c.CameraMac = 2
				c.ShutdownActions = "shutdown"
			},
			finalHardwareState: &hardwareOff{},
			initialEvent:       hardwareStopRequestEvent{},
			hardwareMan:        newDummyHardwareManager(withInitialCameraState(true)),
			newBroadcastMan: func(t *testing.T, c *BroadcastConfig) BroadcastManager {
				return newDummyManager(t, c)
			},
			expectedEvents: []event{hardwareStopRequestEvent{}, timeEvent{}, timeEvent{}, hardwareShutdownEvent{}, timeEvent{}, hardwareStoppedEvent{}},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
		{
			desc: "hardware restart, without shutdown actions",
			cfg: func(c *BroadcastConfig) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOn"
				c.ControllerMAC = 1
				c.CameraMac = 2
			},
			finalHardwareState: &hardwareOn{},
			initialEvent:       hardwareResetRequestEvent{},
			hardwareMan:        newDummyHardwareManager(withInitialCameraState(true)),
			newBroadcastMan: func(t *testing.T, c *BroadcastConfig) BroadcastManager {
				return newDummyManager(t, c)
			},
			expectedEvents: []event{
				hardwareResetRequestEvent{},
				hardwareShutdownFailedEvent{},
				timeEvent{},
				timeEvent{},
				hardwareStoppedEvent{},
				timeEvent{},
				timeEvent{},
				hardwareStartedEvent{},
			},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
		{
			desc: "hardware restart, with shutdown actions",
			cfg: func(c *BroadcastConfig) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOn"
				c.ControllerMAC = 1
				c.CameraMac = 2
				c.ShutdownActions = "shutdown"
			},
			finalHardwareState: &hardwareOn{},
			initialEvent:       hardwareResetRequestEvent{},
			hardwareMan:        newDummyHardwareManager(withInitialCameraState(true)),
			newBroadcastMan: func(t *testing.T, c *BroadcastConfig) BroadcastManager {
				return newDummyManager(t, c)
			},
			expectedEvents: []event{
				hardwareResetRequestEvent{},
				timeEvent{},
				timeEvent{},
				hardwareShutdownEvent{},
				timeEvent{},
				hardwareStoppedEvent{},
				timeEvent{},
				timeEvent{},
				hardwareStartedEvent{},
			},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			logRecorder := newLogRecorder(t)

			ctx, _ := context.WithCancel(context.Background())

			// Apply broadcast config modifications
			// and update the broadcast state based on the initial state.
			cfg := &BroadcastConfig{}
			tt.cfg(cfg)

			// Use a monkey patch to replace time.Now() with our own time.
			// This will be updated before each tick to simulate time passing.
			testTime := time.Now()
			monkey.Patch(time.Now, func() time.Time { return testTime })
			defer monkey.Unpatch(time.Now)

			sys, err := newHardwareOnlySystem(
				ctx,
				newDummyStore(),
				cfg,
				logRecorder.log,
				hardwareSys.withEventBus(newMockEventBus(func(msg string, args ...interface{}) { logForBroadcast(cfg, logRecorder.log, msg, args...) })),
				hardwareSys.withBroadcastManager(tt.newBroadcastMan(t, cfg)),
				hardwareSys.withHardwareManager(tt.hardwareMan),
				hardwareSys.withNotifier(newMockNotifier()),
			)
			if err != nil {
				t.Fatalf("failed to create broadcast system: %v", err)
			}

			if tt.initialEvent != nil {
				sys.ctx.bus.publish(tt.initialEvent)
			}

			// Tick until we reach the final state. It's expected this occurs within
			// reasonable time otherwise we have a problem.
			const defaultMaxTicks = 10
			for tick := 0; true; tick++ {
				// Test test case overwrite the default max ticks.
				maxTicks := defaultMaxTicks
				if tt.requiredTicks > 0 {
					maxTicks = tt.requiredTicks
				}

				if tick > maxTicks {
					t.Errorf("failed to reach expected state after %d ticks", maxTicks)
					return
				}

				// We've replaced time.Now() with the monkey patch, but it means we need to
				// manually advance time before ticking the broadcast system.
				testTime = testTime.Add(1 * time.Minute)

				err = sys.tick()
				if err != nil {
					t.Errorf("failed to tick broadcast system: %v", err)
					return
				}
				if stateToString(sys.hsm.currentState) == stateToString(tt.finalHardwareState) {
					break
				}
			}

			// Check the events that we got.
			err = sys.ctx.bus.(*mockEventBus).checkEvents(tt.expectedEvents)
			if err != nil {
				t.Errorf("unexpected events: %v", err)
			}

			// Check the logs that we got.
			err = logRecorder.checkLogs(tt.expectedLogs)
			if err != nil {
				t.Errorf("unexpected logs: %v", err)
			}

			// Check we got expected notifications.
			err = sys.ctx.notifier.(*mockNotifier).checkNotifications(tt.expectedNotify)
			if err != nil {
				t.Errorf("unexpected notifications: %v", err)
			}

			// Also check if we ended up with the correct broadcast hardware machine state.
			if stateToString(sys.hsm.currentState) != stateToString(tt.finalHardwareState) {
				t.Errorf("unexpected state after handling started event: got %v, want %v",
					stateToString(sys.hsm.currentState), stateToString(tt.finalHardwareState))
			}
		})
	}
}
