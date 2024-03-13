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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	mailjet "github.com/mailjet/mailjet-apiv3-go"

	"github.com/ausocean/cloud/gauth"
)

const (
	defaultOpsPeriod = 60
)

// TimeStore is an interface for getting and setting notification times.
type TimeStore interface {
	Set(int64, string, time.Time) error   // Set a time for a key.
	Get(int64, string) (time.Time, error) // Get a time for a key.
}

// Notifier represents a notifier.
type Notifier struct {
	mutex       sync.Mutex // Lock access.
	initialized bool       // True if initialized.
	sender      string     // Sender email address.
	store       TimeStore  // Notification persistence (optional).
	publicKey   string     // Public key for accessing MailJet API.
	privateKey  string     // Public key for accessing MailJet API.
}

// Init initializes a notifier for use with the given project. It
// looks up secrets from either a file or Google Storage bucket
// specified by the <PROJECTID>_SECRETS environment variable. The
// optional (non-nil) timestore keeps track of notification times, to
// avoid sending too frequently.
// For testing, projectID and sender should be empty strings.
func (n *Notifier) Init(ctx context.Context, projectID string, sender string, store TimeStore) error {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if n.initialized {
		return nil
	}

	n.sender = sender
	n.store = store

	if projectID == "" {
		n.initialized = true
		return nil
	}

	secrets, err := gauth.GetSecrets(ctx, projectID, nil)
	if err != nil {
		return fmt.Errorf("could not get secrets: %w", err)
	}

	var ok bool
	n.publicKey, ok = secrets["mailjetPublicKey"]
	if !ok {
		return errors.New("mailjetPublicKey secret not found")
	}
	n.privateKey, ok = secrets["mailjetPrivateKey"]
	if !ok {
		return errors.New("mailjetPrivateKey secret not found")
	}

	n.initialized = true
	return nil
}

// SendOps sends an email to the email address defined by the
// OPS_EMAIL environment variable at most every OPS_PERIOD minutes.
func (n *Notifier) SendOps(ctx context.Context, skey int64, kind, msg string) error {
	recipient := os.Getenv("OPS_EMAIL")
	if recipient == "" {
		return errors.New("OPS_EMAIL undefined")
	}
	p := os.Getenv("OPS_PERIOD")
	mins, err := strconv.Atoi(p)
	if err != nil {
		log.Printf("defaulting to default OPS_PERIOD %d", defaultOpsPeriod)
		mins = defaultOpsPeriod
	}
	return n.Send(ctx, skey, kind, recipient, msg, mins)
}

// Send sends an email message to the given recipient, unless the same
// kind of email was sent to the same recipient recently.
func (n *Notifier) Send(ctx context.Context, skey int64, kind, recipient, msg string, mins int) error {
	if n.store != nil {
		t, err := n.store.Get(skey, kind+"."+recipient)
		if err != nil {
			log.Printf("error getting time: %v", err)
		}
		if time.Since(t) < time.Duration(mins)*time.Minute {
			log.Printf("too soon to send %s a %s message", recipient, kind)
			return nil // Recently notified.
		}
	}

	log.Printf("sending %s a %s message", recipient, kind)

	if n.sender != "" {
		clt := mailjet.NewMailjetClient(n.publicKey, n.privateKey)
		info := []mailjet.InfoMessagesV31{{
			From:     &mailjet.RecipientV31{Email: n.sender},
			To:       &mailjet.RecipientsV31{mailjet.RecipientV31{Email: recipient}},
			Subject:  strings.Title(kind) + " notification",
			TextPart: msg,
		}}

		msgs := mailjet.MessagesV31{Info: info}
		_, err := clt.SendMailV31(&msgs)
		if err != nil {
			return fmt.Errorf("could not send mail: %w", err)
		}
	}

	if n.store != nil {
		err := n.store.Set(skey, kind+"."+recipient, time.Now())
		if err != nil {
			log.Printf("error setting time: %v", err)
		}
	}

	return nil
}
