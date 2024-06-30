//go:build nowatchdog
// +build nowatchdog

/*
DESCRIPTION
  nowatchdog.go sets the notifyWatchdog global to false.

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

func init() {
	notifyWatchdog = false
}
