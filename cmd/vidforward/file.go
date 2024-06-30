/*
DESCRIPTION
  files.go provides the functionality required for saving and loading
  broadcastManager state. This includes marshalling/unmarshalling overrides.

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
	"fmt"
	"os"

	"github.com/ausocean/cloud/cmd/vidforward/global"
)

// inTest is used to indicate if we are within a test; some functionality is not
// employed in this case.
var inTest bool

// The file name for the broadcast manager state save.
const fileName = "state.json"

// BroadcastBasic is a crude version of the Broadcast used to simplify
// marshal/unmarshal overriding.
type BroadcastBasic struct {
	MAC
	URLs   []string
	Status string
}

// ManagerBasic is a crude version of the BroadcastManager struct use to simplify
// marshal/unmarshal overriding.
type ManagerBasic struct {
	Broadcasts map[MAC]*Broadcast
}

// MarshalJSON calls the default marshalling behaviour for the BroadcastBasic
// struct using the information from b.
func (b Broadcast) MarshalJSON() ([]byte, error) {
	return json.Marshal(BroadcastBasic{
		MAC:    b.mac,
		URLs:   b.urls,
		Status: b.status,
	})
}

// UnmarshalJSON unmarshals into a value of the BroadcastBasic struct and then
// populates a Broadcast value.
func (b *Broadcast) UnmarshalJSON(data []byte) error {
	var bm BroadcastBasic
	err := json.Unmarshal(data, &bm)
	if err != nil {
		return fmt.Errorf("could not unmarshal JSON: %w", err)
	}

	b.mac = bm.MAC
	b.urls = bm.URLs
	b.status = bm.Status

	return nil
}

// MarshalJSON calls the default marshaller for a ManagerBasic value using data
// from a broadcastManager value.
func (m *broadcastManager) MarshalJSON() ([]byte, error) {
	return json.Marshal(ManagerBasic{Broadcasts: m.broadcasts})
}

// UnmarshalJSON populates a ManagerBasic value from the provided data and then
// populates the receiver broadcastManager to a usable state based on this data.
func (m *broadcastManager) UnmarshalJSON(data []byte) error {
	var mb ManagerBasic
	err := json.Unmarshal(data, &mb)
	if err != nil {
		return fmt.Errorf("could not unmarshal JSON: %w", err)
	}

	m.broadcasts = mb.Broadcasts
	m.slateExitSignals = make(map[MAC]chan struct{})
	m.log = global.GetLogger()

	notifier, err := newWatchdogNotifier(m.log, terminationCallback(m))
	if err != nil {
		return fmt.Errorf("could not create watchdog notifier: %w", err)
	}
	m.dogNotifier = notifier

	for mac, b := range m.broadcasts {
		if b.status == statusSlate {
			sigCh := make(chan struct{})
			m.slateExitSignals[mac] = sigCh
			rv, err := m.getPipeline(mac)
			if err != nil {
				return fmt.Errorf("could not get revid pipeline: %v", err)
			}
			if !inTest {
				err := writeSlateAndCheckErrors(rv, sigCh, m.log)
				if err != nil {
					return fmt.Errorf("couldn't write slate for MAC %v: %w", mac, err)
				}
			}
		}
	}

	return nil
}

// save utilises marshalling functionality to save the broadcastManager state
// to a file.
func (m *broadcastManager) save() error {
	f, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("could not open file: %w", err)
	}
	defer f.Close()

	bytes, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("could not marshal broadcast manager: %w", err)
	}

	_, err = f.Write(bytes)
	if err != nil {
		return fmt.Errorf("could not write bytes to file: %w", err)
	}

	return nil
}

// load populates a broadcastManager value based on the previously saved state.
func (m *broadcastManager) load() error {
	bytes, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("could not read state file: %w", err)
	}

	err = json.Unmarshal(bytes, &m)
	if err != nil {
		return fmt.Errorf("could not unmarshal state data: %w", err)
	}
	return nil
}
