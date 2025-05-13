/*
AUTHORS

	Trek Hopton <trek@ausocean.org>

LICENSE

	Copyright (C) 2025 the Australian Ocean Lab (AusOcean).

	This is free software: you can redistribute it and/or modify it
	under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This is distributed in the hope that it will be useful, but WITHOUT
	ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
	or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
	License for more details.

	You should have received a copy of the GNU General Public License in
	gpl.txt. If not, see http://www.gnu.org/licenses/.
*/
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ausocean/cloud/backend"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/openfish/cmd/openfish/api"
	"github.com/ausocean/openfish/datastore"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/encryptcookie"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Project constants.
const (
	projectID     = "ausocean-roadmap"
	oauthClientID = "1034725146926-srirvgd3j0gd20n45luju68q2vaago7c.apps.googleusercontent.com"
	oauthMaxAge   = 60 * 60 * 24 * 7 // 7 days.
	version       = "v0.0.1"
)

// service defines the properties of our web service.
type service struct {
	setupMutex  sync.Mutex
	store       datastore.Store
	debug       bool
	standalone  bool
	development bool
	lite        bool
	storePath   string
	auth        *gauth.UserAuth
	frontendURL string
}

// svc is an instance of our service.
var svc *service = &service{}

func registerAPIRoutes(app *fiber.App) {
	v1 := app.Group("/api/v1")

	// Authentication Routes.
	v1.Group("/auth").
		Get("/login", svc.loginHandler).
		Get("/logout", svc.logoutHandler).
		Get("oauth2callback", svc.callbackHandler).
		Get("profile", svc.profileHandler)

	v1.Post("/update", updateHandler)

	v1.Get("/timeline", timelineHandler)
}

func main() {
	defaultPort := 8080
	v := os.Getenv("PORT")
	if v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			defaultPort = i
		}
	}

	var port int
	flag.IntVar(&port, "port", defaultPort, "Port we listen on in standalone mode")

	svc.frontendURL = os.Getenv("FRONTEND_URL")
	if svc.frontendURL == "" {
		svc.frontendURL = "http://localhost:5173"
	}

	// Create app.
	app := fiber.New(fiber.Config{ErrorHandler: api.ErrorHandler, ReadBufferSize: 8192})

	ctx := context.Background()
	key, err := gauth.GetSecret(ctx, projectID, "sessionKey")
	if err != nil {
		log.Panicf("unable to get sessionKey secret: %v", err)
	}

	svc.setup()

	app.Use(encryptcookie.New(encryptcookie.Config{
		Key: key,
	}))

	registerAPIRoutes(app)

	// Start web server.
	listenOn := fmt.Sprintf(":%d", port)
	fmt.Printf("starting web server on %s\n", listenOn)
	log.Fatal(app.Listen(listenOn))
}

// setup executes per-instance one-time warmup and is used to
// initialize the service. Any errors are considered fatal.
//
// NOTE: This function must be called before any middleware which uses
// cookies is attached to the app.
func (svc *service) setup() {
	svc.setupMutex.Lock()
	defer svc.setupMutex.Unlock()

	// Initialise OAuth2.
	svc.auth = &gauth.UserAuth{ProjectID: projectID, ClientID: oauthClientID, MaxAge: oauthMaxAge}
	svc.auth.Init(backend.NewFiberHandler(nil))
}

// Spreadsheet details
const (
	spreadsheetID = "1nWk8oX4qBApaPcvBmz4VeTieU2Q2cK_qD71nW8LHAsc" // Replace with your actual spreadsheet ID
	readRange     = "Sheet1!A2:M"                                  // Adjust based on your sheet layout
)

// AuthGSheetsRead is a wrapper for the google oauth2 credentials from JSON method.
// It returns a readonly google credential.
func AuthGSheetsRead(ctx context.Context) (*google.Credentials, error) {
	return google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/spreadsheets.readonly")
}

func AuthGSheets(ctx context.Context) (*google.Credentials, error) {
	return google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/spreadsheets")
}

// ReadSpreadsheet returns the range of values specified in A1 or R1C1 notation
func ReadSpreadsheet(ctx context.Context, credentials *google.Credentials, spreadsheetID, readRange string) ([][]interface{}, error) {
	// Create a new Sheets API service using the provided credentials.
	srv, err := sheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		return nil, fmt.Errorf("couldn't create new service: %w", err)
	}

	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to read from sheet: %w", err)
	}

	if len(resp.Values) == 0 {
		return nil, errors.New("no data found")
	}

	return resp.Values, nil
}

// Convert spreadsheet data into structured JSON
func parseRoadmapData(data [][]interface{}) []map[string]string {
	headers := []string{"ID", "Category", "Title", "Description", "Priority", "Owner", "Status", "Start", "End", "Milestone Type", "Dependencies", "Actual Start", "Actual End"}
	var tasks []map[string]string

	for _, row := range data {
		entry := make(map[string]string)
		for i, col := range row {
			if i < len(headers) {
				entry[headers[i]] = fmt.Sprintf("%v", col)
			}
		}
		tasks = append(tasks, entry)
	}

	return tasks
}

// API handler for serving roadmap data
func timelineHandler(c *fiber.Ctx) error {
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if c.Method() == fiber.MethodOptions {
		return c.SendStatus(fiber.StatusOK)
	}

	ctx := context.Background()

	// Authenticate with Google Sheets API
	credentials, err := AuthGSheetsRead(ctx)
	if err != nil {
		log.Printf("Failed to authenticate with Google Sheets: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to authenticate with Google Sheets")
	}

	// Fetch data from Google Sheets
	data, err := ReadSpreadsheet(ctx, credentials, spreadsheetID, readRange)
	if err != nil {
		log.Printf("Error fetching data: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to fetch data: %v", err))
	}

	tasks := parseRoadmapData(data)

	return c.JSON(tasks)
}

type TaskUpdate struct {
	ID    string `json:"id"`
	Start string `json:"start"`
	End   string `json:"end"`
}

func updateHandler(c *fiber.Ctx) error {
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if c.Method() == fiber.MethodOptions {
		return c.SendStatus(fiber.StatusOK)
	}

	var payload struct {
		Tasks []TaskUpdate `json:"tasks"`
	}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("Invalid request: %v", err))
	}

	if len(payload.Tasks) == 0 {
		return c.Status(fiber.StatusBadRequest).SendString("No tasks to update")
	}

	ctx := context.Background()
	credentials, err := AuthGSheets(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Auth failed: %v", err))
	}

	srv, err := sheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to create Sheets service: %v", err))
	}

	rowMap, err := getTaskRowMap(srv)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to fetch task list: %v", err))
	}

	var updates []*sheets.ValueRange
	for _, task := range payload.Tasks {
		rowIndex, exists := rowMap[task.ID]
		if !exists {
			continue
		}

		formattedStart := formatDateToSheet(task.Start)
		formattedEnd := formatDateToSheet(task.End)

		updateRange := fmt.Sprintf("Sheet1!H%d:I%d", rowIndex, rowIndex)
		values := [][]interface{}{{formattedStart, formattedEnd}}

		updates = append(updates, &sheets.ValueRange{
			Range:          updateRange,
			Values:         values,
			MajorDimension: "ROWS",
		})
	}

	err = batchUpdateGoogleSheet(srv, updates)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Batch update failed: %v", err))
	}

	return c.SendString("✅ Tasks updated successfully")
}

// Convert YYYY-MM-DD to DD/MM/YYYY
func formatDateToSheet(date string) string {
	parsedTime, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date // If parsing fails, return the original date
	}
	return parsedTime.Format("02/01/2006")
}

func getTaskRowMap(srv *sheets.Service) (map[string]int, error) {
	rangeToRead := "Sheet1!A2:A" // Read Task ID column

	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, rangeToRead).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch task list: %v", err)
	}

	rowMap := make(map[string]int)
	for i, row := range resp.Values {
		if len(row) > 0 {
			taskID := fmt.Sprintf("%v", row[0])
			rowMap[taskID] = i + 2 // +2 because A2 is the first data row
		}
	}

	return rowMap, nil
}

func batchUpdateGoogleSheet(srv *sheets.Service, updates []*sheets.ValueRange) error {
	if len(updates) == 0 {
		return nil // No updates to send
	}

	batchUpdateRequest := &sheets.BatchUpdateValuesRequest{
		ValueInputOption: "RAW",
		Data:             updates,
	}

	_, err := srv.Spreadsheets.Values.BatchUpdate(spreadsheetID, batchUpdateRequest).Do()
	if err != nil {
		return fmt.Errorf("failed to batch update: %v", err)
	}

	return nil
}
