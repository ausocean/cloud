//go:build debug
// +build debug

/*
DESCRIPTION
  When this file is included in build by using the debug build tag, the logging
  level is changed to debug.

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

import "github.com/ausocean/utils/logging"

func init() {
	loggingLevel = logging.Debug
}
