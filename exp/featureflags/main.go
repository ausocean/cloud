// Experimental implementation of feature flags utilizing redirects for each feature.

package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// See the Design Note for a full description of the following types.
// Subscriber represents a user with associated feature flags.
type Subscriber struct {
	ID    string // Unique identifier for the subscriber
	Flags string // Comma-separated list of active flags, e.g., "featureA,featureB"
}

// FeatureRedirect maps a flag to a target URL for redirection.
type FeatureRedirect struct {
	Flag      string `json:"flag"`      // The name of the feature flag.
	TargetURL string `json:"targetURL"` // The URL to redirect to if the flag is active.
	Active    bool   `json:"active"`    // Whether this redirect is currently active.
}

// Feature provides a human-readable description of a feature.
type Feature struct {
	Flag        string    `json:"flag"`        // The name of the feature flag.
	Description string    `json:"description"` // Human-readable description.
	Active      bool      `json:"active"`      // Whether the feature is generally active.
	Deployed    time.Time `json:"deployed"`    // When the feature was deployed.
	Disabled    time.Time `json:"disabled"`    // When the feature was disabled (if applicable).
}

const (
	standardHomePage = "https://nobletech.com"
)

var (
	subscribers      = make(map[string]Subscriber)
	featureRedirects = make(map[string]FeatureRedirect)
	features         = make(map[string]Feature)
)

// Mock datastore.

// initMockData initializes some sample data for demonstration.
func initMockData() {
	// Sample Subscribers
	subscribers["alan"] = Subscriber{ID: "alan", Flags: "featureA"}
	subscribers["trek"] = Subscriber{ID: "trek", Flags: "featureB"}
	subscribers["cath"] = Subscriber{ID: "cath", Flags: "featureC"}
	subscribers["david"] = Subscriber{ID: "david", Flags: ""}

	// Sample FeatureRedirects
	featureRedirects["featureA"] = FeatureRedirect{
		Flag:      "featureA",
		TargetURL: "https://a.nobletech.com",
		Active:    true,
	}
	featureRedirects["featureB"] = FeatureRedirect{
		Flag:      "featureB",
		TargetURL: "https://b.nobletech.com",
		Active:    true,
	}
	featureRedirects["featureC"] = FeatureRedirect{
		Flag:      "featureC",
		TargetURL: "https://c.nobletech.com",
		Active:    false, // Inactive redirect
	}

	// Sample Features
	features["featureA"] = Feature{
		Flag:        "featureA",
		Description: "Spiffy feature A",
		Active:      true,
		Deployed:    time.Now(),
	}
	features["featureB"] = Feature{
		Flag:        "featureB",
		Description: "Spiffy feature B",
		Active:      true,
		Deployed:    time.Now(),
	}
	features["featureC"] = Feature{
		Flag:        "featureC",
		Description: "Spiffy feature C - inactive",
		Active:      false,
		Deployed:    time.Now().Add(-72 * time.Hour),
	}
}

// getSubscriber simulates fetching a subscriber from a database.
func getSubscriber(subscriberID string) (Subscriber, bool) {
	sub, ok := subscribers[subscriberID]
	return sub, ok
}

// getFeatureRedirect simulates fetching a feature redirect from a database.
func getFeatureRedirect(flag string) (FeatureRedirect, bool) {
	fr, ok := featureRedirects[flag]
	return fr, ok
}

// FeatureFlagMiddleware is our HTTP middleware that handles feature-based redirection.
// It invokes the next http.Handler in the chain unless an active feature is found,
// in which case it redirects accordingly.
func FeatureFlagMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In AusOcean TV, the subscriber ID would come from an authenticated user.
		// Here we'll use a query parameter and mock lookup.
		subscriberID := r.URL.Query().Get("id")
		if subscriberID == "" {
			// If no subscriber ID, proceed to the next handler (e.g., standard home page)
			log.Println("No subscriber ID found in request. Proceeding to default handler.")
			next.ServeHTTP(w, r)
			return
		}

		subscriber, ok := getSubscriber(subscriberID)
		if !ok {
			log.Printf("Subscriber '%s' not found. Proceeding to default handler.", subscriberID)
			next.ServeHTTP(w, r)
			return
		}

		// Parse the subscriber's flags
		subscriberFlags := strings.Split(subscriber.Flags, ",")
		log.Printf("Subscriber '%s' has flags: %v", subscriber.ID, subscriberFlags)

		// Check for active feature redirects
		for _, flag := range subscriberFlags {
			flag = strings.TrimSpace(flag)
			if flag == "" {
				continue
			}

			fr, found := getFeatureRedirect(flag)
			if found && fr.Active {
				log.Printf("Redirecting subscriber '%s' (flag '%s') to %s", subscriber.ID, flag, fr.TargetURL)
				http.Redirect(w, r, fr.TargetURL, http.StatusFound)
				return

			} else if found && !fr.Active {
				log.Printf("FeatureRedirect for flag '%s' found but is inactive. Not redirecting.", flag)

			} else {
				log.Printf("No FeatureRedirect found for flag '%s'.", flag)
			}
		}

		// If no active feature redirect is found, proceed to the standard handler.
		log.Printf("No active feature redirect for subscriber '%s'. Proceeding to standard handler.", subscriber.ID)
		next.ServeHTTP(w, r)
	})
}

// homeHandler is the standard home page handler.
func homeHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, standardHomePage, http.StatusFound)
}

func main() {
	initMockData()

	// Define the default home page handler
	standardHomeHandler := http.HandlerFunc(homeHandler)

	// Apply the FeatureFlagMiddleware.
	// All requests to "/" will first go through the middleware.
	http.Handle("/", FeatureFlagMiddleware(standardHomeHandler))

	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

/*
To test:

    * Standard Home Page (no subscriber ID):
        http://localhost:8080/
        Should not redirect, i.e., serve the standard home page https://nobletech.com.

    * Subscriber with 'featureA' flag (should redirect):
        http://localhost:8080/?id=alan
        Should redirect to https://a.nobletech.com.

    * Subscriber with 'featureB' flag (should redirect):
        http://localhost:8080/?id=trek
        Should redirect to https://b.nobletech.com.

    * Subscriber with no flags:
        http://localhost:8080/?id=cath
        Should not redirect since featureC is not active.

    * Subscriber with no flags:
        http://localhost:8080/?id=david
        Should not redirect since no feature flags for this subscriber.
*/
