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
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	mailjet "github.com/mailjet/mailjet-apiv3-go"
)

const defaultSender = "vidgrindservice@gmail.com"

// Notifier represents a notifier that uses the MailJet API to send email.
type Notifier struct {
	mutex      sync.Mutex    // Lock access.
	sender     string        // Sender email address.
	recipients []string      // Recipient email addresses.
	lookup     Lookup        // Recipient lookup function (optional).
	store      TimeStore     // Notification store (optional).
	period     time.Duration // Minimum notification period (optional)
	filters    []string      // Message filters (optional).
	publicKey  string        // Public key for accessing MailJet API.
	privateKey string        // Public key for accessing MailJet API.
}

// Kind represents a kind of notification.
type Kind string

// Errors.
var ErrNoRecipient = errors.New("no recipient")

// Init initializes a notifier with the supplied options. See
// WithSender, WithRecipient, WithFilter, WithStore and WithSecrets
// for a description of the various options. Secrets are required to
// send actual emails using the MailJet API, but can be omitted during
// testing. It is permissable to re-initalize a Notifier with
// different options, however missing options will revert to their
// defaults.
func (n *Notifier) Init(options ...Option) error {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	// Set default values.
	n.sender = defaultSender
	n.recipients = nil
	n.lookup = nil
	n.store = nil
	n.period = 0
	n.filters = nil
	n.publicKey = ""
	n.privateKey = ""

	// Apply options.
	for i, opt := range options {
		err := opt(n)
		if err != nil {
			return fmt.Errorf("could not apply option # %d, %v", i, err)
		}
	}

	return nil
}

// Send sends an email message, depending on what options are present.
// With filters, then all filters must match in order to send.
// With persistence, then the message is sent only if it was not sent to the same recipient recently.
func (n *Notifier) Send(ctx context.Context, skey int64, kind Kind, msg string) error {
	recipients, period, err := n.Recipients(skey, kind)
	if err != nil {
		return err
	}

	for _, f := range n.filters {
		if !strings.Contains(msg, f) {
			log.Printf("filter '%s' applied: not sending %s message to %s", f, string(kind), recipients)
			return nil
		}
	}

	if n.store != nil {
		sendable, err := n.store.Sendable(ctx, skey, period, string(kind)+"."+recipients)
		if err != nil {
			log.Printf("store.IsSendable returned error: %v", err)
		}
		if !sendable {
			log.Printf("too soon to send %s message to %s", kind, recipients)
			return nil
		}
	}

	log.Printf("sending %s message to %s", kind, recipients)

	if n.publicKey != "" && n.privateKey != "" {
		clt := mailjet.NewMailjetClient(n.publicKey, n.privateKey)
		var recipients mailjet.RecipientsV31
		for _, recipient := range n.recipients {
			recipients = append(recipients, mailjet.RecipientV31{Email: recipient})
		}
		info := []mailjet.InfoMessagesV31{{
			From:     &mailjet.RecipientV31{Email: n.sender},
			To:       &recipients,
			Subject:  strings.Title(string(kind)) + " notification",
			TextPart: msg,
		}}

		msgs := mailjet.MessagesV31{Info: info}
		_, err := clt.SendMailV31(&msgs)
		if err != nil {
			return fmt.Errorf("could not send mail: %w", err)
		}
	}

	if n.store != nil {
		err := n.store.Sent(ctx, skey, string(kind)+"."+recipients)
		if err != nil {
			log.Printf("store.Sent returned error: %v", err)
		}
	}

	return nil
}

// Recipients returns a comma-separated list of recipients and their
// corresponding minimum notification period for the given site and
// notification kind. It uses the WithRecipientLookup function if
// supplied, else defaults to the recipients supplied by either
// WithRecipient or WithRecipients and the period supplied by
// WithPeriod. ErrNoRecipient is returned if there are no recipients.
func (n *Notifier) Recipients(skey int64, kind Kind) (string, time.Duration, error) {
	recipients := n.recipients
	period := n.period
	var err error
	if n.lookup != nil {
		recipients, period, err = n.lookup(skey, kind)
	}
	if err == nil && len(recipients) == 0 {
		return "", 0, ErrNoRecipient
	}
	return strings.Join(recipients, ","), period, err
}
