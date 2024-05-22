/*
NAME
  Ocean Bench notification functions.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2021-2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Bench. Ocean Bench is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean Bench is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	mailjet "github.com/mailjet/mailjet-apiv3-go"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

const (
	defaultOpsPeriod = 60
	monitorURL       = "http://netreceiver.appspot.com/monitor"
	senderEmail      = "vidgrindservice@gmail.com"
)

var secrets map[string]string

// notifyOps sends a notification to the email address defined by the
// OPS_EMAIL environment variable at most every OPS_PERIOD minutes.
func notifyOps(ctx context.Context, skey int64, kind, msg string) error {
	email := os.Getenv("OPS_EMAIL")
	if email == "" {
		return errors.New("OPS_EMAIL undefined")
	}
	p := os.Getenv("OPS_PERIOD")
	mins, err := strconv.Atoi(p)
	if err != nil {
		log.Printf("defaulting to default OPS_PERIOD %d", defaultOpsPeriod)
		mins = defaultOpsPeriod
	}
	return notify(ctx, skey, kind, email, msg, mins)
}

// notify sends a notification unless an identical notice was sent recently.
func notify(ctx context.Context, skey int64, kind, recipient, msg string, mins int) error {
	v, err := model.GetVariable(ctx, settingsStore, skey, "_"+kind+"."+recipient)
	switch err {
	case nil:
		if time.Since(v.Updated) < time.Duration(mins)*time.Minute {
			return nil // Recently notified, so nothing to do.
		}
	case datastore.ErrNoSuchEntity:
		break
	default:
		return err // Unexpected datastore error.
	}

	log.Printf("notify %s %s", kind, recipient)

	if secrets == nil {
		secrets, err = gauth.GetSecrets(ctx, projectID, nil)
		if err != nil {
			return fmt.Errorf("could not get secrets: %w", err)
		}
	}

	publicKey, ok := secrets["mailjetPublicKey"]
	if !ok {
		return errors.New("mailjetPublicKey secret not found")
	}
	privateKey, ok := secrets["mailjetPrivateKey"]
	if !ok {
		return errors.New("mailjetPrivateKey secret not found")
	}

	mailClient := mailjet.NewMailjetClient(publicKey, privateKey)
	msgInfo := []mailjet.InfoMessagesV31{{
		From:     &mailjet.RecipientV31{Email: senderEmail},
		To:       &mailjet.RecipientsV31{mailjet.RecipientV31{Email: recipient}},
		Subject:  strings.Title(kind) + " notification",
		TextPart: msg,
		HTMLPart: msg + " (<a href=\"" + monitorURL + "?sk=" + strconv.FormatInt(skey, 10) + "\">monitor</a>)",
	}}
	msgs := mailjet.MessagesV31{Info: msgInfo}
	_, err = mailClient.SendMailV31(&msgs)
	if err != nil {
		return fmt.Errorf("could not send mail: %w", err)
	}

	err = model.PutVariable(ctx, settingsStore, skey, "_"+kind+"."+recipient, "")
	if err != nil {
		return fmt.Errorf("could not put variable: %w", err)
	}
	return nil
}
