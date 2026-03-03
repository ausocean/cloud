package main

import (
	"testing"
	"time"
)

func fixedBroadcastTestTime(t testing.TB) time.Time {
	t.Helper()

	loc, err := time.LoadLocation(locationID)
	if err != nil {
		t.Fatalf("could not load test location %q: %v", locationID, err)
	}

	// Midday in Adelaide standard time avoids date rollovers and DST edge cases.
	return time.Date(2025, time.June, 18, 12, 0, 0, 0, loc)
}
