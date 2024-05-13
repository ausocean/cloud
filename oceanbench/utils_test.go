/*
DESCRIPTION
  utils_test.go provides unit tests utilites in utils.go.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2021-2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Bench. Ocean Bench is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean Bench is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"testing"
	"time"
)

func TestCronSpecToTime(t *testing.T) {
	testCases := []struct {
		input string
		want  string
		isErr bool
	}{
		{"0 0 * * *", "00:00", false},
		{"30 2 * * *", "02:30", false},
		{"0 12 * * *", "12:00", false},
		{"0 23 * * 0", "23:00", false},
		{"15 10 25 12 *", "10:15", false},
		{"0 0 * * 3-6", "00:00", false},
		{"0 0 * *", "", true},
		{"60 23 * * *", "", true},
		{"30 2 * * * *", "", true},
		{"0 24 * * *", "", true},
	}

	for _, tc := range testCases {
		got, err := cronSpecToTime(tc.input)

		if err == nil && tc.isErr {
			t.Errorf("CronSpecToTime(%q) expected an error but got none", tc.input)
		}

		if err != nil && !tc.isErr {
			t.Errorf("CronSpecToTime(%q) returned an error: %v", tc.input, err)
		}

		if got != tc.want {
			t.Errorf("CronSpecToTime(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIsTimeStr(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "Valid time string 1",
			input: "10:30",
			want:  true,
		},
		{
			name:  "Valid time string 2",
			input: "01:30",
			want:  true,
		},
		{
			name:  "Valid time string 3",
			input: "11:00",
			want:  true,
		},
		{
			name:  "Invalid format 1",
			input: "10-30",
			want:  false,
		},
		{
			name:  "Invalid format 2",
			input: "10:30:01",
			want:  false,
		},
		{
			name:  "Invalid hour value",
			input: "24:30",
			want:  false,
		},
		{
			name:  "Invalid minute value",
			input: "10:60",
			want:  false,
		},
		{
			name:  "Invalid hour and minute value",
			input: "24:60",
			want:  false,
		},
		{
			name:  "Negative hour",
			input: "-03:60",
			want:  false,
		},
		{
			name:  "Negative minute",
			input: "03:-45",
			want:  false,
		},
		{
			name:  "Empty input",
			input: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTimeStr(tt.input)
			if got != tt.want {
				t.Errorf("isTimeStr(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsCronSpec(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "Valid cron spec",
			input: "0 * * * *",
			want:  true,
		},
		{
			name:  "Invalid cron spec - too many fields",
			input: "0 * * * * *",
			want:  false,
		},
		{
			name:  "Invalid cron spec - too few fields",
			input: "0 * * *",
			want:  false,
		},
		{
			name:  "Invalid cron spec - invalid field value",
			input: "0 * * * 24",
			want:  false,
		},
		{
			name:  "Invalid cron spec - invalid field syntax",
			input: "0 * * * /",
			want:  false,
		},
		{
			name:  "Empty input",
			input: "",
			want:  false,
		},
		{
			name:  "Invalid minute field - out of range",
			input: "60 * * * *",
			want:  false,
		},
		{
			name:  "Invalid hour field - out of range",
			input: "0 24 * * *",
			want:  false,
		},
		{
			name:  "Invalid day of month field - out of range",
			input: "0 * 0 * *",
			want:  false,
		},
		{
			name:  "Invalid day of week field - out of range",
			input: "0 * * * 8",
			want:  false,
		},
		{
			name:  "Valid minute field - wildcard",
			input: "* * * * *",
			want:  true,
		},
		{
			name:  "Valid minute field - single value",
			input: "30 * * * *",
			want:  true,
		},
		{
			name:  "Valid minute field - range",
			input: "0-15 * * * *",
			want:  true,
		},
		{
			name:  "Valid minute field - comma-separated list",
			input: "0,15,30,45 * * * *",
			want:  true,
		},
		{
			name:  "Valid hour field - wildcard",
			input: "0 * * * *",
			want:  true,
		},
		{
			name:  "Valid hour field - single value",
			input: "0 12 * * *",
			want:  true,
		},
		{
			name:  "Valid hour field - range",
			input: "0 9-17 * * *",
			want:  true,
		},
		{
			name:  "Valid hour field - comma-separated list",
			input: "0 0,6,12,18 * * *",
			want:  true,
		},
		{
			name:  "Valid day of month field - wildcard",
			input: "0 0 * * *",
			want:  true,
		},
		{
			name:  "Valid day of month field - single value",
			input: "0 0 1 * *",
			want:  true,
		},
		{
			name:  "Valid day of month field - range",
			input: "0 0 10-15 * *",
			want:  true,
		},
		{
			name:  "Valid day of month field - comma-separated list",
			input: "0 0 1,15,31 * *",
			want:  true,
		},
		{
			name:  "Valid month field - single value",
			input: "0 0 * 12 *",
			want:  true,
		},
		{
			name:  "Valid month field - range",
			input: "0 0 * 4-6 *",
			want:  true,
		},
		{
			name:  "Valid month field - comma-separated list",
			input: "0 0 * 1,4,7,10 *",
			want:  true,
		},
		{
			name:  "Valid day of week field - single value",
			input: "0 0 * * 1",
			want:  true,
		},
		{
			name:  "Valid day of week field - range",
			input: "0 0 * * 1-5",
			want:  true,
		},
		{
			name:  "Valid day of week field - comma-separated list",
			input: "0 0 * * 1,3,5",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCronSpec(tt.input)
			if got != tt.want {
				t.Errorf("isCronSpec(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTimeToCronSpec(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "Valid time string",
			input:    "11:45",
			expected: "45 11 * * *",
			wantErr:  false,
		},
		{
			name:     "Invalid time string",
			input:    "11:61",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Empty input",
			input:    "",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := timeToCronSpec(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("timeToCronSpec(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("timeToCronSpec(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHourMinMoreThan(t *testing.T) {
	testCases := []struct {
		a, b     time.Time
		expected bool
	}{
		{
			a:        time.Date(2023, 4, 5, 13, 45, 0, 0, time.UTC),
			b:        time.Date(2023, 4, 5, 9, 10, 0, 0, time.UTC),
			expected: true,
		},
		{
			a:        time.Date(2023, 4, 5, 13, 45, 0, 0, time.UTC),
			b:        time.Date(2023, 4, 5, 13, 13, 0, 0, time.UTC),
			expected: true,
		},
		{
			a:        time.Date(2023, 4, 5, 9, 10, 0, 0, time.UTC),
			b:        time.Date(2023, 4, 5, 13, 45, 0, 0, time.UTC),
			expected: false,
		},
		{
			a:        time.Date(2023, 4, 5, 13, 45, 0, 0, time.UTC),
			b:        time.Date(2023, 4, 5, 13, 45, 0, 0, time.UTC),
			expected: false,
		},
	}

	for i, tc := range testCases {
		result := hourMinMoreThan(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("Test case %d failed: a=%v, b=%v, expected=%v, got=%v", i, tc.a, tc.b, tc.expected, result)
		}
	}
}
