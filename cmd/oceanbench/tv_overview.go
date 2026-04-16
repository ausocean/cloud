package main

import (
	"net/http"
)

// tvOverviewHandler handles request to the tv overview page. This page lets the user
// see an overview of different broadcasts which can be selected and setups saved. The
// user must be a superadmin to access this feature, and their personal configuration
// is saved in a variable scoped to their username (email before the host, stripped of
// any fullstops, user.name@ausocean.org -> username).
func tvOverviewHandler(w http.ResponseWriter, r *http.Request) {
	data := &commonData{
		// This page is not accessible from the nav-menu.
		Pages: pages(""),
	}

	// Write the template with the minimum data, and load the
	// rest of the data asynchronously through the lit element.
	writeTemplate(w, r, "tv-overview.html", data, "")
}
