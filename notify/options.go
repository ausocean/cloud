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
	"errors"
	"time"
)

// Option is a functional option supplied to Init.
type Option func(*Notifier) error

// Lookup is a function that returns the recipients and their
// corresponding notification period for a given site and notification
// kind. It is used with WithRecipientLookup.
type Lookup func(int64, Kind) ([]string, time.Duration, error)

// WithSender sets the sender email address.
func WithSender(sender string) Option {
	return func(n *Notifier) error {
		n.sender = sender
		return nil
	}
}

// WithRecipient sets a single recipient email address.
func WithRecipient(recipient string) Option {
	return func(n *Notifier) error {
		n.recipients = []string{recipient}
		return nil
	}
}

// WithRecipients sets multiple recipient email addresses.
func WithRecipients(recipients []string) Option {
	return func(n *Notifier) error {
		n.recipients = recipients
		return nil
	}
}

// WithRecipientLookup sets a function to look up multiple recipients
// and their corresponding notification period given a site key and a
// notification kind.
func WithRecipientLookup(lookup Lookup) Option {
	return func(n *Notifier) error {
		n.lookup = lookup
		return nil
	}
}

// WithFilter applies a filter string. If multiple WithFilter options
// are applied, they form a compound conjunctive filter.
// Specifiying an empty filter string clears the filter.
func WithFilter(filter string) Option {
	return func(n *Notifier) error {
		if filter == "" {
			n.filters = nil
			return nil
		}
		n.filters = append(n.filters, filter)
		return nil
	}
}

// WithStore applies a TimeStore for notification persistence.
// Combine with WithPeriod to enforce a minimum notification period.
// See also TimeStore.
func WithStore(store TimeStore) Option {
	return func(n *Notifier) error {
		n.store = store
		return nil
	}
}

// WithPeriod sets the minimum notification period, which is used in
// conjunction with a TimeStore.
func WithPeriod(period time.Duration) Option {
	return func(n *Notifier) error {
		n.period = period
		return nil
	}
}

// WithSecrets applies the secrets necessary for sending email,
// notably the public and private mail API keys. This is always
// required, unless testing.
func WithSecrets(secrets map[string]string) Option {
	return func(n *Notifier) error {
		var ok bool
		n.publicKey, ok = secrets["mailjetPublicKey"]
		if !ok {
			return errors.New("mailjetPublicKey secret not found")
		}
		n.privateKey, ok = secrets["mailjetPrivateKey"]
		if !ok {
			return errors.New("mailjetPrivateKey secret not found")
		}
		return nil
	}
}
