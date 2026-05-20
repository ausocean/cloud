/*
AUTHORS

	Trek Hopton <trek@ausocean.org>

LICENSE

	Copyright (C) 2026 the Australian Ocean Lab (AusOcean).

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
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
)

//go:embed roadmap.config.json
var rawConfig []byte

// Config is the per-user roadmap timeline configuration embedded from
// roadmap.config.json (Google Sheet wiring and column layout). OAuth,
// branding, and other operator-owned settings live in main.go, not here.
type Config struct {
	Spreadsheet struct {
		ID              string   `json:"id"`
		SheetName       string   `json:"sheetName"`
		FirstDataRow    int      `json:"firstDataRow"`
		IDColumn        string   `json:"idColumn"`
		StartDateColumn string   `json:"startDateColumn"`
		EndDateColumn   string   `json:"endDateColumn"`
		Headers         []string `json:"headers"`
	} `json:"spreadsheet"`

	Tasks struct {
		FilterOutStatuses []string `json:"filterOutStatuses"`
		Defaults          struct {
			Category string `json:"category"`
			Owner    string `json:"owner"`
			Priority string `json:"priority"`
		} `json:"defaults"`
	} `json:"tasks"`
}

// ReadRange returns the A1-notation range that covers all data rows for
// every header column, e.g. "Sheet1!A2:M".
func (c *Config) ReadRange() string {
	lastCol := columnLetter(len(c.Spreadsheet.Headers))
	return fmt.Sprintf("%s!%s%d:%s", c.Spreadsheet.SheetName, c.Spreadsheet.IDColumn, c.Spreadsheet.FirstDataRow, lastCol)
}

// IDRange returns the A1-notation range that covers the task ID column,
// e.g. "Sheet1!A2:A".
func (c *Config) IDRange() string {
	return fmt.Sprintf("%s!%s%d:%s", c.Spreadsheet.SheetName, c.Spreadsheet.IDColumn, c.Spreadsheet.FirstDataRow, c.Spreadsheet.IDColumn)
}

// UpdateRange returns the A1-notation range covering the start- and
// end-date cells on the given row.
func (c *Config) UpdateRange(row int) string {
	return fmt.Sprintf("%s!%s%d:%s%d", c.Spreadsheet.SheetName, c.Spreadsheet.StartDateColumn, row, c.Spreadsheet.EndDateColumn, row)
}

// columnLetter converts a 1-based column index to its spreadsheet letter
// (1 -> A, 26 -> Z, 27 -> AA, ...).
func columnLetter(n int) string {
	if n <= 0 {
		return ""
	}
	out := ""
	for n > 0 {
		n--
		out = string(rune('A'+n%26)) + out
		n /= 26
	}
	return out
}

// cfg is the singleton parsed configuration. It is populated once by
// loadConfig and then read-only.
var cfg Config

// loadConfig parses the embedded roadmap.config.json. Any error is fatal
// because the application cannot run without configuration.
func loadConfig() {
	if err := json.Unmarshal(rawConfig, &cfg); err != nil {
		log.Fatalf("invalid roadmap.config.json: %v", err)
	}
	if cfg.Spreadsheet.SheetName == "" || len(cfg.Spreadsheet.Headers) == 0 {
		log.Fatalf("roadmap.config.json: spreadsheet.sheetName and spreadsheet.headers are required")
	}
}
