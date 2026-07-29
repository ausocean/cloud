package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/cmd/oceantv/forwarding"
	"github.com/ausocean/cloud/cmd/oceantv/hardware"
	"github.com/ausocean/cloud/cmd/oceantv/manager"
	"github.com/ausocean/cloud/notify"
	"github.com/stretchr/testify/assert"
)

func TestGetHardwareStateStorage(t *testing.T) {
	tests := []struct {
		name         string
		initialState state
	}{
		{"test hardware off", newHardwareOff()},
		{"test hardware on", newHardwareOn()},
		{"test hardware starting", newHardwareStarting(&broadcastContext{hardware: &dummyHardwareManager{}, logOutput: t.Log, notifier: newMockNotifier()})},
		{"test hardware stopping", newHardwareStopping(minimalMockBroadcastContext(t))},
		{"test hardware restarting", newHardwareRestarting(minimalMockBroadcastContext(t))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateStr := hardwareStateToString(tt.initialState)
			gotState := getHardwareState(&broadcastContext{cfg: &Cfg{HardwareState: stateStr}, logOutput: t.Log, notifier: newMockNotifier()})
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
			bus := event.NewBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &Cfg{}
			bCtx.man = newDummyManager(t, bCtx.cfg)
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.Subscribe(sm.handleEvent)
			bus.Publish(event.HardwareStopped{})

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
			bus := event.NewBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &Cfg{}
			bCtx.man = newDummyManager(t, bCtx.cfg)
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.Subscribe(sm.handleEvent)
			bus.Publish(event.HardwareStopFailed{})

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
			bus := event.NewBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &Cfg{}
			bCtx.man = newDummyManager(t, bCtx.cfg)
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.Subscribe(sm.handleEvent)
			bus.Publish(event.HardwareStartFailed{})

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
			bus := event.NewBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &Cfg{}
			bCtx.man = newDummyManager(t, bCtx.cfg)
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.Subscribe(sm.handleEvent)
			bus.Publish(event.HardwareStarted{})

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
			bus := event.NewBasicEventBus(ctx, nil, func(string, ...interface{}) {})

			bCtx.fwd = newDummyForwardingService()
			bCtx.cfg = &Cfg{}
			bCtx.man = newDummyManager(t, bCtx.cfg)
			bCtx.bus = bus

			sm := newHardwareStateMachine(bCtx)
			sm.currentState = tt.initialState

			bus.Subscribe(sm.handleEvent)
			bus.Publish(event.HardwareResetRequest{})

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
	for _, eStr := range hs.ctx.cfg.Events {
		e := event.StringToEvent(eStr)
		hs.log("publishing stored event: %s", e.String())
		hs.ctx.bus.Publish(e)
	}

	// Remove stored events we just published from the config.
	err := hs.ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.Events = nil })
	if err != nil {
		return fmt.Errorf("could not clear config events: %w", err)
	}

	hs.ctx.bus.Publish(event.Time{time.Now()})
	return nil
}

type hardwareSystemOption func(*hardwareSystem) error

func (h hardwareSystem) withBroadcastManager(bm manager.Broadcast) hardwareSystemOption {
	return func(bs *hardwareSystem) error {
		bs.ctx.man = bm
		return nil
	}
}

func (h hardwareSystem) withBroadcastService(bs Svc) hardwareSystemOption {
	return func(b *hardwareSystem) error {
		b.ctx.svc = bs
		return nil
	}
}

func (h hardwareSystem) withForwardingService(fs forwarding.Service) hardwareSystemOption {
	return func(bs *hardwareSystem) error {
		bs.ctx.fwd = fs
		return nil
	}
}

func (h hardwareSystem) withHardwareManager(hm hardware.Manager) hardwareSystemOption {
	return func(bs *hardwareSystem) error {
		bs.ctx.hardware = hm
		return nil
	}
}

func (h hardwareSystem) withEventBus(bus event.EventBus) hardwareSystemOption {
	return func(bs *hardwareSystem) error {
		bs.ctx.bus = bus
		bus.Subscribe(bs.hsm.handleEvent)
		return nil
	}
}

func (h hardwareSystem) withNotifier(n notify.Notifier) hardwareSystemOption {
	return func(bs *hardwareSystem) error {
		bs.ctx.notifier = n
		return nil
	}
}

func newHardwareOnlySystem(ctx Ctx, store Store, cfg *Cfg, logOutput func(v ...any), options ...hardwareSystemOption) (*hardwareSystem, error) {
	if ctx.Done() == nil {
		return nil, errors.New("context must be cancellable")
	}

	// Handy log wrapper that shims with interfaces that like the
	// classic func(string, ...interface{}) signature.
	// This can be used by a lot of the components here.
	log := func(msg string, args ...interface{}) {
		broadcast.LogForBroadcast(cfg, logOutput, msg, args...)
	}

	var man manager.Broadcast

	// This will get called in the case that events are published to
	// the event bus but our context is cancelled. This might happen if a routine
	// is used to do a broadcast start and this function returns. We'll save them
	// to the config and then load them next time we perform checks.
	storeEventsAfterCtx := func(e event.Event) {
		log("storing event after cancel: %s", e.String())
		try(
			man.Save(nil, func(_cfg *Cfg) {
				_cfg.Events = append(_cfg.Events, e.String())
			}),
			"could not update config with callback",
			log,
		)
	}

	bus := event.NewBasicEventBus(ctx, storeEventsAfterCtx, log)

	// This context will be used by the state machines for access to our bits and bobs.
	broadcastContext := &broadcastContext{cfg, man, store, nil, nil, bus, nil, logOutput, nil}

	// The hardware state machine will be responsible for the external camera hardware
	// state.
	hsm := newHardwareStateMachine(broadcastContext)
	bus.Subscribe(hsm.handleEvent)

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

	_ = func(n int) []event.Event {
		var events []event.Event
		for i := 0; i < n; i++ {
			events = append(events, event.Time{})
		}
		return events
	}

	tests := []struct {
		desc               string
		cfg                func(*Cfg)
		finalHardwareState state
		initialEvent       event.Event
		hardwareMan        hardware.Manager
		newBroadcastMan    func(*testing.T, *Cfg) manager.Broadcast

		// Leave unset to use default max ticks.
		// Some tests may require more ticks to reach the final state.
		requiredTicks int

		expectedEvents []event.Event
		expectedLogs   []string
		expectedNotify map[int64]map[notify.Kind][]string
	}{
		{
			desc: "normal hardware stop, without shutdown actions",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOn"
				c.ControllerMAC = 1
				c.CameraMac = 2
			},
			finalHardwareState: &hardwareOff{},
			initialEvent:       event.HardwareStopRequest{},
			hardwareMan:        newDummyHardwareManager(withInitialCameraState(true)),
			newBroadcastMan: func(t *testing.T, c *Cfg) manager.Broadcast {
				return newDummyManager(t, c)
			},
			expectedEvents: []event.Event{event.HardwareStopRequest{},
				event.HardwareShutdownFailed{},
				event.Time{},
				event.Time{},
				event.HardwareStopped{},
			},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
		{
			desc: "normal hardware stop, with shutdown actions",
			cfg: func(c *Cfg) {
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
			initialEvent:       event.HardwareStopRequest{},
			hardwareMan:        newDummyHardwareManager(withInitialCameraState(true)),
			newBroadcastMan: func(t *testing.T, c *Cfg) manager.Broadcast {
				return newDummyManager(t, c)
			},
			expectedEvents: []event.Event{event.HardwareStopRequest{}, event.Time{}, event.Time{}, event.HardwareShutdown{}, event.Time{}, event.HardwareStopped{}},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
		{
			desc: "hardware restart, without shutdown actions",
			cfg: func(c *Cfg) {
				c.Enabled = true
				c.SKey = testSiteKey
				c.Start = time.Now().Add(-1 * time.Hour)
				c.End = time.Now().Add(1 * time.Hour)
				c.HardwareState = "hardwareOn"
				c.ControllerMAC = 1
				c.CameraMac = 2
			},
			finalHardwareState: &hardwareOn{},
			initialEvent:       event.HardwareResetRequest{},
			hardwareMan:        newDummyHardwareManager(withInitialCameraState(true)),
			newBroadcastMan: func(t *testing.T, c *Cfg) manager.Broadcast {
				return newDummyManager(t, c)
			},
			expectedEvents: []event.Event{
				event.HardwareResetRequest{},
				event.HardwareShutdownFailed{},
				event.Time{},
				event.Time{},
				event.HardwareStopped{},
				event.Time{},
				event.Time{},
				event.HardwareStarted{},
			},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
		{
			desc: "hardware restart, with shutdown actions",
			cfg: func(c *Cfg) {
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
			initialEvent:       event.HardwareResetRequest{},
			hardwareMan:        newDummyHardwareManager(withInitialCameraState(true)),
			newBroadcastMan: func(t *testing.T, c *Cfg) manager.Broadcast {
				return newDummyManager(t, c)
			},
			expectedEvents: []event.Event{
				event.HardwareResetRequest{},
				event.Time{},
				event.Time{},
				event.HardwareShutdown{},
				event.Time{},
				event.HardwareStopped{},
				event.Time{},
				event.Time{},
				event.HardwareStarted{},
			},
			expectedLogs:   []string{},
			expectedNotify: map[int64]map[notify.Kind][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			logRecorder := newLogRecorder(t)

			ctx, _ := context.WithCancel(context.Background())

			// Use a monkey patch to replace time.Now() with a stable test time.
			// This will be updated before each tick to simulate time passing.
			testTime := fixedBroadcastTestTime(t)
			monkey.Patch(time.Now, func() time.Time { return testTime })
			defer monkey.Unpatch(time.Now)

			// Apply broadcast config modifications
			// and update the broadcast state based on the initial state.
			cfg := &Cfg{}
			tt.cfg(cfg)

			sys, err := newHardwareOnlySystem(
				ctx,
				newDummyStore(),
				cfg,
				logRecorder.log,
				hardwareSys.withEventBus(newMockEventBus(func(msg string, args ...interface{}) { broadcast.LogForBroadcast(cfg, logRecorder.log, msg, args...) })),
				hardwareSys.withBroadcastManager(tt.newBroadcastMan(t, cfg)),
				hardwareSys.withHardwareManager(tt.hardwareMan),
				hardwareSys.withNotifier(newMockNotifier()),
			)
			if err != nil {
				t.Fatalf("failed to create broadcast system: %v", err)
			}

			if tt.initialEvent != nil {
				sys.ctx.bus.Publish(tt.initialEvent)
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
					t.Errorf(
						"failed to reach expected state after %d ticks, current state: %s, wanted state: %s",
						maxTicks,
						stateToString(sys.hsm.currentState),
						stateToString(tt.finalHardwareState),
					)
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

func TestHardwareRestartingMarshaling(t *testing.T) {
	substate := &hardwareShuttingDown{
		stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(&broadcastContext{}, time.Date(2025, 2, 17, 13, 45, 0, 0, time.UTC)),
	}

	original := &hardwareRestarting{
		stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(&broadcastContext{}, time.Date(2025, 2, 17, 13, 50, 0, 0, time.UTC)),
		Substate: &hardwareStopping{
			stateWithTimeoutFields: newStateWithTimeoutFieldsWithLastEntered(&broadcastContext{}, time.Date(2025, 2, 17, 13, 55, 0, 0, time.UTC)),
			Substate:               substate, // Nested state
		},
	}

	data, err := json.Marshal(original)
	assert.NoError(t, err, "Failed to marshal JSON")

	unmarshaled := newHardwareRestarting(minimalMockBroadcastContext(t))
	err = json.Unmarshal(data, unmarshaled)
	assert.NoError(t, err, "Failed to unmarshal JSON")

	// Validate main struct fields.
	assert.Equal(t, original.LastEntered, unmarshaled.LastEntered, "LastEntered mismatch")
	assert.Equal(t, original.Timeout, unmarshaled.Timeout, "Timeout mismatch")

	// Validate substate (hardwareStopping).
	hwStopping, ok := unmarshaled.Substate.(*hardwareStopping)
	assert.True(t, ok, "Substate type assertion for hardwareStopping failed")
	assert.Equal(t, original.Substate.(*hardwareStopping).LastEntered, hwStopping.LastEntered, "hardwareStopping LastEntered mismatch")
	assert.Equal(t, original.Substate.(*hardwareStopping).Timeout, hwStopping.Timeout, "hardwareStopping Timeout mismatch")

	// Validate nested substate (hardwareStarting).
	hwStarting, ok := hwStopping.Substate.(*hardwareShuttingDown)
	assert.True(t, ok, "Nested substate type assertion for hardwareStarting failed")
	assert.Equal(t, substate.LastEntered, hwStarting.LastEntered, "hardwareStarting LastEntered mismatch")
	assert.Equal(t, substate.Timeout, hwStarting.Timeout, "hardwareStarting Timeout mismatch")
}
