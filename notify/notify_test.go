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

	"github.com/ausocean/cloud/gauth"
)

const (
	projectID = "test"
	kind      = "test"
	message   = "This is a test."
	recipient = "testing@ausocean.org"
)

// testStore implements a dummy time store for testing purposes.
type testStore struct {
	Attempted int
	Delivered int
}

// TestStore tests the time store functionality.
// For this test, we supply a test store without any secrets.
func TestStore(t *testing.T) {
	ctx := context.Background()

	n := Notifier{}
	ts := testStore{}
	err := n.Init(WithStore(&ts))
	if err != nil {
		t.Errorf("Init failed with error: %v", err)
	}

	// Even numbered attempts should not be delivered.
	tests1 := []struct {
		attempted int
		delivered int
	}{
		{
			attempted: 1,
			delivered: 1,
		},
		{
			attempted: 2,
			delivered: 1,
		},
		{
			attempted: 3,
			delivered: 2,
		},
	}

	for i, test := range tests1 {
		err = n.Send(ctx, 0, kind, message)
		if err != nil {
			t.Errorf("Send #%d failed with error: %v", i, err)
		}
		if ts.Attempted != test.attempted {
			t.Errorf("Expected attempted to be %d, got  %d", test.attempted, ts.Attempted)
		}
		if ts.Delivered != test.delivered {
			t.Errorf("Expected delivered to be %d, got %d", test.delivered, ts.Delivered)
		}
	}

	// Now try with filters.
	tests2 := []struct {
		filter    string
		attempted int
		delivered int
	}{
		{
			filter:    "test",
			attempted: 4,
			delivered: 2,
		},
		{
			filter:    "test",
			attempted: 5,
			delivered: 3,
		},
		{
			filter:    "Error:",
			attempted: 5,
			delivered: 3,
		},
	}
	for i, test := range tests2 {
		// Re-initialize with the filter.
		err = n.Init(WithFilter(test.filter), WithStore(&ts))
		if err != nil {
			t.Errorf("Init failed with error: %v", err)
		}
		err = n.Send(ctx, 0, kind, message)
		if err != nil {
			t.Errorf("Send #%d failed with error: %v", i, err)
		}
		if ts.Attempted != test.attempted {
			t.Errorf("Expected attempted to be %d, got  %d", test.attempted, ts.Attempted)
		}
		if ts.Delivered != test.delivered {
			t.Errorf("Expected delivered to be %d, got %d", test.delivered, ts.Delivered)
		}
	}
}

// TestSend tests sending an actual email.
// For this test, we supply secrets and a test recipient.
// It is recommended to run this only locally, as it sends actual emails.
func TestSend(t *testing.T) {
	if os.Getenv("TEST_SECRETS") == "" {
		t.Skip("TEST_SECRETS required for TestSend")
	}

	ctx := context.Background()
	n := Notifier{}

	secrets, err := gauth.GetSecrets(ctx, projectID, nil)
	if err != nil {
		t.Errorf("Could not get secrets for %s: %v", projectID, err)
	}

	err = n.Init(WithSecrets(secrets), WithRecipient(recipient))
	if err != nil {
		t.Errorf("Init failed with error: %v", err)
	}

	err = n.Send(ctx, 0, kind, message)
	if err != nil {
		t.Errorf("Send failed with error: %v", err)
	}
}

// Sendable alternates between returning true and false.
func (ts *testStore) Sendable(ctx context.Context, skey int64, key string) (bool, error) {
	ts.Attempted++
	if ts.Attempted%2 == 0 {
		return false, nil
	} else {
		return true, nil
	}
}

// Sent just increments the sent counter.
func (ts *testStore) Sent(ctx context.Context, skey int64, key string) error {
	ts.Delivered++
	return nil
}
