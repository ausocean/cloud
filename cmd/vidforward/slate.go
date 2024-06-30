/*
DESCRIPTION
  slate.go houses vidforward slate functionality including the slate writing
  routine and HTTP request handlers for receiving of new slate videos.

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
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ausocean/av/codec/h264"
	"github.com/ausocean/av/device/file"
	"github.com/ausocean/utils/logging"
)

// @note this is temporary; slate names will be created based on device mac so that we can
// have a slate per device (or maybe multiple slates per device).
const slateFileName = "/home/saxon/go/src/github.com/ausocean/av/cmd/vidforward/slate.h264"

func writeSlateAndCheckErrors(dst io.Writer, signalCh chan struct{}, log logging.Logger) error {
	// Also create an errCh that will be used to communicate errors from the
	// writeSlate routine.
	errCh := make(chan error)

	go writeSlate(dst, errCh, signalCh, log)

	// We'll watch out for any errors that happen within a 5 second window. This
	// will indicate something seriously wrong with init, like a missing file etc.
	const startupWindowDuration = 5 * time.Second
	startupWindow := time.NewTimer(startupWindowDuration)
	select {

	// If this triggers first, we're all good.
	case <-startupWindow.C:
		log.Debug("out of error window")

		// We consider any errors after this either to be normal i.e. as a result
		// of stopping the slate input, or something that can not be handled, and
		// only logged, therefore we can close the error channel errCh now.
		// This will also let the routine know that errors can no longer be sent
		// down errCh.
		close(errCh)

	// This means we got a slate error pretty early and need to let caller know.
	case err := <-errCh:
		return fmt.Errorf("could not write slate image: %w", err)
	}
	return nil
}

// writeSlate is a routine that employs a file input device and h264 lexer to
// write a h264 encoded slate image to the provided revid pipeline.
func writeSlate(dst io.Writer, errCh chan error, exitSignal chan struct{}, log logging.Logger) {
	log.Info("writing slate")
	const (
		// Assume 25fps until this becomes configurable.
		slateFrameRate = 25

		loopSetting = true
		frameDelay  = time.Second / slateFrameRate
	)

	fileInput := file.NewWith(log, slateFileName, loopSetting)
	err := fileInput.Start()
	if err != nil {
		errCh <- err
		return
	}

	// This will wait for a signal from the provided slateExitSignal (or from a
	// timeout) to stop writing the slate by "Stopping" the file input which will
	// terminate the Lex function.
	go func() {
		slateTimeoutTimer := time.NewTimer(24 * time.Hour)
		select {
		case <-slateTimeoutTimer.C:
			log.Warning("slate timeout")
		case <-exitSignal:
			log.Info("slate exit signal")
		}
		log.Info("stopping file input")
		fileInput.Stop()
	}()

	// Begin lexing the slate file and send frames to rv pipeline. We'll stay in
	// here until file input closes or there's an unexpected error.
	err = h264.Lex(dst, fileInput, frameDelay)

	// If we get to this point, it means that the we've finished lexing for some
	// reason; let's figure out why.
	select {
	// The only reason we'd get a receive on errCh from this side is if its been
	// closed. This means that we've exceeded the "startup error" period, and that
	// either the error is normal from stopping the input, or we can no longer inform
	// the caller and just need to log the problem.
	case <-errCh:
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			log.Debug("got expected error", "error", err)
			return
		}
		log.Error("got unexpected error", "error", err)

	// This means that a problem occured pretty early in lexing.
	default:
		log.Error("unexpected error during lex startup", "error", err)
		errCh <- err
	}
	log.Error("finished writing slate")
}
