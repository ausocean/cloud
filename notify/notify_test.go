/*
LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  It is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package notify

import (
	"context"
	"os"
	"testing"
	"time"
)

const (
	projectID = "test"
	kind      = "test"
	sender    = "vidgrindservice@gmail.com"
	recipient = "vidgrindservice@gmail.com"
	message   = "This is a test."
	minPeriod = 1
)

// testStore implements a dummy time store for testing purposes.
type testStore struct {
	count int
}

// TestStore tests the time store functionality.
func TestStore(t *testing.T) {
	ctx := context.Background()

	n := Notifier{}
	ts := testStore{}
	err := n.Init(ctx, "", "", &ts)
	if err != nil {
		t.Errorf("Init failed with error: %v", err)
	}

	err = n.Send(ctx, 0, kind, recipient, message, minPeriod)
	if err != nil {
		t.Errorf("Send failed with error: %v", err)
	}
	err = n.Send(ctx, 0, kind, recipient, message, minPeriod)
	if err != nil {
		t.Errorf("Send failed with error: %v", err)
	}
	err = n.Send(ctx, 0, kind, recipient, message, minPeriod)
	if err != nil {
		t.Errorf("Send failed with error: %v", err)
	}
}

// TestSend tests sending an actual email.
// It is recommended to run this only locally.
func TestSend(t *testing.T) {
	if os.Getenv("TEST_SECRETS") == "" {
		t.Skip("TEST_SECRETS required for TestSend")
	}

	ctx := context.Background()
	n := Notifier{}

	err := n.Init(ctx, projectID, sender, nil)
	if err != nil {
		t.Errorf("Init failed with error: %v", err)
	}

	err = n.Send(ctx, 0, kind, recipient, message, minPeriod)
	if err != nil {
		t.Errorf("Send failed with error: %v", err)
	}
}

// Get alternates between returning the current time and a time long past.
func (ts *testStore) Get(skey int64, key string) (time.Time, error) {
	ts.count++
	if ts.count%2 == 0 {
		return time.Now(), nil
	} else {
		return time.Time{}, nil
	}
}

// Set is a no-op.
func (ts *testStore) Set(skey int64, key string, t time.Time) error {
	return nil
}
