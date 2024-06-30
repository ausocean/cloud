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
	projectID          = "test"
	kind          Kind = "test"
	message            = "This is a test."
	testRecipient      = "somebody"
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

	// Even numbered attempts should not be delivered.
	tests := []struct {
		recipient string
		filter    string
		attempted int
		delivered int
	}{
		{
			recipient: "nobody",
			attempted: 1,
			delivered: 1,
		},
		{
			recipient: "nobody",
			attempted: 2,
			delivered: 1,
		},
		{
			recipient: "nobody",
			attempted: 3,
			delivered: 2,
		},
		{
			recipient: "nobody",
			filter:    "test",
			attempted: 4,
			delivered: 2,
		},
		{
			recipient: "nobody",
			filter:    "test",
			attempted: 5,
			delivered: 3,
		},
		{
			recipient: "nobody",
			filter:    "Error:",
			attempted: 5,
			delivered: 3,
		},
	}

	for i, test := range tests {
		err := n.Init(WithRecipient(test.recipient), WithFilter(test.filter), WithStore(&ts))
		if err != nil {
			t.Errorf("%d: Init failed with error: %v", i, err)
		}
		err = n.Send(ctx, 0, kind, message)
		if err != nil {
			t.Errorf("%d: Send failed with error: %v", i, err)
		}
		if ts.Attempted != test.attempted {
			t.Errorf("%d: Expected attempted to be %d, got  %d", i, test.attempted, ts.Attempted)
		}
		if ts.Delivered != test.delivered {
			t.Errorf("%d: Expected delivered to be %d, got %d", i, test.delivered, ts.Delivered)
		}
	}
}

// TestSend tests sending actual emails.
// For this test, we supply Mailjet API secrets and recipients.
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

	tests := []struct {
		recipients []string
		ok         bool
	}{
		{
			recipients: []string{"testing@ausocean.org"},
			ok:         true,
		},
		{
			recipients: []string{"user1@rfc-5322.invalid", "user2@rfc-5322.invalid"},
			ok:         true,
		},
		{
			recipients: []string{"rfc-5322.invalid"},
			ok:         false,
		},
	}

	for i, test := range tests {
		err := n.Init(WithSecrets(secrets), WithRecipients(test.recipients))
		if err != nil {
			t.Errorf("%d: Init failed with error: %v", i, err)
		}
		err = n.Send(ctx, 0, kind, message)
		if test.ok && err != nil {
			t.Errorf("%d: Unexpected error %v", i, err)
		}
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

// TestRecipients tests recipient lookup.
func TestRecipients(t *testing.T) {
	n := Notifier{}
	err := n.Init(WithRecipientLookup(testLookup))
	if err != nil {
		t.Errorf("Init with error: %v", err)
	}

	want := n.Recipients(0, kind)
	if want != testRecipient {
		t.Errorf("Recipients returned %s, expected %s", want, testRecipient)
	}
}

// testLookup is our recipient lookup function.
func testLookup(skey int64, kind Kind) []string {
	if kind == "test" {
		return []string{testRecipient}
	}
	return []string{""}
}
