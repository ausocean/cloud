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
	"github.com/stripe/stripe-go/v81/paymentintent"

	"github.com/ausocean/cloud/gauth"
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
	// TODO: Get product details.
	// 	description := product.description
	// 	price := calculatePrice(product)

	// Enable auto payment method for better conversions.
	autoPaymentMethodEnabled := true

	// Create a PaymentIntent with amount and currency.
	params := &stripe.PaymentIntentParams{
		Amount:                  stripe.Int64(1099),
		Currency:                stripe.String(string(stripe.CurrencyAUD)),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{Enabled: &autoPaymentMethodEnabled},
	}

	// NOTE: DO NOT LOG PAYMENT INTENT.
	pi, err := paymentintent.New(params)
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
