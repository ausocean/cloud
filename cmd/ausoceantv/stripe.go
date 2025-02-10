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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/paymentintent"
	"github.com/stripe/stripe-go/v81/price"
	"github.com/stripe/stripe-go/v81/product"
	"github.com/stripe/stripe-go/v81/subscription"

	"github.com/ausocean/cloud/backend"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

// Errors from malformed API requests.
var (
	ErrNoProductSelected = errors.New("no product selected")
	errEmptyPriceID      = errors.New("price_id empty")
	errEmptyProductID    = errors.New("product_id empty")
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
	if svc.standalone || svc.development {
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

func (svc *service) handleStripeWebhook(c *fiber.Ctx) error {
	event := stripe.Event{}

	if err := json.Unmarshal(c.Body(), &event); err != nil {
		log.Errorf("Failed to parse webhook body json: %w", err.Error())
		return err
	}

	switch event.Type {
	case stripe.EventTypePaymentIntentSucceeded:
		var pi stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &pi)
		if err != nil {
			log.Errorf("Error parsing webhook JSON: %w", err)
			return err
		}
		return svc.handleSuccessfulPaymentIntent(pi)
	case stripe.EventTypeInvoicePaymentSucceeded:
		var invoice stripe.Invoice
		err := json.Unmarshal(event.Data.Raw, &invoice)
		if err != nil {
			log.Errorf("Error parsing webhook JSON: %w", err)
			return err
		}
		return svc.handleInvoicePaymentSuccess(invoice)
	default:
		return errors.New("unsupported event type")
	}
}

func (svc *service) handleSuccessfulPaymentIntent(pi stripe.PaymentIntent) error {
	ctx := context.Background()
	customer, err := svc.getCustomer(&model.Subscriber{PaymentInfo: pi.Customer.ID})
	if err != nil {
		return fmt.Errorf("error getting customer: %w", err)
	}
	sub, err := model.GetSubscriberByEmail(ctx, svc.store, customer.Email)
	if err != nil {
		log.Errorf("failed to get subscriber for email: %s, err: %v", pi.Customer.Email, err)
		return fmt.Errorf("failed to get subscriber for email: %s, err: %w", pi.Customer.Email, err)
	}

	if pi.Invoice != nil {
		// This payment intent is for a subscription for which we will handle the invoice_payment.succeeded event.
		return nil
	}

	// This is for a day pass.
	return model.CreateSubscription(ctx, svc.store, sub.ID, 0, "", false,
		model.WithSubscriptionClass(model.SubscriptionDay),
		model.WithStripePaymentIntentID(pi.ID),
	)
}

func (svc *service) handleInvoicePaymentSuccess(invoice stripe.Invoice) error {
	ctx := context.Background()
	customer, err := svc.getCustomer(&model.Subscriber{PaymentInfo: invoice.Customer.ID})
	if err != nil {
		return fmt.Errorf("error getting customer: %w", err)
	}
	subber, err := model.GetSubscriberByEmail(ctx, svc.store, customer.Email)
	if err != nil {
		return fmt.Errorf("failed to get subscriber for email: %s, err: %w", invoice.Customer.Email, err)
	}
	sub, err := subscription.Get(invoice.Subscription.ID, nil)
	if err != nil {
		return fmt.Errorf("error getting subscription from stripe: %w", err)
	}

	start := sub.CurrentPeriodStart
	end := sub.CurrentPeriodEnd
	renew := !sub.CancelAtPeriodEnd
	id := sub.ID

	curr, err := model.GetSubscription(ctx, svc.store, subber.ID, 0)
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		return model.CreateSubscription(ctx, svc.store, subber.ID, 0, "", renew,
			model.WithStartEnd(start, end),
			model.WithStripeSubscriptionID(id),
			model.WithStripePaymentIntentID(invoice.PaymentIntent.ID),
		)
	}
	if err != nil {
		return fmt.Errorf("unable to get current subscription: %w", err)
	}

	curr.Start = time.Unix(start, 0)
	curr.Finish = time.Unix(end, 0)
	return model.UpdateSubscription(ctx, svc.store, curr)
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

	subscriber, err := model.GetSubscriberByEmail(ctx, svc.store, p.Email)
	if err != nil {
		return fmt.Errorf("failed getting subscriber, try logging in again: %w", err)
	}

	customer, err := svc.getCustomer(subscriber)
	if err != nil {
		return fmt.Errorf("error getting customer ID: %w", err)
	}

	priceID := c.FormValue("priceID")
	if priceID == "" {
		err := c.SendStatus(fiber.StatusBadRequest)
		if err != nil {
			return fmt.Errorf("error sending status: %d, err: %w", fiber.StatusBadRequest, err)
		}
		return ErrNoProductSelected
	}

	price, err := getPrice(priceID)
	if err != nil {
		return fmt.Errorf("error getting price: %w", err)
	}

	// If the price is recurring the selected product is a subscription and needs
	// to be handled differently to the once off case.
	if price.Recurring == nil {
		return svc.createPaymentIntent(c, customer.ID, price)
	}
	return svc.createSubscriptionIntent(c, customer.ID, priceID)
}

type clientSecretResponse struct {
	ClientSecret string `json:"clientSecret"`
}

func (svc *service) createPaymentIntent(c *fiber.Ctx, cid string, price *stripe.Price) error {
	params := &stripe.PaymentIntentParams{
		Amount:                  stripe.Int64(price.UnitAmount),
		Currency:                (*string)(&price.Currency),
		Customer:                stripe.String(cid),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{Enabled: stripe.Bool(true)},
	}
	// Check for an incomplete payment intent. This could happen if a user restarts the checkout flow,
	// and Stripe recommends reusing payment intents to track the history of a purchase.
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
			return c.JSON(clientSecretResponse{pi.ClientSecret})
		}
	}

	// NOTE: DO NOT LOG PAYMENT INTENT.
	pi, err = paymentintent.New(params)
	if err != nil {
		log.Errorf("error creating new Stripe payment intent: %v", err)
		return fmt.Errorf("could not create payment intent: %w", err)
	}

	return c.JSON(clientSecretResponse{pi.ClientSecret})
}

func (svc *service) createSubscriptionIntent(c *fiber.Ctx, cid, pid string) error {
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

// getCustomer returns the stripe customer for the current user, if no customer exists, a
// new customer is created.
func (svc *service) getCustomer(sub *model.Subscriber) (*stripe.Customer, error) {
	if sub.PaymentInfo != "" {
		return customer.Get(sub.PaymentInfo, nil)
	}

	id, err := createCustomer(sub.GivenName, sub.FamilyName, sub.Email)
	if err != nil {
		return nil, fmt.Errorf("error creating new customer: %w", err)
	}

	sub.PaymentInfo = id
	err = model.UpdateSubscriber(context.Background(), svc.store, sub)
	if err != nil {
		return nil, fmt.Errorf("error updating subscriber with new payment info: %w", err)
	}

	return customer.Get(sub.PaymentInfo, nil)
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

func (svc *service) cancelSubscription(c *fiber.Ctx) error {
	ctx := context.Background()
	p, err := svc.auth.GetProfile(backend.NewFiberHandler(c))
	if errors.Is(err, gauth.SessionNotFound) || errors.Is(err, gauth.TokenNotFound) {
		return fiber.NewError(fiber.StatusUnauthorized, fmt.Sprintf("error getting profile: %v", err))
	} else if err != nil {
		return fmt.Errorf("unable to get profile: %w", err)
	}

	subscriber, err := model.GetSubscriberByEmail(ctx, svc.store, p.Email)
	if err != nil {
		return fmt.Errorf("error getting subscriber by email for: %s: %w", p.Email, err)
	}

	sub, err := model.GetSubscription(ctx, svc.store, subscriber.ID, 0)
	if err != nil {
		return fmt.Errorf("error getting subscription for id: %d: %w", subscriber.ID, err)
	}

	// Since the day pass is not a renewing subscription, we shouldn't need to cancel it with stripe.
	if sub.Class != model.SubscriptionDay {
		subParams := &stripe.SubscriptionParams{CancelAtPeriodEnd: stripe.Bool(true)}
		_, err = subscription.Update(sub.StripeSubscriptionID, subParams)
		if err != nil {
			return fmt.Errorf("failed to cancel subscription: %w", err)
		}
	}

	sub.Renew = false
	return model.UpdateSubscription(ctx, svc.store, sub)
}

func (svc *service) handleGetPrice(c *fiber.Ctx) error {
	pid := c.Params("id")
	if pid == "" {
		return errEmptyPriceID
	}
	log.Infof("getting price for '%s'", pid)

	price, err := getPrice(pid)
	if err != nil {
		return fmt.Errorf("error getting price: %w", err)
	}

	log.Infof("price: %+v", price)

	return c.JSON(price)
}

func (svc *service) handleGetProduct(c *fiber.Ctx) error {
	pid := c.Params("id")
	if pid == "" {
		return errEmptyProductID
	}
	log.Infof("getting product for '%s'", pid)

	params := &stripe.ProductParams{}
	product, err := product.Get(pid, params)
	if err != nil {
		return fmt.Errorf("error getting product for id: %s, err: %w", pid, err)
	}

	return c.JSON(product)
}
