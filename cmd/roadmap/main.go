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
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

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
func timelineHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
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

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*") // Allow all origins (change as needed)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

type TaskUpdate struct {
	ID    string `json:"id"`
	Start string `json:"start"`
	End   string `json:"end"`
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var payload struct {
		Tasks []TaskUpdate `json:"tasks"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if len(payload.Tasks) == 0 {
		http.Error(w, "No tasks to update", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	credentials, err := AuthGSheets(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Auth failed: %v", err), http.StatusInternalServerError)
		return
	}

	srv, err := sheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create Sheets service: %v", err), http.StatusInternalServerError)
		return
	}

	// Fetch all task IDs in one request
	rowMap, err := getTaskRowMap(srv)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch task list: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare batch update data
	var updates []*sheets.ValueRange
	for _, task := range payload.Tasks {
		rowIndex, exists := rowMap[task.ID]
		if !exists {
			continue // Skip tasks that aren't in the sheet
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

	// Batch update all modified tasks
	err = batchUpdateGoogleSheet(srv, updates)
	if err != nil {
		http.Error(w, fmt.Sprintf("Batch update failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "✅ Tasks updated successfully")
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

func main() {
	http.HandleFunc("/timeline", timelineHandler)
	http.HandleFunc("/update", updateHandler)
	fmt.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
