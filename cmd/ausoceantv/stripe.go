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
	"errors"
	"fmt"

	"cloud.google.com/go/datastore"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/paymentintent"
	"github.com/stripe/stripe-go/v81/price"
	"github.com/stripe/stripe-go/v81/subscription"

	"github.com/ausocean/cloud/backend"
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
		key, err = gauth.GetSecret(ctx, projectID, "STRIPE_SECRET_KEY")
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
	// Check if a subscriber already exists.
	p, err := svc.auth.GetProfile(backend.NewFiberHandler(c))
	if errors.Is(err, gauth.SessionNotFound) || errors.Is(err, gauth.TokenNotFound) {
		return fiber.NewError(fiber.StatusUnauthorized, fmt.Sprintf("error getting profile: %v", err))
	} else if err != nil {
		return fmt.Errorf("unable to get profile: %w", err)
	}

	ctx := context.Background()

	subscriber, err := model.GetSubscriberByEmail(ctx, svc.settingsStore, p.Email)
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		subscriber = &model.Subscriber{GivenName: p.GivenName, FamilyName: p.FamilyName, Email: p.Email}
		err := model.CreateSubscriber(ctx, svc.settingsStore, subscriber)
		if err != nil {
			return fmt.Errorf("unable to create susbcriber %v: %w", subscriber, err)
		}
	} else if err != nil {
		return fmt.Errorf("failed getting subscriber by email: %w", err)
	}

	// Get the customer ID for the current user.
	customerID, err := svc.getCustomerID(subscriber)
	if err != nil {
		return fmt.Errorf("error getting customer ID: %w", err)
	}

	// Get the product price ID.
	priceID := c.FormValue("priceID")
	if priceID == "" {
		c.SendStatus(fiber.StatusBadRequest)
		return fmt.Errorf("no product selected")
	}

	price, err := getPrice(priceID)
	if err != nil {
		return fmt.Errorf("error getting price: %v", err)
	}

	// If the price is recurring the selected product is a subscription and needs
	// to be handled differently to the once off case.
	if price.Recurring == nil {
		return svc.createPaymentIntent(c, customerID, price)
	} else {
		return svc.createSubscriptionIntent(c, customerID, priceID)
	}
}

func (svc *service) createPaymentIntent(c *fiber.Ctx, cid string, price *stripe.Price) error {
	// Create a PaymentIntent with amount and currency.
	params := &stripe.PaymentIntentParams{
		Amount:                  stripe.Int64(price.UnitAmount),
		Currency:                (*string)(&price.Currency),
		Customer:                stripe.String(cid),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{Enabled: stripe.Bool(true)},
	}

	// Check for an existing payment intent that is not complete.
	var err error
	pi := getActivePaymentIntent(cid)
	if pi != nil {
		// Update the existing payment intent.
		// Since customer and auto payment methods cannot be updated, they should be removed from the params.
		params.Customer = nil
		params.AutomaticPaymentMethods = nil
		pi, err = paymentintent.Update(pi.ID, params)
		if err != nil {
			log.Errorf("error updating payment intent, will retry with new payment intent: %v", err)
		} else {
			v := struct {
				ClientSecret string `json:"clientSecret"`
			}{
				ClientSecret: pi.ClientSecret,
			}
			return c.JSON(v)
		}
	}

	// NOTE: DO NOT LOG PAYMENT INTENT.
	pi, err = paymentintent.New(params)
	if err != nil {
		log.Errorf("Error creating new Stripe payment intent: %v", err)
		return c.App().ErrorHandler(c, fmt.Errorf("could not create payment intent: %w", err))
	}

	v := struct {
		ClientSecret string `json:"clientSecret"`
	}{
		ClientSecret: pi.ClientSecret,
	}

	return c.JSON(v)
}

func (svc *service) createSubscriptionIntent(c *fiber.Ctx, cid, pid string) error {
	// Set the subscription parameters.
	paymentSettings := &stripe.SubscriptionPaymentSettingsParams{
		SaveDefaultPaymentMethod: stripe.String("on_subscription"),
	}
	subParams := &stripe.SubscriptionParams{
		Customer: stripe.String(cid),
		Items: []*stripe.SubscriptionItemsParams{{
			Price: stripe.String(pid),
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

// getActivePaymentIntent returns a payment intent for the user which does not have the
// state succeeded or cancelled.
func getActivePaymentIntent(cid string) *stripe.PaymentIntent {
	// Get a payment intent.
	intents := paymentintent.List(&stripe.PaymentIntentListParams{
		Customer:   stripe.String(cid),
		ListParams: stripe.ListParams{Limit: stripe.Int64(1)},
	})

	// Move the list to point to the returned payment intent.
	if !intents.Next() {
		return nil
	}

	pi := intents.PaymentIntent()
	if pi.Status == stripe.PaymentIntentStatusSucceeded || pi.Status == stripe.PaymentIntentStatusCanceled {
		return nil
	}

	return pi
}

// getCustomer returns the customer ID for the current user, if no customer exists, a
// new customer is created.
func (svc *service) getCustomerID(sub *model.Subscriber) (string, error) {
	if sub.PaymentInfo != "" {
		return sub.PaymentInfo, nil
	}

	id, err := createCustomer(sub.GivenName, sub.FamilyName, sub.Email)
	if err != nil {
		return "", fmt.Errorf("error creating new customer: %w", err)
	}

	sub.PaymentInfo = id
	err = model.UpdateSubscriber(context.Background(), svc.settingsStore, sub)
	if err != nil {
		return "", fmt.Errorf("error updating subscriber with new payment info: %w", err)
	}

	return sub.PaymentInfo, nil
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

// getPrice is a helper function to get the stripe price struct for a given
// price ID (pid).
func getPrice(pid string) (*stripe.Price, error) {
	params := &stripe.PriceParams{}
	return price.Get(pid, params)
}
