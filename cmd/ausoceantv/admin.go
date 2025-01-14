package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ausocean/cloud/model"
	"github.com/gofiber/fiber/v2"
)

var (
	errNoName  = errors.New("no name")
	errNoArea  = errors.New("no area")
	errNoClass = errors.New("no class")
)

// adminFeedNewHandler updates a feed from the query form.
func (svc *service) adminFeedUpdateHandler(c *fiber.Ctx) error {
	feed, err := getFeedFromForm(c, false)
	if err != nil {
		return fmt.Errorf("unable to get feed from form: %w", err)
	}

	ctx := context.Background()
	err = model.PutFeed(ctx, svc.settingsStore, feed)
	if err != nil {
		return fmt.Errorf("unable to put feed: %w", err)
	}

	return c.JSON(feed)
}

// adminFeedNewHandler creates a new feed from the query form.
func (svc *service) adminFeedNewHandler(c *fiber.Ctx) error {
	feed, err := getFeedFromForm(c, true)
	if err != nil {
		return fmt.Errorf("unable to get feed from form: %w", err)
	}

	ctx := context.Background()
	feed, err = model.CreateFeed(ctx, svc.settingsStore, feed)
	if err != nil {
		return fmt.Errorf("unable to create feed: %w", err)
	}

	return c.JSON(feed)
}

// getFeedFromForm parses a feed entity from the query form.
func getFeedFromForm(c *fiber.Ctx, isNew bool) (*model.Feed, error) {
	sid := c.FormValue("id")
	id, err := strconv.Atoi(sid)
	if err != nil && !isNew {
		return nil, fmt.Errorf("failed to parse id as int: %w", err)
	}
	name := c.FormValue("name")
	if name == "" {
		return nil, errNoName
	}
	area := c.FormValue("area")
	if name == "" {
		return nil, errNoArea
	}
	class := c.FormValue("class")
	if name == "" {
		return nil, errNoClass
	}
	bundle := c.FormValue("bundle")

	return &model.Feed{ID: int64(id), Name: name, Area: area, Class: class, Bundle: strings.Split(bundle, ",")}, nil
}
