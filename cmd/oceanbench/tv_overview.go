package main

import (
	"errors"
	"log"

	"github.com/ausocean/cloud/gauth"
	"github.com/gofiber/fiber/v2"
)

// tvOverviewHandler handles request to the tv overview page. This page lets the user
// see an overview of different broadcasts which can be selected and setups saved. The
// user must be a superadmin to access this feature, and their personal configuration
// is saved in a variable scoped to their username (email before the host, stripped of
// any fullstops, user.name@ausocean.org -> username).
func tvOverviewHandler(c *fiber.Ctx) error {
	logRequest(c)

	p, err := getProfile(c)
	switch {
	case err != nil && !errors.Is(err, gauth.TokenNotFound):
		log.Printf("authentication error: %v", err)
		fallthrough
	case err != nil:
		return c.Redirect("/", fiber.StatusUnauthorized)
	}

	if !isSuperAdmin(p.Email) {
		return c.Redirect("/", fiber.StatusUnauthorized)
	}

	data := &commonData{
		// This page is not accessible from the nav-menu.
		Pages: pages(c, ""),
	}

	// Write the template with the minimum data, and load the
	// rest of the data asynchronously through the lit element.
	writeTemplate(c, "tv-overview.html", data, "")
	return nil
}
