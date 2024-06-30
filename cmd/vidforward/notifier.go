/*
DESCRIPTION
  notifier.go provides a tool for notifying a systemd watchdog under healthy
  operation of the vidforward service.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ausocean/utils/logging"
	"github.com/coreos/go-systemd/daemon"
)

// By default we assume we should be notifying a systemd watchdog. This can be
// toggled off by using the nowatchdog build tag (see nowatchdog.go file).
var notifyWatchdog = true

// watchdogNotifier keeps track of the watchdog interval from the external
// sysd service settings, the currently active request handlers and a curId
// field that is is incremented to generate new handler ids for storage.
type watchdogNotifier struct {
	watchdogInterval time.Duration
	activeHandlers   map[int]handlerInfo
	curId            int
	termCallback     func()
	log              logging.Logger
	mu               sync.Mutex
	haveRun          bool
}

// handlerInfo keeps track of a handlers name (for any logging purposes) and
// time at which the handler was invoked, which is later used to calculate time
// active and therefore heatlh.
type handlerInfo struct {
	name string
	time time.Time
}

// newWatchdogNotifier creates a new watchdogNotifier with the provided logger
// and termination callback that is called if a SIGINT or SIGTERM signal is
// received. Recommended use of this is an attempted state save.
func newWatchdogNotifier(l logging.Logger, termCallback func()) (*watchdogNotifier, error) {
	interval := 1 * time.Minute

	return &watchdogNotifier{
		activeHandlers:   make(map[int]handlerInfo),
		watchdogInterval: interval,
		log:              l,
		termCallback:     termCallback,
	}, nil
}

// notify is to be called as a routine. This is responsible for checking if the
// handlers are healthy and then notifying the watchdog if so, otherwise we
// wait, continue and check again until they are. If the handlers take too long to
// become healthy, we risk exceeding the watchdog interval causing a process restart.
// notify also starts a routine to monitor for any SIGINT or SIGTERM, upon which
// a callback that's provided at initialisation is called.
func (n *watchdogNotifier) notify() {
	notifyTicker := time.NewTicker(n.watchdogInterval / 2.0)

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
		sig := <-sigs
		n.log.Warning("received termination signal, calling termination callback", "signal", sig.String())
		n.termCallback()
	}()

	var consecutiveUnhealthyStates int
	for {
		const nUnhealthyStatesForTrace = 10
		if n.handlersUnhealthy() {
			consecutiveUnhealthyStates++
			if consecutiveUnhealthyStates >= nUnhealthyStatesForTrace {
				logTrace(n.log.Debug,n.log.Warning)
				consecutiveUnhealthyStates = 0
			}
			const unhealthyHandlerWait = 1 * time.Second
			time.Sleep(unhealthyHandlerWait)
			continue
		}
		consecutiveUnhealthyStates = 0

		<-notifyTicker.C

		if !notifyWatchdog {
			continue
		}

		if !n.haveRun {
			n.haveRun = true

			const clearEnvVars = false
			ok, err := daemon.SdNotify(clearEnvVars, daemon.SdNotifyReady)
			if err != nil {
				n.log.Fatal("unexpected watchog notify read error", "error", err)
			}

			if !ok {
				n.log.Fatal("watchdog notification not supported")
			}

			n.watchdogInterval, err = daemon.SdWatchdogEnabled(clearEnvVars)
			if err != nil {
				n.log.Fatal("unexpected watchdog error", "error", err)
			}

			if n.watchdogInterval == 0 {
				n.log.Fatal("Watchdog not enabled or this is the wrong PID")
			}

		}

		// If this fails for any reason it indicates a systemd service configuration
		// issue, and therefore programmer error, so do fatal log to cause crash.
		n.log.Debug("notifying watchdog")
		supported, err := daemon.SdNotify(false, daemon.SdNotifyWatchdog)
		if err != nil {
			n.log.Fatal("error from systemd watchdog notify", "error", err)
		}

		if !supported {
			n.log.Fatal("watchdog notification not supported")
		}
	}
}

// handlersUnhealthy returns true if it is detected that any handlers are unhealthy,
// that is, if they have been handling for longer than the unhealthyHandleDuration.
func (n *watchdogNotifier) handlersUnhealthy() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, info := range n.activeHandlers {
		const unhealthyHandleDuration = 30 * time.Second
		if time.Now().Sub(info.time) > unhealthyHandleDuration {
			n.log.Warning("handler unhealthy", "name", info.name)
			return true
		}
	}
	return false
}

// handlerInvoked is to be called at the start of a request handler to indicate
// that handling has begun. The name and start time of the handler is recorded
// in the active handlers map with a unique ID as the key. A function is returned
// that must be called at exit of the handler to indicate that handling has
// finished. It is recommended this be done using a defer statement immediately
// after receiveing it.
func (n *watchdogNotifier) handlerInvoked(name string) func() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.log.Info("handler invoked", "name", name)

	id := n.curId
	n.curId++
	n.activeHandlers[id] = handlerInfo{time: time.Now(), name: name}

	return func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		n.log.Info("handler done", "name", name)

		if _, ok := n.activeHandlers[id]; !ok {
			n.log.Fatal("handler id not in map", "name", name)
		}

		delete(n.activeHandlers, id)
	}
}
