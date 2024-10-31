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

type Notifier interface {
	Send(context.Context, int64, Kind, string) error
	Recipients(int64, Kind) ([]string, time.Duration, error)
}

// Notifier represents a notifier that uses the Mailjet API to send email.
type MailjetNotifier struct {
	mutex      sync.Mutex    // Lock access.
	sender     string        // Sender email address.
	recipients []string      // Recipient email addresses.
	lookup     Lookup        // Recipient lookup function (optional).
	store      TimeStore     // Notification store (optional).
	period     time.Duration // Minimum notification period (optional)
	filters    []string      // Message filters (optional).
	publicKey  string        // Public key for accessing Mailjet API.
	privateKey string        // Public key for accessing Mailjet API.
}

// Kind represents a kind of notification.
type Kind string

// Errors.
var ErrNoRecipient = errors.New("no recipient")

// NewMailjetNotifier initializes a MailjetNotifier with the supplied
// options. See WithSender, WithRecipient, WithFilter, WithStore and
// WithSecrets for a description of the various options. Secrets are
// required to send actual emails using the Mailjet API, but can be
// omitted during testing.
func NewMailjetNotifier(options ...Option) (*MailjetNotifier, error) {
	n := &MailjetNotifier{}
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
			return nil, fmt.Errorf("could not apply option # %d, %v", i, err)
		}
	}

	return n, nil
}

// Send sends an email message, depending on what options are present.
// With filters, then all filters must match in order to send.
// With persistence, then the message is sent only if it was not sent to the same recipient recently.
func (n *MailjetNotifier) Send(ctx context.Context, skey int64, kind Kind, msg string) error {
	recipients, period, err := n.Recipients(skey, kind)
	if err != nil {
		return err
	}
	csvRecipients := strings.Join(recipients, ",")

	for _, f := range n.filters {
		if !strings.Contains(msg, f) {
			log.Printf("filter '%s' applied: not sending %s message to %s", f, string(kind), csvRecipients)
			return nil
		}
	}

	if n.store != nil {
		sendable, err := n.store.Sendable(ctx, skey, period, string(kind)+"."+csvRecipients)
		if err != nil {
			log.Printf("store.IsSendable returned error: %v", err)
		}
		if !sendable {
			log.Printf("too soon to send %s message to %s", kind, csvRecipients)
			return nil
		}
	}

	log.Printf("sending %s message to %s", kind, csvRecipients)

	if n.publicKey != "" && n.privateKey != "" {
		err = send(n.publicKey, n.privateKey, n.sender, recipients, strings.Title(string(kind))+" notification", msg)
		if err != nil {
			return fmt.Errorf("could not send mail: %w", err)
		}
	}

	if n.store != nil {
		err := n.store.Sent(ctx, skey, string(kind)+"."+csvRecipients)
		if err != nil {
			log.Printf("store.Sent returned error: %v", err)
		}
	}

	return nil
}

func send(publicKey, privateKey, sender string, recipients []string, subject, msg string) error {
	clt := mailjet.NewMailjetClient(publicKey, privateKey)
	var mjRecipients mailjet.RecipientsV31
	for _, recipient := range recipients {
		mjRecipients = append(mjRecipients, mailjet.RecipientV31{Email: recipient})
	}
	info := []mailjet.InfoMessagesV31{{
		From:     &mailjet.RecipientV31{Email: sender},
		To:       &mjRecipients,
		Subject:  subject,
		TextPart: msg,
	}}

	msgs := mailjet.MessagesV31{Info: info}
	_, err := clt.SendMailV31(&msgs)
	if err != nil {
		return fmt.Errorf("could not send mail: %w", err)
	}
	return nil
}

// Send sends an email message using the Mailjet API.
func Send(publicKey, privateKey, sender string, recipients []string, subject, msg string) error {
	return send(publicKey, privateKey, sender, recipients, subject, msg)
}

// Recipients returns a list of recipients and their corresponding
// minimum notification period for the given site and notification
// kind. It uses the WithRecipientLookup function if supplied, else
// defaults to the recipients supplied by either WithRecipient or
// WithRecipients and the period supplied by WithPeriod.
// ErrNoRecipient is returned if there are no recipients.
func (n *MailjetNotifier) Recipients(skey int64, kind Kind) ([]string, time.Duration, error) {
	recipients := n.recipients
	period := n.period
	var err error
	if n.lookup != nil {
		recipients, period, err = n.lookup(skey, kind)
	}
	var recipients_ []string
	for _, r := range recipients {
		if r != "" {
			recipients_ = append(recipients_, r)
		}
	}
	if len(recipients_) == 0 {
		return nil, 0, ErrNoRecipient
	}
	return recipients_, period, err
}
