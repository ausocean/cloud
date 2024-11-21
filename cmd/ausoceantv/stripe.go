package main

import (
	"context"
	"log"
	"net/http"

	"github.com/ausocean/cloud/gauth"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/paymentintent"
)

// setupStripe gets the secrets required to set the stripe Key.
// The secrets required are DEV_STRIPE_SECRET_KEY for standalone mode,
// and STRIPE_SECRET_KEY for appengine mode.
//
// NOTE: If stripe keys aren't found, this causes a fatal error.
func setupStripe(ctx context.Context) {
	var (
		key string
		err error
	)

	// In standalone mode we want to use developer test keys.
	if app.standalone {
		key, err = gauth.GetSecret(ctx, projectID, "DEV_STRIPE_SECRET_KEY")
	} else {
		// NOTE: This will be linked to production keys, and not test keys.
		// Warn the user.
		log.Println(`
			***************
			*** WARNING ***
			***************

			Using production Stripe keys
		`)
		key, err = gauth.GetSecret(ctx, projectID, "STRIPE_SECRET_KEY")
	}

	if err != nil {
		log.Fatalln("unable to get stripe secret key, payments will not work:", err)
		return
	}

	// Set the global stripe key.
	stripe.Key = key

	log.Println("set up stripe")
}

// handleCreatePaymentIntent handles requests to /stripe/create-payment-intent.
func handleCreatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	app.logRequest(r)

	if r.Method == http.MethodOptions {
		w.WriteHeader(200)
		return
	} else if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// TODO: Get product details.
	//	description := product.description
	//	price := calculatePrice(product)

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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error creating new Stripe payment intent: %v", err)
		return
	}

	writeJSON(w, struct {
		ClientSecret string `json:"clientSecret"`
		// DpmCheckerLink string `json:"dpmCheckerLink"` <-- Can be used for debugging of the integration.
	}{
		ClientSecret: pi.ClientSecret,
		// [DEV]: For demo purposes only, you should avoid exposing the PaymentIntent ID in the client-side code.
		// DpmCheckerLink: fmt.Sprintf("https://dashboard.stripe.com/settings/payment_methods/review?transaction_id=%s", pi.ID),
	})
}
