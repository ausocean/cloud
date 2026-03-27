/*
AUTHORS
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

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
  in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

// Ocean Bench test data generator.
// This tool generates randomized MtsMedia test entities for standalone mode
// testing without needing the cloud datastore. It is highly configurable
// via JSON to control exactly how many entities to generate and how their
// properties should be fuzzed (e.g. durations, past dates, active pins).

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/ausocean/cloud/model"
)

// MediaConfig defines how the individual MtsMedia entity properties are fuzzed.
type MediaConfig struct {
	DaysInPast int `json:"days_in_past"` // Spread media dates over this many past days
	// The allowed pins to randomly distribute across.
	// IMPORTANT: Only V, S, T, and B pin types are valid for MtsMedia.
	// 'A' pins are NOT handled by model.putMtsPin and encode identically
	// to 'V' pins (both become 0x00), causing duplicate MIDs and key clashes!
	// Valid examples: "V0", "V1", "S0", "S1", "T0", "B0"
	Pins           []string `json:"pins"`
	DurationSecMin int      `json:"duration_sec_min"` // Minimum clip duration in seconds
	DurationSecMax int      `json:"duration_sec_max"` // Maximum clip duration in seconds
}

// GeneratorConfig defines the configuration parsed from JSON.
type GeneratorConfig struct {
	OutputFile     string      `json:"output_file"`      // Path relative to execution dir
	Seed           int64       `json:"seed"`             // Random seed for reproducible output
	Sites          int         `json:"sites"`            // Exact number of sites
	DevicesPerSite int         `json:"devices_per_site"` // Exact devices per site
	MediaPerDevice int         `json:"media_per_device"` // Exact media clips per device
	MediaConfig    MediaConfig `json:"media_config"`     // Fuzzing configuration for media
}

// OutputData matches the TestDataConfig struct expected by oceanbench standalone mode.
type OutputData struct {
	Sites    []*model.Site     `json:"sites"`
	Users    []*model.User     `json:"users"`
	Devices  []*model.Device   `json:"devices"`
	MtsMedia []*model.MtsMedia `json:"mtsmedia"`
}

func main() {
	// Provide a CLI flag for users to point to custom config files
	configPath := flag.String("config", "config.json", "Path to the JSON generator config")
	flag.Parse()

	b, err := os.ReadFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config file: %v\n", err)
		os.Exit(1)
	}

	var cfg GeneratorConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse config file: %v\n", err)
		os.Exit(1)
	}

	rng := rand.New(rand.NewSource(cfg.Seed))
	if cfg.Seed == 0 {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	out := OutputData{
		Sites:    make([]*model.Site, 0),
		Users:    make([]*model.User, 0),
		Devices:  make([]*model.Device, 0),
		MtsMedia: make([]*model.MtsMedia, 0),
	}

	globalDeviceIndex := 0
	now := time.Now().Truncate(time.Hour)
	baseTS := now.Unix()

	// Generate standard numbers of items per config
	for i := 0; i < cfg.Sites; i++ {
		skey := int64(i + 1)
		public := (i%2 == 0)

		// Create Site
		out.Sites = append(out.Sites, &model.Site{
			Skey:    skey,
			Name:    fmt.Sprintf("Auto Site %d", skey),
			Enabled: true,
			Public:  public,
		})

		// Create Super Admin User for Site
		out.Users = append(out.Users, &model.User{
			Skey:  skey,
			Email: "localuser@localhost",
			Perm:  7, // View/Write/Admin
		})

		// Create Devices for Site
		for j := 0; j < cfg.DevicesPerSite; j++ {
			globalDeviceIndex++
			macVal := int64(globalDeviceIndex)

			dev := &model.Device{
				Skey:          skey,
				Mac:           macVal,
				Dkey:          int64(j + 1),
				Name:          fmt.Sprintf("auto-dev-%d-%d", skey, j+1),
				Inputs:        "V0,S0,T0,B0", // Only valid MTS pin types (A pins encode same as V, so excluded)
				MonitorPeriod: 60,
				Enabled:       true,
			}
			out.Devices = append(out.Devices, dev)

			// Generate fuzzed media
			generateMedia(&out, &cfg, rng, dev, cfg.MediaPerDevice, baseTS)
		}
	}

	outBytes, err := json.MarshalIndent(out, "", "    ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling final JSON: %v\n", err)
		os.Exit(1)
	}

	// Make sure the directory exists
	outDir := filepath.Dir(cfg.OutputFile)
	if outDir != "." && outDir != "" {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output dir: %v\n", err)
			os.Exit(1)
		}
	}

	if err := os.WriteFile(cfg.OutputFile, outBytes, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s successfully!\n", cfg.OutputFile)
	fmt.Printf("  Sites:    %d\n", len(out.Sites))
	fmt.Printf("  Devices:  %d\n", len(out.Devices))
	fmt.Printf("  MtsMedia: %d\n", len(out.MtsMedia))
}

func generateMedia(out *OutputData, cfg *GeneratorConfig, rng *rand.Rand, dev *model.Device, count int, baseTS int64) {
	macStr := model.MacDecode(dev.Mac)

	pins := cfg.MediaConfig.Pins
	if len(pins) == 0 {
		pins = []string{"S0"} // fallback
	}

	// Spread them across DaysInPast
	maxOffsetSecs := int64(cfg.MediaConfig.DaysInPast) * 24 * 3600
	if maxOffsetSecs <= 0 {
		maxOffsetSecs = 86400 // Default to 1 day if 0
	}

	durMin := cfg.MediaConfig.DurationSecMin
	durMax := cfg.MediaConfig.DurationSecMax
	if durMax < durMin {
		durMax = durMin
	}

	for i := 0; i < count; i++ {
		pin := pins[rng.Intn(len(pins))]
		mid := model.ToMID(macStr, pin)

		// Fuzz the creation time.
		ts := baseTS - rng.Int63n(maxOffsetSecs)

		mediaType := "audio/wav"
		switch pin[0] {
		case 'V':
			mediaType = "video/mp2t"
		case 'T':
			mediaType = "text/plain"
		}

		// Fuzz the duration in seconds, then convert to PTS (90,000 pts ticks per second)
		durSec := durMin
		if durMax > durMin {
			durSec = durMin + rng.Intn(durMax-durMin+1)
		}
		durationPTS := int64(durSec) * 90000

		out.MtsMedia = append(out.MtsMedia, &model.MtsMedia{
			MID:       mid,
			Geohash:   "r1f9",
			Timestamp: ts,
			PTS:       ts * 90000,
			Duration:  durationPTS,
			Continues: rng.Intn(10) > 2, // 70% continues realistically
			Type:      mediaType,
			Date:      time.Unix(ts, 0).UTC(),
			Clip:      []byte{}, // empty byte slice means zero size in UI but valid entity
		})
	}
}
