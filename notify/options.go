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
)

type Option func(*Notifier) error

// WithSender sets the sender email address.
func WithSender(sender string) Option {
	return func(n *Notifier) error {
		n.sender = sender
		return nil
	}
}

// WithRecipient sets the recipient email address.
func WithRecipient(recipient string) Option {
	return func(n *Notifier) error {
		n.recipient = recipient
		return nil
	}
}

// WithFilter applies a filter string. If multiple WithFilter options
// are applied, they form a compound conjunctive filter.
func WithFilter(filter string) Option {
	return func(n *Notifier) error {
		n.filters = append(n.filters, filter)
		return nil
	}
}

// WithStore applies a TimeStore for notification persistence.
// See TimeStore.
func WithStore(store TimeStore) Option {
	return func(n *Notifier) error {
		n.store = store
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
