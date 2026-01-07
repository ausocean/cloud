/*
DESCRIPTION
  Contains tests for video handling utilities provided in video.go.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2019-2024 the Australian Ocean Lab (AusOcean).

  This file is free software: you can redistribute it and/or modify it
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
	"bytes"
	"reflect"
	"testing"

	"context"

	"github.com/ausocean/av/container/mts"
	"github.com/ausocean/av/container/mts/meta"
	"github.com/ausocean/av/container/mts/pes"
	"github.com/ausocean/av/container/mts/psi"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
)

var (
	patTable = (&psi.PSI{
		PointerField:    0x00,
		TableID:         0x00,
		SyntaxIndicator: true,
		PrivateBit:      false,
		SectionLen:      0x0d,
		SyntaxSection: &psi.SyntaxSection{
			TableIDExt:  0x01,
			Version:     0,
			CurrentNext: true,
			Section:     0,
			LastSection: 0,
			SpecificData: &psi.PAT{
				Program:       0x01,
				ProgramMapPID: 0x1000,
			},
		},
	}).Bytes()

	pmtTable = (&psi.PSI{
		PointerField:    0x00,
		TableID:         0x02,
		SyntaxIndicator: true,
		PrivateBit:      false,
		SectionLen:      0x12,
		SyntaxSection: &psi.SyntaxSection{
			TableIDExt:  0x01,
			Version:     0,
			CurrentNext: true,
			Section:     0,
			LastSection: 0,
			SpecificData: &psi.PMT{
				ProgramClockPID: 0x0100,
				ProgramInfoLen:  0,
				StreamSpecificData: &psi.StreamSpecificData{
					StreamType:    pes.H264SID,
					PID:           0x0100,
					StreamInfoLen: 0x00,
				},
			},
		},
	}).Bytes()
)

// TestWriteMtsMedia test writeMtsMedia using a test writer rather than writing to the datastore.
func TestWriteMtsMedia(t *testing.T) {
	ctx := context.Background()

	mts.Meta = meta.New()

	// The packet types we will be dealing with.
	const (
		pat = iota
		pmt
		vid
	)

	tests := []struct {
		pkts []int    // The order of packets we'd like to write.
		ts   []string // The timestamps of packets (only when we have PMT is this considered).
		rng  [][2]int // The idx ranges over the entire clip for the resultant segments.
	}{
		{
			pkts: []int{pat, pmt, vid, vid, vid},
			ts:   []string{"", "1", "", "", ""},
			rng: [][2]int{
				{0 * mts.PacketSize, 5 * mts.PacketSize},
			},
		},
		{
			pkts: []int{pat, pmt, vid, vid, vid, pat, pmt, vid, vid},
			ts:   []string{"", "1", "", "", "", "", "2", "", ""},
			rng: [][2]int{
				{0 * mts.PacketSize, 5 * mts.PacketSize},
				{5 * mts.PacketSize, 9 * mts.PacketSize},
			},
		},
		{
			pkts: []int{vid, pat, pmt, vid, vid, vid},
			ts:   []string{"", "", "1", "", "", ""},
			rng: [][2]int{
				{1 * mts.PacketSize, 6 * mts.PacketSize},
			},
		},
		{
			pkts: []int{vid, vid, vid},
			ts:   []string{"", "", ""},
			rng: [][2]int{
				{0 * mts.PacketSize, 3 * mts.PacketSize},
			},
		},
	}

	// Run through each test.
	for i, test := range tests {
		var (
			clip bytes.Buffer // Will hold the entire clip for a given test.
			got  [][]byte     // The 1-second clips segment has produced.
			want [][]byte     // The 1-second clips we expect.
		)

		// writeMtsMedia will use this function to write the 1-second clips.
		// In this case the function writes to our clips buffer.
		write := func(ctx context.Context, s datastore.Store, m *model.MtsMedia) error {
			got = append(got, m.Clip)
			return nil
		}

		// Write the packets as directed by test.pkts.
		for j, p := range test.pkts {
			switch p {
			case pat:
				err := writePATToBuffer(&clip)
				if err != nil {
					t.Fatalf("did not expect error: %v writing PAT", err)
				}
			case pmt:
				mts.Meta.Add("ts", test.ts[j])
				err := writePMTToBuffer(&clip)
				if err != nil {
					t.Fatalf("did not expect error: %v writing PMT", err)
				}
			case vid:
				err := writeMediaToBuffer(&clip)
				if err != nil {
					t.Fatalf("did not expect error: %v writing media packet", err)
				}
			default:
				t.Fatal("invalid packet type")
			}
		}

		// Now use writeMtsMedia to get 1-second segments
		bytes := clip.Bytes()
		err := writeMtsMedia(ctx, 0, "", 0, clip.Bytes(), write)
		if err != nil {
			t.Errorf("unexpected error: %v for test: %d", err, i)
		}

		// Generate the clips that we expect using the ranges specified in the tests.
		for _, r := range test.rng {
			want = append(want, bytes[r[0]:r[1]])
		}

		// Check the clips.
		if !reflect.DeepEqual(got, want) {
			t.Errorf("did not get expected result for test: %v\nGot: %v\nWant: %v\n", i, got, want)
		}
	}
}

// writePATToBuffer writes a PAT MPEG-TS packet to the given buffer.
func writePATToBuffer(b *bytes.Buffer) error {
	pat := mts.Packet{
		PUSI:    true,
		PID:     mts.PatPid,
		CC:      0,
		AFC:     mts.HasPayload,
		Payload: psi.AddPadding(patTable),
	}
	_, err := b.Write(pat.Bytes(nil))
	if err != nil {
		return err
	}
	return nil
}

// writePMTToBuffer writes a PMT MPEG-TS packet to the given buffer.
func writePMTToBuffer(b *bytes.Buffer) error {
	pmtTable, err := updateMeta(pmtTable)
	if err != nil {
		return err
	}
	pmt := mts.Packet{
		PUSI:    true,
		PID:     mts.PmtPid,
		CC:      0,
		AFC:     mts.HasPayload,
		Payload: psi.AddPadding(pmtTable),
	}
	_, err = b.Write(pmt.Bytes(nil))
	if err != nil {
		return err
	}
	return nil
}

// writeMediaToBuffer writes a media MPEG-TS packet to the given buffer.
func writeMediaToBuffer(b *bytes.Buffer) error {
	pesPkt := pes.Packet{
		StreamID:     pes.H264SID,
		PDI:          1,
		PTS:          0,
		Data:         []byte{},
		HeaderLength: 5,
	}
	buf := pesPkt.Bytes(nil)

	pkt := mts.Packet{
		PUSI: true,
		PID:  mts.PIDVideo,
		RAI:  true,
		CC:   0,
		AFC:  mts.HasAdaptationField | mts.HasPayload,
		PCRF: true,
	}
	pkt.FillPayload(buf)

	_, err := b.Write(pkt.Bytes(nil))
	if err != nil {
		return err
	}
	return nil
}

// updateMeta adds/updates a metaData descriptor in the given psi bytes using data
// contained in the global Meta struct.
func updateMeta(b []byte) ([]byte, error) {
	p := psi.PSIBytes(b)
	err := p.AddDescriptor(psi.MetadataTag, mts.Meta.Encode())
	return []byte(p), err
}
