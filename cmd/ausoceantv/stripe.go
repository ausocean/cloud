/*
AUTHORS
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This file is part of AusOcean TV. AusOcean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  AusOcean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with AusOcean TV in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/subscription"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
)

// setupStripe gets the secrets required to set the stripe Key.
// The secrets required are DEV_STRIPE_SECRET_KEY for standalone mode,
// and STRIPE_SECRET_KEY for appengine mode.
//
// NOTE: If stripe keys aren't found, this causes a fatal error.
func (svc *service) setupStripe(ctx context.Context) {
	var (
		key string
		err error
	)

	// In standalone mode we want to use developer test keys.
	if svc.standalone {
		key, err = gauth.GetSecret(ctx, projectID, "DEV_STRIPE_SECRET_KEY")
	} else {
		// NOTE: This will be linked to production keys, and not test keys.
		// Warn the user.
		log.Warn(`
			***************
			*** WARNING ***
			***************

			Using production Stripe keys
		`)
		key, err = gauth.GetSecret(ctx, projectID, "DEV_STRIPE_SECRET_KEY")
	}

	if err != nil {
		log.Fatal("unable to get stripe secret key, payments will not work:", err)
		return
	}

	// Set the global stripe key.
	stripe.Key = key

	log.Info("setup stripe")
}

// handleCreatePaymentIntent handles requests to /stripe/create-payment-intent.
func (svc *service) handleCreatePaymentIntent(c *fiber.Ctx) error {
	// Get priceID from query parameters.
	priceID := c.FormValue("priceID")
	if priceID == "" {
		return fmt.Errorf("no product selected")
	}

	// Get the customer ID for the current user.
	customerID, err := svc.getCustomerID(c)
	if err != nil {
		return fmt.Errorf("error getting customer ID: %w", err)
	}

	// Set the subscription parameters.
	paymentSettings := &stripe.SubscriptionPaymentSettingsParams{
		SaveDefaultPaymentMethod: stripe.String("on_subscription"),
	}
	subParams := &stripe.SubscriptionParams{
		Customer: stripe.String(customerID),
		Items: []*stripe.SubscriptionItemsParams{{
			Price: stripe.String("price_1QRoJWDMhXzQAv2T7Raa9QcV"),
		}},
		PaymentSettings: paymentSettings,
		PaymentBehavior: stripe.String("default_incomplete"),
	}
	subParams.AddExpand("latest_invoice.payment_intent")
	s, err := subscription.New(subParams)
	if err != nil {
		log.Errorf("subscription.New: %v", err)
		return fmt.Errorf("unable to create new stripe subscription: %w", err)
	}

	return c.JSON(
		struct {
			SubscriptionID string `json:"subscriptionId"`
			ClientSecret   string `json:"clientSecret"`
		}{
			SubscriptionID: s.ID,
			ClientSecret:   s.LatestInvoice.PaymentIntent.ClientSecret,
		},
	)
}

// createCustomer creates a new Stripe customer object, returning the ID of
// the newly created customer.
func createCustomer(givenName, familyName, email string) (string, error) {
	// Set the parameters.
	params := &stripe.CustomerParams{
		Name:  stripe.String(fmt.Sprintf("%s %s", givenName, familyName)),
		Email: stripe.String(email),
	}

	// Create a new customer.
	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("error creating new stripe customer: %w", err)
	}

	return cust.ID, nil
}

// getCustomer returns the customer ID for the current user.
func (svc *service) getCustomerID(c *fiber.Ctx) (string, error) {
	p, err := svc.GetProfile(c)
	if err != nil {
		return "", fmt.Errorf("error getting profile: %w", err)
	}

	sub, err := model.GetSubscriberByEmail(context.Background(), svc.settingsStore, p.Email)
	if err != nil {
		return "", fmt.Errorf("error getting subscriber for email: %s, err: %w", p.Email, err)
	}

	if sub.PaymentInfo == "" {
		return "", fmt.Errorf("no customerID attached to subscriber entity")
	}

	return sub.PaymentInfo, nil
}
