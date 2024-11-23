package main

import (
	"context"
	"fmt"

	"github.com/ausocean/cloud/gauth"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/paymentintent"
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
func (app *service) handleCreatePaymentIntent(c *fiber.Ctx) error {
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
