/*
DESCRIPTION
  utils.go houses generic vidforward utilities and helper functions.

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
	"fmt"
	"net/http"
	"strconv"
	"runtime"

	"github.com/ausocean/av/codec/codecutil"
	"github.com/ausocean/av/revid"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
)

var loggingLevel = logging.Info

func newRevid(log logging.Logger, urls []string) (*revid.Revid, error) {
	if len(urls) <= 0 {
		return nil, fmt.Errorf("cannot have %d URLs", len(urls))
	}
	var outputs []uint8
	for _ = range urls {
		outputs = append(outputs, config.OutputRTMP)
	}
	cfg := config.Config{
		Logger:     log,
		Input:      config.InputManual,
		InputCodec: codecutil.H264_AU,
		Outputs:    outputs,
		RTMPURL:    urls,
		LogLevel:   logging.Debug,
	}
	return revid.New(
		cfg, nil)
}

// writeError logs an error and writes to w in JSON format.
func (m *broadcastManager) errorLogWrite(log logging.Logger, w http.ResponseWriter, msg string, args ...interface{}) {
	log.Error(msg, args...)
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprint(w, `{"er":"`+msg+`"}`)
}

// isMac returns true if the provided string is a valid mac address.
func isMac(m string) bool {
	if len(m) != 17 || m == "00:00:00:00:00:00" {
		return false
	}

	for i := 0; i <= 15; i++ {
		if (i+1)%3 == 0 && m[i] != ':' {
			return false
		}

		if (3-i)%3 != 0 {
			continue
		}

		_, err := strconv.ParseUint(m[i:i+2], 16, 64)
		if err != nil {
			return false
		}
	}
	return true
}

type Log func(msg string, args ...interface{})

func logTrace(debug, warning Log){
	const (
		maxStackTraceSize = 100000
		allStacks         = true
	)
	buf := make([]byte, maxStackTraceSize)
	n := runtime.Stack(buf, allStacks)
	if n > maxStackTraceSize && warning != nil {
		warning("stacktrace exceeded buffer size")
	}
	debug("got stacktrace at termination", "stacktrace", string(buf[:n]))
}
