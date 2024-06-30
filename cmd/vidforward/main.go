/*
DESCRIPTION
  vidforward is a service for receiving video from cameras and then forwarding to
  youtube. By acting as the RTMP encoder (instead of the camera) vidforward can enable
  persistent streams by sending slate images during camera downtime.

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
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"reflect"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/ausocean/av/container/mts"
	"github.com/ausocean/av/revid"
	"github.com/ausocean/cloud/cmd/vidforward/global"
	"github.com/ausocean/utils/logging"
	"gopkg.in/natefinch/lumberjack.v2"
)

// This is the path to the vidforward configuration.
// This contains parameters such as log level and logging filters.
const configFileName = "/etc/vidforward/config.json"

// Server defaults.
const (
	defaultPort = "8080"
	defaultHost = ""
)

// Logging configuration.
const (
	logPath      = "/var/log/vidforward/vidforward.log"
	logMaxSize   = 500 // MB
	logMaxBackup = 10
	logMaxAge    = 28 // days
	logSuppress  = true
)

// recvErrorDelay is a delay used when there's recv issues. It is intended to
// prevent spamming from a single client.
const recvErrorDelay = 7 * time.Second

type MAC string

// The possible states for a broadcast.
const (
	statusActive = "active"
	statusSlate  = "slate"
	statusCreate = "create"
	statusPlay   = "play"
)

// Broadcast is representative of a broadcast to be forwarded.
type Broadcast struct {
	mac    MAC          // MAC address of the device from which the video is being received.
	urls   []string     // The destination youtube RTMP URLs.
	status string       // The broadcast status i.e. active or slate.
	rv     *revid.Revid // The revid pipeline which will handle forwarding to youtube.
}

// equal checks to see if the broadcast is equal to another broadcast.
// NOTE: This is not a deep equal, and is only used to check if a broadcast
// should be updated.
func (b *Broadcast) equal(other Broadcast) bool {
	return b.mac == other.mac &&
		b.status == other.status &&
		reflect.DeepEqual(b.urls, other.urls)
}

// broadcastManager manages a map of Broadcasts we expect to be forwarding video
// for. The broadcastManager is communicated with through a series of HTTP request
// handlers. There is a basic REST API through which we can add/delete broadcasts,
// and a recv handler which is invoked when a camera wishes to get its video
// forwarded to youtube.
type broadcastManager struct {
	broadcasts          map[MAC]*Broadcast
	slateExitSignals    map[MAC]chan struct{} // Used to signal to stop writing slate image.
	lastLoggedNonActive map[MAC]time.Time     // Used to log non-active MACs every minute.
	log                 logging.Logger
	dogNotifier         *watchdogNotifier
	mu                  sync.Mutex
}

// newBroadcastManager returns a new broadcastManager with the provided logger.
func newBroadcastManager(l logging.Logger) (*broadcastManager, error) {
	m := &broadcastManager{
		log:              l,
		broadcasts:       make(map[MAC]*Broadcast),
		slateExitSignals: make(map[MAC]chan struct{}),
	}
	notifier, err := newWatchdogNotifier(l, terminationCallback(m))
	if err != nil {
		return nil, err
	}
	m.dogNotifier = notifier
	return m, nil
}

// terminationCallback provides a callback that saves the provided
// broadcastManagers state.
func terminationCallback(m *broadcastManager) func() {
	return func() {
		err := m.save()
		if err != nil {
			m.log.Error("could not save on notifier termination signal", "error", err)
			return
		}
		m.log.Info("successfully saved broadcast manager state on termination signal")
		logTrace(m.log.Debug, m.log.Warning)
	}
}

// loadConfig loads the vidforward configuration file. This primarily concerns logging
// configuration for the time being, with the intended use case of debugging.
func (m *broadcastManager) loadConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.log.Info("loading logger config file")
	data, err := ioutil.ReadFile(configFileName)
	if err != nil {
		return fmt.Errorf("could not read config file: %w", err)
	}

	var cfg struct {
		LogLevel         string   `json:"LogLevel"`
		LogSuppress      bool     `json:"LogSuppress"`
		LogCallerFilters []string `json:"LogCallerFilters"`
	}

	if err = json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("could not unmarshal config file: %w", err)
	}

	m.log.Debug("logger config loaded", "cfg", cfg)
	m.log.(*logging.JSONLogger).SetLevel(map[string]int8{
		"debug":   logging.Debug,
		"info":    logging.Info,
		"warning": logging.Warning,
		"error":   logging.Error,
		"fatal":   logging.Fatal,
	}[cfg.LogLevel])
	m.log.(*logging.JSONLogger).SetSuppress(cfg.LogSuppress)
	m.log.(*logging.JSONLogger).SetCallerFilters(cfg.LogCallerFilters...)

	return nil
}

// This is a callback that can be used by file watchers to reload the config.
func (m *broadcastManager) onConfigChange() {
	err := m.loadConfig()
	if err != nil {
		m.log.Error("could not load config", "error", err)
		return
	}
}

// recvHandler handles recv requests for video forwarding. The MAC is firstly
// checked to ensure it is "active" i.e. should be sending data, and then the
// video is extracted from the request body and provided to the revid pipeline
// corresponding to said MAC.
// Clips of MPEG-TS h264 are the only accepted format and codec.
func (m *broadcastManager) recv(w http.ResponseWriter, r *http.Request) {
	done := m.dogNotifier.handlerInvoked("recv")
	defer done()

	q := r.URL.Query()
	ma := MAC(q.Get("ma"))

	// Check that we're not receiving video when we shouldn't be. There's
	// two conditions when this can happen; when the MAC is not mapped to a
	// broadcast, or when the broadcast is in slate mode.
	// It's expected this might happen a little bit under normal operation.
	// It's difficult to get the camera power timing right, so we might
	// receive a request before the camera has been registered, or after
	// we've transitioned into slate mode.
	// If this happens too much however, it may indicate a problem.
	var reason string
	switch {
	case !m.isActive(ma):
		reason = "forward request mac is not mapped, doing nothing"
		fallthrough
	case m.getStatus(ma) == statusSlate:
		if reason == "" {
			reason = "cannot receive video for this mac, status is slate"
		}

		// We don't want to clutter the logs so only log non-active MACs every
		// minute.
		const logNonActiveInternal = 1 * time.Minute
		last, ok := m.lastLoggedNonActive[ma]
		if !ok || ok && time.Now().Sub(last) > logNonActiveInternal {
			m.errorLogWrite(m.log, w, reason, "mac", ma)
			m.lastLoggedNonActive[ma] = time.Now()
		}

		// Stall the client with a delay to prevent spamming. Probably cause timeout
		// on client.
		time.Sleep(recvErrorDelay)
		return

	default: // Continue (seems like mac is active and we're not in slate.)
	}

	const videoPin = "V0"
	sizeStr := q.Get(videoPin)
	size, err := strconv.Atoi(sizeStr)
	if err != nil || size <= 0 {
		m.errorLogWrite(m.log, w, "invalid video size", "error", err, "size str", sizeStr)
		return
	}

	// Prepare HTTP response with received video size and device mac.
	resp := map[string]interface{}{"ma": ma, "V0": size}

	mtsClip, err := io.ReadAll(r.Body)
	if err != nil {
		m.errorLogWrite(m.log, w, "could not read forward request body", "error", err)
		return
	}
	defer r.Body.Close()

	if len(mtsClip)%mts.PacketSize != 0 {
		m.errorLogWrite(m.log, w, "invalid clip length", "length", len(mtsClip))
		return
	}

	// Extract the pure h264 from the MPEG-TS clip.
	h264Clip, err := mts.Extract(mtsClip)
	if err != nil {
		m.errorLogWrite(m.log, w, "could not extract m.264 from the MPEG-TS clip", "error", err)
		return
	}

	rv, err := m.getPipeline(ma)
	if err != nil {
		m.errorLogWrite(m.log, w, "could not get revid pipeline", "mac", ma, "error", err)
		return
	}

	for i, frame := range h264Clip.Frames() {
		_, err := rv.Write(frame.Media)
		if err != nil {
			m.errorLogWrite(m.log, w, "could not write frame", "no.", i, "error", err)
			return
		}
	}

	// Return response to client as JSON.
	jsn, err := json.Marshal(resp)
	if err != nil {
		m.errorLogWrite(m.log, w, "could not get json for response", "error", err)
		return
	}
	fmt.Fprint(w, string(jsn))
}

// control handles control API requests.
func (m *broadcastManager) control(w http.ResponseWriter, r *http.Request) {
	done := m.dogNotifier.handlerInvoked("control")
	defer func() {
		if r := recover(); r != nil {
			m.log.Error("panicked in control request!", "error", r.(error).Error(), "stack", string(debug.Stack()))
		}
		done()
	}()

	m.log.Info("control request", "method", r.Method)
	switch r.Method {
	case http.MethodPut:
		m.processRequest(w, r, m.createOrUpdate)
	case http.MethodDelete:
		m.processRequest(w, r, m.delete)
	default:
		m.errorLogWrite(m.log, w, "unhandled http method", "method", r.Method)
	}
	m.log.Info("finished handling control request")
}

// slate handles slate API requests to upload a new slate video.
func (m *broadcastManager) slate(w http.ResponseWriter, r *http.Request) {
	done := m.dogNotifier.handlerInvoked("slate")
	defer done()

	if r.Method != http.MethodPost {
		http.Error(w, "invalid request method", http.StatusMethodNotAllowed)
		return
	}

	file, _, err := r.FormFile("slate-file")
	if err != nil {
		m.errorLogWrite(m.log, w, "could not get slate file from form", "error", err)
		return
	}
	defer file.Close()

	// This will overwrite the slate file if it already exists.
	dst, err := os.OpenFile(slateFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		m.errorLogWrite(m.log, w, "could not open slate file", "error", err)
		return
	}
	defer dst.Close()

	n, err := io.Copy(dst, file)
	if err != nil {
		m.errorLogWrite(m.log, w, "could not copy slate file", "error", err)
		return
	}

	// Return response to client as JSON.
	jsn, err := json.Marshal(map[string]interface{}{"size": n})
	if err != nil {
		m.errorLogWrite(m.log, w, "could not get json for response", "error", err)
		return
	}
	fmt.Fprint(w, string(jsn))
}

// processRequest unmarshals the broadcast data object from the request into
// a Broadcast value, and then performs the provided action with that value.
func (m *broadcastManager) processRequest(w http.ResponseWriter, r *http.Request, action func(Broadcast) error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		m.errorLogWrite(m.log, w, "could not read request body", "body", r.Body)
		return
	}
	defer r.Body.Close()

	var broadcast Broadcast
	err = json.Unmarshal(body, &broadcast)
	if err != nil {
		m.errorLogWrite(m.log, w, "could not marshal data", "error", err)
		return
	}

	err = action(broadcast)
	if err != nil {
		m.errorLogWrite(m.log, w, "could not perform action", "method", r.Method, "error", err)
		return
	}

	err = m.save()
	if err != nil {
		m.errorLogWrite(m.log, w, "could not save manager state", "error", err)
	}
}

// getPipeline gets the revid pipeline corresponding to a provided device MAC.
// If it hasn't been created yet, it's created, and if it hasn't been started yet
// (or just created) then it is started.
func (m *broadcastManager) getPipeline(ma MAC) (*revid.Revid, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.broadcasts[ma]
	if !ok {
		panic("shouldn't be getting pipeline if this mac isn't registered")
	}

	var err error
	b.rv, err = m.initOrStartPipeline(b.rv, b.urls)
	if err != nil {
		return nil, fmt.Errorf("could not init or start pipeline: %v", err)
	}

	return b.rv, nil
}

// initOrStartPipeline ensures that provided Revid pointer points to an
// initialised and running revid pipeline.
func (m *broadcastManager) initOrStartPipeline(rv *revid.Revid, urls []string) (*revid.Revid, error) {
	var err error
	if rv == nil {
		rv, err = newRevid(m.log, urls)
		if err != nil {
			return nil, fmt.Errorf("could not create new revid: %v", err)
		}
	}
	if !rv.Running() {
		err = rv.Start()
		if err != nil {
			return nil, fmt.Errorf("could not start revid pipeline: %v", err)
		}
	}
	return rv, nil
}

// getStatus gets the broadcast's status corresponding to the provided MAC.
func (m *broadcastManager) getStatus(ma MAC) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.broadcasts[ma]
	if !ok {
		return ""
	}
	return v.status
}

// isActive returns true if a MAC is registered to the broadcast manager.
func (m *broadcastManager) isActive(ma MAC) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.broadcasts[ma]
	return ok
}

// createOrUpdate creates or updates a Broadcast record. If the record already
// exists, it will be updated with the new data. If it doesn't exist, a new
// revid pipeline will be created and started. Actions occur according to the
// status field of the broadcast i.e. whether we expect data from a source
// or write the slate image.
func (m *broadcastManager) createOrUpdate(broadcast Broadcast) error {
	m.log.Debug("create or update", "mac", broadcast.mac)
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try to get any old broadcasts for the provided MAC.
	maybeOld, ok := m.broadcasts[broadcast.mac]
	if ok {
		if maybeOld.rv != nil {
			m.log.Debug("stopping old revid pipeline", "mac", broadcast.mac)
			closeDone := make(chan struct{})
			go func() { maybeOld.rv.Stop(); close(closeDone) }()
			select {
			case <-closeDone:
				m.log.Debug("stopped old revid pipeline", "mac", broadcast.mac)
			case <-time.After(5 * time.Second):
				m.log.Warning("could not stop old revid pipeline, looks like we'll end up with some leaked memory then :(", "mac", broadcast.mac)
			}
		}
	}

	var err error
	maybeOld.rv, err = newRevid(m.log, broadcast.urls)
	if err != nil {
		return fmt.Errorf("could not create new revid: %w", err)
	}
	maybeOld.rv.Start()

	m.log.Info("updating configuration for mac", "mac", broadcast.mac)
	signal, ok := m.slateExitSignals[broadcast.mac]
	if ok {
		close(signal)
		delete(m.slateExitSignals, broadcast.mac)
	}

	if broadcast.status == statusSlate {
		// First create a signal that can be used to stop the slate writing routine.
		// This will be provided to the writeSlate routine below.
		signalCh := make(chan struct{})
		m.slateExitSignals[broadcast.mac] = signalCh

		err = writeSlateAndCheckErrors(maybeOld.rv, signalCh, m.log)
		if err != nil {
			return fmt.Errorf("could not write slate and check for errors: %w", err)
		}
	}

	// We need to give the revid pipeline to the new broadcast record.
	broadcast.rv = maybeOld.rv

	// And then replace the old record with the new one in the map.
	m.broadcasts[broadcast.mac] = &broadcast

	return nil
}

// delete removes a broadcast from the record.
func (m *broadcastManager) delete(broadcast Broadcast) error {
	m.mu.Lock()
	b, ok := m.broadcasts[broadcast.mac]
	if !ok {
		return errors.New("no broadcast by that mac in record")
	}
	b.rv.Stop()
	delete(m.broadcasts, broadcast.mac)
	m.mu.Unlock()
	return nil
}

func main() {
	host := flag.String("host", defaultHost, "Host IP to run video forwarder on.")
	port := flag.String("port", defaultPort, "Port to run video forwarder on.")
	flag.Parse()

	if *host == "" || net.ParseIP(*host) == nil {
		panic(fmt.Sprintf("invalid host, host: %s", *host))
	}

	// Create lumberjack logger to handle logging to file.
	fileLog := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    logMaxSize,
		MaxBackups: logMaxBackup,
		MaxAge:     logMaxAge,
	}

	// Create logger that we call methods on to log, which in turn writes to the
	// lumberjack and netloggers.
	log := logging.New(loggingLevel, io.MultiWriter(fileLog), logSuppress)

	global.SetLogger(log)

	m, err := newBroadcastManager(log)
	if err != nil {
		log.Fatal("could not create new broadcast manager", "error", err)
	}

	// Try to load any previous state. There may be a previous state if the
	// watchdog did a process restart.
	err = m.load()
	if err != nil {
		log.Warning("could not load previous state", "error", err)
	}

	// Try to load the config file.
	err = m.loadConfig()
	if err != nil {
		log.Warning("could not load config file", "error", err)
	}

	// Set up a file watcher to watch the config file. This will allow us
	// to perform updates to configuration while the service is running.
	watchFile(configFileName, m.onConfigChange, log)

	http.HandleFunc("/recv", m.recv)
	http.HandleFunc("/control", m.control)
	http.HandleFunc("/slate", m.slate)

	go m.dogNotifier.notify()

	log.Info("listening", "host", *host, "port", *port)
	http.ListenAndServe(*host+":"+*port, nil)
}
