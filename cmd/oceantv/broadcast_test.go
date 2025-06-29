/*
DESCRIPTION
  broadcast_test.go houses testing for functionality found in broadcast.go.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2022 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/notify"
	"github.com/ausocean/openfish/datastore"
)

// TestRemoveDate tests the removeDate helper function.
func TestRemoveDate(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "A Broadcast 23/04/15", want: "A Broadcast "},
		{in: "A Broadcast 04/23/15", want: "A Broadcast "},
		{in: "ABroadcast04/23/15", want: "ABroadcast"},
		{in: "ABroadcast04/23/15AStream", want: "ABroadcastAStream"},
	}

	for i, test := range tests {
		got := removeDate(test.in)
		if got != test.want {
			t.Errorf("did not get expected result for test no. %d \ngot: %s \nwant: %s", i, got, test.want)
		}
	}
}

// dummyManager is a dummy implementation of the broadcastManager interface.
type dummyManager struct {
	cfg                                                                *Cfg
	startDone                                                          chan struct{}
	saved, started, stopped, healthHandled, statusHandled, chatHandled bool
	Limiter                                                            RateLimiter
	t                                                                  *testing.T
	broadcastUnhealthy                                                 bool
}

type dummyManagerOption func(interface{}) error

func withRateLimiter(l RateLimiter) dummyManagerOption {
	return func(i interface{}) error {
		if s, ok := i.(*dummyManager); ok {
			s.Limiter = l
		}
		return nil
	}
}

func withBadHealth() dummyManagerOption {
	return func(i interface{}) error {
		if s, ok := i.(*dummyManager); ok {
			s.broadcastUnhealthy = true
		}
		return nil
	}
}

func newDummyManager(t *testing.T, cfg *Cfg, options ...dummyManagerOption) *dummyManager {
	t.Log("creating dummy manager")
	man := &dummyManager{
		t:         t,
		startDone: make(chan struct{}),
		cfg:       cfg,
	}
	for _, option := range options {
		option(man)
	}
	return man
}

func (d *dummyManager) CreateBroadcast(
	cfg *Cfg,
	store Store,
	svc BroadcastService,
) error {
	if d.Limiter != nil && !d.Limiter.RequestOK() {
		return ErrRequestLimitExceeded
	}
	return nil
}

func (d *dummyManager) StartBroadcast(
	ctx Ctx,
	cfg *Cfg,
	store Store,
	svc Svc,
	extStart func() error,
	onSuccess func(),
	onFailure func(error),
) {
	onSuccess()
	// This will only close the channel if it's not closed yet.
	defer func() {
		ok := true
		select {
		case _, ok = <-d.startDone:
		default:
		}
		if ok {
			close(d.startDone)
		}
	}()
	d.started = true
}
func (d *dummyManager) StopBroadcast(ctx Ctx, cfg *Cfg, store Store, svc Svc) error {
	d.stopped = true
	return nil
}
func (d *dummyManager) Save(ctx Ctx, update func(*BroadcastConfig)) error {
	d.saved = true
	if update != nil {
		update(d.cfg)
	}
	return nil
}
func (d *dummyManager) HandleStatus(ctx Ctx, cfg *Cfg, store Store, svc Svc, call BroadcastCallback) error {
	d.statusHandled = true
	return nil
}
func (d *dummyManager) HandleChatMessage(ctx Ctx, cfg *Cfg) error {
	d.chatHandled = true
	return nil
}
func (d *dummyManager) HandleHealth(ctx Ctx, cfg *Cfg, store Store, goodHealthCallback func(), badHealthCallback func(string)) error {
	d.healthHandled = true
	if d.broadcastUnhealthy {
		badHealthCallback("poor ingestion rate")
	} else {
		goodHealthCallback()
	}
	return nil
}
func (d *dummyManager) SetupSecondary(ctx Ctx, cfg *Cfg, store Store) error { return nil }

func (d *dummyManager) logf(format string, args ...interface{}) {
	if d.t == nil {
		return
	}
	d.t.Logf(format, args...)
}

// dummyStore is a dummy implementation of the datastore.Store interface.
// It basically does nothing and is used to test the broadcast functions.
type dummyStore struct {
	tokenBucketLimiter *OceanTokenBucketLimiter
}

type dummyStoreOption func(*dummyStore)

// newDummyStore creates a new dummyStore with the provided options.
func newDummyStore(options ...dummyStoreOption) *dummyStore {
	ds := &dummyStore{}
	for _, opt := range options {
		opt(ds)
	}
	return ds
}

// WithTokenBucketLimiter is an option function for setting the token bucket limiter in the dummyStore.
func WithTokenBucketLimiter(limiter *OceanTokenBucketLimiter) dummyStoreOption {
	return func(ds *dummyStore) {
		ds.tokenBucketLimiter = limiter
	}
}

func (d *dummyStore) IDKey(kind string, id int64) *Key { return nil }
func (d *dummyStore) NameKey(kind, name string) *Key   { return nil }
func (d *dummyStore) IncompleteKey(kind string) *Key   { return nil }
func (d *dummyStore) Get(ctx Ctx, key *Key, dst Ety) error {
	// Check if the key corresponds to the token bucket limiter and if it is set.
	if d.tokenBucketLimiter != nil {
		data, err := json.Marshal(d.tokenBucketLimiter)
		if err != nil {
			return fmt.Errorf("could not marshal token bucket limiter: %w", err)
		}
		if v, ok := dst.(*model.Variable); ok {
			v.Value = string(data)
			return nil
		}
	}
	// Return nil by default.
	return nil
}
func (d *dummyStore) DeleteMulti(ctx Ctx, keys []*Key) error { return nil }
func (d *dummyStore) NewQuery(kind string, keysOnly bool, keyParts ...string) datastore.Query {
	return nil
}
func (d *dummyStore) GetAll(ctx Ctx, q datastore.Query, dst interface{}) ([]*Key, error) {
	return nil, nil
}
func (d *dummyStore) Put(ctx Ctx, key *Key, src Ety) (*Key, error)          { return nil, nil }
func (d *dummyStore) Create(ctx Ctx, key *Key, src Ety) error               { return nil }
func (d *dummyStore) Update(ctx Ctx, key *Key, fn func(Ety), dst Ety) error { return nil }
func (d *dummyStore) Delete(ctx Ctx, key *Key) error                        { return nil }

// dummyService is a dummy implementation of the BroadcastService interface.
// It does nothing and is used to test the broadcast functions.
type dummyService struct {
	status string
	start  time.Time
}

type dummyServiceOption func(*dummyService)

func newDummyService(options ...dummyServiceOption) *dummyService {
	ds := &dummyService{}
	for _, opt := range options {
		opt(ds)
	}
	return ds
}

// WithStatus is an option function for fixing the returned status of the dummyService.
func WithStatus(status string) dummyServiceOption {
	return func(ds *dummyService) {
		ds.status = status
	}
}

// WithStart is an option function for fixing the returned scheduled start time of the dummyService's broadcast.
func WithStart(start time.Time) dummyServiceOption {
	return func(ds *dummyService) {
		ds.start = start
	}
}

func (d *dummyService) CreateBroadcast(
	ctx Ctx,
	broadcastName, description, streamName, privacy, resolution string,
	start, end time.Time,
	opts ...BroadcastOption,
) (ServerResponse, broadcast.IDs, string, error) {
	return nil, broadcast.IDs{}, "", nil
}

func (d *dummyService) StartBroadcast(
	name, bID, sID string,
	saveLink func(key, link string) error,
	extStart, extStop func() error,
	notify func(msg string) error,
	onLiveActions func() error,
) error {
	return nil
}
func (d *dummyService) BroadcastStatus(ctx Ctx, id string) (string, error) { return d.status, nil }
func (d *dummyService) BroadcastScheduledStartTime(ctx Ctx, id string) (time.Time, error) {
	return d.start, nil
}
func (d *dummyService) BroadcastHealth(ctx Ctx, id string) (string, error)    { return "", nil }
func (d *dummyService) RTMPKey(ctx Ctx, streamName string) (string, error)    { return "", nil }
func (d *dummyService) CompleteBroadcast(ctx Ctx, id string) error            { return nil }
func (d *dummyService) PostChatMessage(id, msg string) error                  { return nil }
func (d *dummyService) SetBroadcastPrivacy(ctx Ctx, id, privacy string) error { return nil }

type dummyForwardingService struct{}

func newDummyForwardingService() *dummyForwardingService                                  { return &dummyForwardingService{} }
func (v *dummyForwardingService) Stream(cfg *Cfg) error                                   { return nil }
func (v *dummyForwardingService) Slate(cfg *Cfg, opts ...SlateOption) error               { return nil }
func (v *dummyForwardingService) UploadSlate(cfg *Cfg, name string, file io.Reader) error { return nil }

type request struct {
	kind string
	time.Time
}

type dummyHardwareManager struct {
	hardwareHealthy   bool
	startCalled       bool
	shutdownCalled    bool
	stopCalled        bool
	checkMAC          bool
	volts             float64
	alarmVolts        float64
	chargeRate        float64
	cameraIsReporting bool
	controllerMAC     string
	cameraMAC         string
	latestRequest     request
	hwErr             error
}

func withHardwareFault() func(*dummyHardwareManager) {
	return func(h *dummyHardwareManager) {
		h.hardwareHealthy = false
	}
}

func withLowVoltage() func(*dummyHardwareManager) {
	return func(h *dummyHardwareManager) {
		h.volts = 24.0
	}
}

func withHardwareError(err error) func(*dummyHardwareManager) {
	return func(h *dummyHardwareManager) {
		h.hwErr = err
	}
}

func withMACSanitisation() func(*dummyHardwareManager) {
	return func(h *dummyHardwareManager) {
		h.checkMAC = true
	}
}

func withChargingFault() func(*dummyHardwareManager) {
	return func(h *dummyHardwareManager) {
		h.chargeRate = 0.0
	}
}

func withController(mac string) func(*dummyHardwareManager) {
	return func(h *dummyHardwareManager) {
		h.controllerMAC = mac
	}
}

func withCamera(mac string) func(*dummyHardwareManager) {
	return func(h *dummyHardwareManager) {
		h.cameraMAC = mac
	}
}

func withInitialCameraState(s bool) func(*dummyHardwareManager) {
	return func(h *dummyHardwareManager) {
		h.cameraIsReporting = s
	}
}

func newDummyHardwareManager(options ...func(*dummyHardwareManager)) *dummyHardwareManager {
	const (
		defaultVolts      = 24.8
		defaultAlarmVolts = 24.2
		defaultChargeRate = 0.01 // Volts per tick.
	)
	m := &dummyHardwareManager{
		volts:           defaultVolts,
		alarmVolts:      defaultAlarmVolts,
		chargeRate:      defaultChargeRate,
		hardwareHealthy: true,
		controllerMAC:   "00:00:00:00:00:01",
		cameraMAC:       "00:00:00:00:00:02",
	}
	for _, option := range options {
		option(m)
	}
	return m
}
func (h *dummyHardwareManager) voltage(ctx *broadcastContext) (float64, error) {
	// This is assuming we call this function every tick.
	h.volts += h.chargeRate
	return h.volts, nil
}
func (h *dummyHardwareManager) alarmVoltage(ctx *broadcastContext) (float64, error) {
	return h.alarmVolts, nil
}
func (h *dummyHardwareManager) isUp(ctx *broadcastContext, mac string) (bool, error) {
	if mac == h.controllerMAC {
		ctx.log("checking controller status, volts: %v, alarmVolts: %v, hardwareHealthy: %v", h.volts, h.alarmVolts, h.hardwareHealthy)
		if h.volts < h.alarmVolts {
			return false, nil
		}
		if !h.hardwareHealthy {
			return false, nil
		}
		return true, nil
	}

	if mac == h.cameraMAC {
		if !h.hardwareHealthy {
			return false, nil
		}
		ctx.log("checking camera status: %v", h.latestRequest)
		if h.latestRequest.kind != "" && time.Now().Sub(h.latestRequest.Time) > 1*time.Minute {
			switch h.latestRequest.kind {
			case "start":
				h.cameraIsReporting = true
			case "stop", "shutdown":
				h.cameraIsReporting = false
			default:
				panic("unknown request kind")
			}
			h.latestRequest = request{}
		}
		return h.cameraIsReporting, nil
	}

	return false, fmt.Errorf("could not get device: %w", datastore.ErrNoSuchEntity)
}

func (h *dummyHardwareManager) start(ctx *broadcastContext) {
	ctx.log("starting hardware")
	h.startCalled = true
	// Can't start if we're already shutting down.
	if h.latestRequest.kind != "shutdown" {
		h.latestRequest = request{"start", time.Now()}
	}
}
func (h *dummyHardwareManager) shutdown(ctx *broadcastContext) {
	ctx.log("shutting down hardware")
	h.shutdownCalled = true
	if ctx.cfg.ShutdownActions == "" {
		ctx.bus.publish(hardwareShutdownFailedEvent{})
		return
	}
	h.latestRequest = request{"shutdown", time.Now()}
}
func (h *dummyHardwareManager) stop(ctx *broadcastContext) {
	ctx.log("stopping hardware")
	h.stopCalled = true
	h.latestRequest = request{"stop", time.Now()}
}
func (h *dummyHardwareManager) publishEventIfStatus(ctx *broadcastContext, event event, status bool, mac int64, store Store, log func(format string, args ...interface{}), publish func(event event)) {
	if h.checkMAC && mac == 0 {
		publish(invalidConfigurationEvent{errors.New("camera mac is empty")})
		return
	}
	up, err := h.isUp(ctx, model.MacDecode(mac))
	if err != nil {
		publish(invalidConfigurationEvent{fmt.Errorf("could not get device: %w", err)})
		return
	}

	if status == true && up {
		publish(event)
	} else if status == false {
		publish(event)
	}
}
func (h *dummyHardwareManager) error(ctx *broadcastContext) (error, error) {
	if h.volts > h.alarmVolts {
		return None, nil
	}
	return h.hwErr, nil
}

// mockNotifier to implement Notifier interface.
type mockNotifier struct {
	// Holds sent messages for a site and kind.
	sent map[int64]map[notify.Kind][]string
}

func newMockNotifier() *mockNotifier {
	return &mockNotifier{sent: make(map[int64]map[notify.Kind][]string)}
}

func (m *mockNotifier) Send(ctx Ctx, skey int64, kind notify.Kind, msg string) error {
	if m.sent[skey] == nil {
		m.sent[skey] = make(map[notify.Kind][]string)
	}
	m.sent[skey][kind] = append(m.sent[skey][kind], msg)
	return nil
}

// checkNotifications checks that the messages contained in want were sent (contained in m.sent).
// The order of want messages in sent messages is not checked.
func (m *mockNotifier) checkNotifications(want map[int64]map[notify.Kind][]string) error {
	for skey, kinds := range want {
		for kind, msgs := range kinds {
			if len(m.sent[skey][kind]) != len(msgs) {
				return fmt.Errorf(
					"expected %d messages for site %d and kind %s, got %d. \nGot messages: %v, \nwant messages: %v",
					len(msgs),
					skey,
					kind,
					len(m.sent[skey][kind]),
					m.sent[skey][kind],
					msgs,
				)
			}
			for i, msg := range msgs {
				if !strings.Contains(msg, m.sent[skey][kind][i]) {
					return fmt.Errorf("expected message %s for site %d and kind %s, got %s", msg, skey, kind, m.sent[skey][kind][i])
				}
			}
		}
	}
	return nil
}

func (m *mockNotifier) Recipients(skey int64, k notify.Kind) ([]string, time.Duration, error) {
	return []string{}, 0, nil
}

// factory to create a broadcastContext with mock facilities.
func standardMockBroadcastContext(t *testing.T, hardwareHealthy bool) *broadcastContext {
	return &broadcastContext{
		store:     &dummyStore{},
		svc:       &dummyService{},
		camera:    &dummyHardwareManager{hardwareHealthy: hardwareHealthy},
		notifier:  newMockNotifier(),
		logOutput: t.Log,
	}
}

// factory to create a broadcastContext with minimal mock facilities.
func minimalMockBroadcastContext(t *testing.T) *broadcastContext {
	return &broadcastContext{
		logOutput: t.Log,
		notifier:  newMockNotifier(),
	}
}

type logRecorder struct {
	t    *testing.T
	logs []string
}

func newLogRecorder(t *testing.T) *logRecorder {
	return &logRecorder{t: t}
}

func (r *logRecorder) log(v ...any) {
	r.t.Log(v...)
	r.logs = append(r.logs, fmt.Sprintln(v...))
}

// Note this only checks that want are in the logs, not that they are the only logs.
// We also don't care about the order of the logs.
func (r *logRecorder) checkLogs(want []string) error {
	for _, w := range want {
		found := false
		for _, l := range r.logs {
			if strings.Contains(l, w) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("expected log not found: %s", w)
		}
	}
	return nil
}

// mockEventBus is a simple event bus that stores events when the context is
// cancelled.
type mockEventBus struct {
	disabled     bool
	handlers     []handler
	log          func(string, ...interface{})
	eventHistory []event
}

// newmockEventBus creates a new mockEventBus.
func newMockEventBus(log func(string, ...interface{})) *mockEventBus {
	return &mockEventBus{log: log}
}

func (bus *mockEventBus) subscribe(handler handler) { bus.handlers = append(bus.handlers, handler) }

func (bus *mockEventBus) publish(event event) {
	bus.eventHistory = append(bus.eventHistory, event)
	bus.log("publishing event: %s: %v", event.String(), event)

	for _, handler := range bus.handlers {
		err := handler(event)
		if err != nil {
			bus.log("error handling event: %s: %v", event.String(), err)
		}
	}
}

func (bus *mockEventBus) checkEvents(want []event) error {
	fmtError := func(want, got []event) error {
		return fmt.Errorf(
			"expected %d events, got %d, expected: %v, got: %v",
			len(want),
			len(got),
			eventsToStringSlice(want),
			eventsToStringSlice(got),
		)
	}

	// Basic check on length of expected and actual events
	if len(bus.eventHistory) != len(want) {
		return fmtError(want, bus.eventHistory)
	}

	// Check each published event matches the events we expected to see.
	for i, e := range bus.eventHistory {
		// Assuming you have an eventToString function
		if e.String() != want[i].String() {
			return fmtError(want, bus.eventHistory)
		}
	}
	return nil
}

type mockLimiter struct {
	Limited bool
}

func newMockLimiter(Limited bool) *mockLimiter {
	return &mockLimiter{Limited: Limited}
}

func (l *mockLimiter) RequestOK() bool {
	return !l.Limited
}
