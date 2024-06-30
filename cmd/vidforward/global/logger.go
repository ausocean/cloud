/*
DESCRIPTION
  logger.go provides a "safe" global logger by following the singleton pattern.
  Usage of this should be avoided if possible, but in some instances it might be
  necessary, for example implementations of interfaces where logging is required
  but do not offer parameters where a logger can be passed as an argument.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package global

import "github.com/ausocean/utils/logging"

var logger *globalLogger = nil

type globalLogger struct {
	logging.Logger
}

// SetLogger sets the global logger. This must be set, and only once, before
// the GetLogger function is called. If these requirements are violated panics
// will occur.
func SetLogger(l logging.Logger) {
	if logger != nil {
		logger.Fatal("attempting set of already instantiated global logger")
	}
	logger = &globalLogger{l}
}

// GetLogger returns the global logger. If this has not been set, a panic will
// occur.
func GetLogger() logging.Logger {
	if logger == nil {
		panic("attempted get of uninstantiated global logger")
	}
	// We want to return the underlying logger.
	return logger.Logger
}
