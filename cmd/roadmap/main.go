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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Spreadsheet details
const (
	spreadsheetID = "1nWk8oX4qBApaPcvBmz4VeTieU2Q2cK_qD71nW8LHAsc" // Replace with your actual spreadsheet ID
	readRange     = "Sheet1!A2:L"                                  // Adjust based on your sheet layout
)

// AuthGSheetsRead is a wrapper for the google oauth2 credentials from JSON method.
// It returns a readonly google credential.
func AuthGSheetsRead(ctx context.Context) (*google.Credentials, error) {
	return google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/spreadsheets.readonly")
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
	headers := []string{"ID", "Category", "Title", "Description", "Priority", "Owner", "Status", "Start", "End", "Dependants", "Actual Start", "Actual End"}
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
func timelineHandler(w http.ResponseWriter, r *http.Request) {
	// Allow cross-origin requests (CORS)
	w.Header().Set("Access-Control-Allow-Origin", "*") // Allow all origins
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := context.Background()

	// Authenticate with Google Sheets API
	credentials, err := AuthGSheetsRead(ctx)
	if err != nil {
		log.Printf("Failed to authenticate with Google Sheets: %v", err)
		http.Error(w, "Failed to authenticate with Google Sheets", http.StatusInternalServerError)
		return
	}

	// Fetch data from Google Sheets
	data, err := ReadSpreadsheet(ctx, credentials, spreadsheetID, readRange)
	if err != nil {
		log.Printf("Error fetching data: %v", err)
		http.Error(w, fmt.Sprintf("Failed to fetch data: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert data to JSON format
	tasks := parseRoadmapData(data)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func main() {
	http.HandleFunc("/timeline", timelineHandler)
	fmt.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
