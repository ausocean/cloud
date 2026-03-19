package main

import (
	"testing"

	"github.com/ausocean/cloud/model"
)

// TestParsePins checks that parsePins correctly splits comma-separated pin strings.
func TestParsePins(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"V0", []string{"V0"}},
		{"A0,V0,S0", []string{"A0", "V0", "S0"}},
		{" V0 , S0 ", []string{"V0", "S0"}},
	}
	for _, tt := range tests {
		got := parsePins(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parsePins(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parsePins(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

// TestMIDDeduplication verifies that pins with colliding MID encodings
// (e.g. A0 and V0) are deduplicated when building the MID list.
// This test mirrors the logic in getV1MediaHandler.
func TestMIDDeduplication(t *testing.T) {
	mac := "00:00:00:00:00:01"
	pins := []string{"A0", "V0", "S0", "T0", "B0"}

	type midEntry struct {
		mid int64
		pin string
	}

	seenMIDs := make(map[int64]bool)
	var mids []midEntry
	for _, pin := range pins {
		mid := model.ToMID(mac, pin)
		if seenMIDs[mid] {
			continue
		}
		seenMIDs[mid] = true
		mids = append(mids, midEntry{mid: mid, pin: pin})
	}

	// A0 and V0 encode to the same MID, so we should have 4 unique entries, not 5.
	if len(mids) != 4 {
		t.Errorf("expected 4 unique MIDs, got %d: %v", len(mids), mids)
	}

	// Verify all MIDs are truly unique.
	seen := make(map[int64]int)
	for _, entry := range mids {
		seen[entry.mid]++
		if seen[entry.mid] > 1 {
			t.Errorf("duplicate MID %d found for pin %s", entry.mid, entry.pin)
		}
	}
}

// TestA0V0MIDCollision documents that A0 and V0 encode to the same MID.
// This is caused by putMtsPin not handling 'A' pins, making them default
// to 0x00 which is the same encoding as 'V'.
func TestA0V0MIDCollision(t *testing.T) {
	mac := "00:00:00:00:00:01"

	midA0 := model.ToMID(mac, "A0")
	midV0 := model.ToMID(mac, "V0")

	if midA0 != midV0 {
		t.Skipf("A0 and V0 now encode to different MIDs (A0=%d, V0=%d), collision fixed upstream", midA0, midV0)
	}

	// S0 and T0 should be different from V0.
	midS0 := model.ToMID(mac, "S0")
	midT0 := model.ToMID(mac, "T0")

	if midS0 == midV0 {
		t.Errorf("S0 and V0 should encode to different MIDs, both got %d", midV0)
	}
	if midT0 == midV0 {
		t.Errorf("T0 and V0 should encode to different MIDs, both got %d", midV0)
	}
	if midS0 == midT0 {
		t.Errorf("S0 and T0 should encode to different MIDs, both got %d", midS0)
	}
}
