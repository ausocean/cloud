/*
DESCRIPTION
  MtsMedia datastore type and functions.

AUTHORS
  Alan Noble <alan@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2019-2021 the Australian Ocean Lab (AusOcean).

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

package model

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"time"
	"unsafe"

	"github.com/Comcast/gots/v2/packet"
	"github.com/ausocean/av/container/mts"
	"github.com/ausocean/av/container/mts/pes"
	"github.com/ausocean/cloud/datastore"
)

const (
	typeMtsMedia = "MtsMedia" // MtsMedia datastore type.
)

var (
	ErrInvalidMID          = errors.New("invalid MID")
	ErrIdenticalTimestamps = errors.New("too many identical timestamps")
	ErrInvalidRanges       = errors.New("cannot have both geohash and timestamp ranges")
	ErrInvalidKeyName      = errors.New("malformed MtsMedia key name")
	ErrMediaNotFound       = errors.New("media not found")
	ErrInvalidMtsPackets   = errors.New("clip contains invalid MTS packets")
	ErrNoMtsPackets        = errors.New("clip contains no MTS packets")
	ErrNoMediaPID          = errors.New("no media PID found")
)

// MtsMedia represents a clip of continuous audio/video media data in
// MPEG-TS (MTS) format. Continues is true if the clip is continuous
// in time with respect to the previous one, false otherwise.
//
// The Media ID (MID) can be any identifier that uniquely identifies
// the media source, but conventionally it is formed from the 48-bit
// MAC address followed by the 4-bit encoding of the pin of the
// source device. See ToMID.
//
// NB: Since App Engine stores []byte as a blob, the clip cannot
// exceed 1MB in size. The "noindex" datastore attribute prevents the
// datastore from attempting to index the clip (and failing).
type MtsMedia struct {
	MID       int64          // Media ID.
	Geohash   string         // Geohash, if any.
	Timestamp int64          // Timestamp (in seconds).
	PTS       int64          // Presentation timestamp (in MTS frequency units).
	Duration  int64          // Duration of clip (in MTS frequency units).
	Continues bool           // True if this clip continues from the previous one, false if there a discontinuity.
	Type      string         // MIME type.
	Metadata  string         // Other metadata, if any.
	Date      time.Time      // Date/time this record was created.
	Clip      []byte         `datastore:",noindex"` // Media data.
	Key       *datastore.Key `datastore:"__key__"`  // Not persistent but populated upon reading from the datastore.
	FramePTS  int64          `datastore:"-"`        // Frame period in PTS frequency units (not persistent).
}

// MTSFragment is a series of continuous MtsMedia clips.
// MTSFragment does not have a limit on how many MtsMedias it can contain, whereas MtsMedia clips must be under 1MB.
type MTSFragment struct {
	Medias      []*MtsMedia // MtsMedias that make up fragment.
	Type        string      // MIME type of all this fragment's MtsMedias.
	Duration    int64       // Total duration of media that makes up the fragment in MTS frequency units.
	DurationSec int64       // Total duration of media that makes up the fragment in seconds.
	TSRange     [2]int64    // Contains the timestamp of the first MtsMedia and the timestamp directly after the the duration of the last MtsMedia.
	Continues   bool        // True if this fragment continues from the previous fragment, false if there a discontinuity.
}

// Encode serializes an MtsMedia entity as follows:
//
//   - Octet(s)  Value
//   - 0-2       Reserved
//   - 3         Continues
//   - 4-11      MID (8 octets)
//   - 12-23     Geohash (12 octets)
//   - 24-31     Timestamp (8 octets)
//   - 32-37     PTS (6 octets)
//   - 38-43     Duration (6 octets)
//   - 44-47     Type length (4 octets)
//   - 48-51     Metadata length (4 octets)
//   - 52-55     Clip length (4 octets)
//   - >=56      Type, metadata, and clip data
func (m *MtsMedia) Encode() []byte {
	lenType := len(m.Type)
	lenMeta := len(m.Metadata)
	lenClip := len(m.Clip)
	b := make([]byte, 56+lenType+lenMeta+lenClip)
	if m.Continues {
		b[3] |= 0x01
	}
	binary.BigEndian.PutUint64(b[4:12], uint64(m.MID))
	copy(b[12:24], m.Geohash)
	binary.BigEndian.PutUint64(b[24:32], uint64(m.Timestamp))
	putUint48(b[32:38], uint64(m.PTS))
	putUint48(b[38:44], uint64(m.Duration))
	binary.BigEndian.PutUint32(b[44:48], uint32(lenType))
	binary.BigEndian.PutUint32(b[48:52], uint32(lenMeta))
	binary.BigEndian.PutUint32(b[52:56], uint32(lenClip))
	copy(b[56:56+lenType], m.Type)
	copy(b[56+lenType:56+lenType+lenMeta], m.Metadata)
	copy(b[56+lenType+lenMeta:], m.Clip)
	return b
}

// Decode deserializes a MtsMedia.
func (m *MtsMedia) Decode(b []byte) error {
	if b[3]&0x01 == 1 {
		m.Continues = true
	}
	m.MID = int64(binary.BigEndian.Uint64(b[4:12]))
	m.Geohash = string(bytes.Trim(b[12:24], "\x00"))
	m.Timestamp = int64(binary.BigEndian.Uint64(b[24:32]))
	m.PTS = int64(getUint48(b[32:38]))
	m.Duration = int64(getUint48(b[38:44]))
	lenType := binary.BigEndian.Uint32(b[44:48])
	lenMeta := binary.BigEndian.Uint32(b[48:52])
	lenClip := binary.BigEndian.Uint32(b[52:56])
	m.Type = string(b[56 : 56+lenType])
	m.Metadata = string(b[56+lenType : 56+lenType+lenMeta])
	m.Clip = b[56+lenType+lenMeta : 56+lenType+lenMeta+lenClip]
	return nil
}

// Copy is not currently implemented.
func (m *MtsMedia) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

// GetCache returns nil, indicating no caching.
func (m *MtsMedia) GetCache() datastore.Cache {
	return nil
}

// KeyID returns the MtsMedia key ID as an unsigned integer.
func (m *MtsMedia) KeyID() uint64 {
	return uint64(m.Key.ID)
}

// split finds a point to split the MTS data in m.Clip so that the LHS is
// a clip with a size less that 1MB (App Engine's maximum blob size).
// m.Clip is replaced with that new clip and m's PTS and calculated duration are set to match.
//
// The split will happen at a PAT boundary or at the last MTS packet
// boundary, unless discontinuities are found, in which case the split will
// happen at the discontinuity.
// The clip's PTS is the PTS of the first media PES packet.
// The clip's duration is calculated by finding the difference between the clip's
// PTS and the PTS of the clip's last PES packet with a matching PID, plus one PES frame period.
// It is expected that when this function is called, m.Clip should contain at least one PAT and PMT.
func (m *MtsMedia) split() error {
	const maxSize = int(datastore.MaxBlob/mts.PacketSize) * mts.PacketSize
	sz := len(m.Clip)

	if sz > maxSize {
		_, i, err := mts.LastPid(m.Clip[:maxSize], mts.PatPid)
		if err != nil || i == 0 {
			log.Printf("could not find suitable PAT at which to split")
			i = maxSize
		}
		log.Printf("splitting large clip of %d bytes at %d", sz, i)
		sz = i
	}

	pid, err := firstMediaPID(m.Clip[:sz])
	if err != nil {
		return fmt.Errorf("could not find media PID: %w", err)
	}

	m.Continues = true
	var firstPTS int64 = -1
	var currentPTS int64
	var pkt *packet.Packet
	for i := 0; i < sz; i += mts.PacketSize {
		pkt = gotsPacket(m.Clip[i : i+mts.PacketSize])

		// Read PID, skip packet if no match.
		id := pkt.PID()
		if id != int(pid) {
			continue
		}
		var pts int64
		if pkt.PayloadUnitStartIndicator() {
			pts, err = mts.GetPTS(m.Clip[i:])
			if err != nil {
				return fmt.Errorf("could not find PTS where expected: %w", err)
			}
		} else {
			continue
		}
		currentPTS = pts
		if firstPTS == -1 {
			firstPTS = currentPTS
			m.PTS = firstPTS
		}

		// Read AFC, an AFC of 3 indicates an adaptation field followed by a
		// payload in which case we would like to check for discontinuity.
		if pkt.AdaptationFieldControl() == packet.PayloadAndAdaptationFieldFlag {
			af, err := pkt.AdaptationField()
			if err != nil {
				return err
			}
			d, err := af.Discontinuity()
			if err != nil {
				return fmt.Errorf("could not get discontinuity indicator from adaptation field: %w", err)
			}
			if d {
				if i == 0 {
					// Segment starts with a discontinuity.
					m.Continues = false
					// Clear discontinuity indicator if clip is H264.
					s, err := pes.SIDToMIMEType(pes.H264SID)
					if err != nil {
						panic(fmt.Errorf("could not get type from H264 SID: %w", err))
					}
					if m.Type == s {
						err = af.SetDiscontinuity(false)
						if err != nil {
							return fmt.Errorf("could not set discontinuity indicator: %w", err)
						}
					}
					continue
				}
				// Ending segment at discontinuity.
				log.Printf("splitting at discontinuity")
				sz = i
				break
			}
		}
	}
	// Check if PTS rollover has occurred.
	var adj int64
	if currentPTS < firstPTS {
		adj = mts.MaxPTS
	}

	m.Duration = adj + currentPTS - firstPTS + m.FramePTS

	m.Clip = m.Clip[:sz]

	return nil
}

// WriteMtsMedia stores MTS (MPEG-TS) data (i.e, audio/video data),
// splitting it into 1MB chunks and/or at discontinuities.
// Note: this function currently will only write media from one elementary stream in the program.
func WriteMtsMedia(ctx context.Context, store datastore.Store, m *MtsMedia) error {
	if len(m.Clip) == 0 {
		return ErrNoMtsPackets
	}
	if (len(m.Clip) % mts.PacketSize) != 0 {
		return ErrInvalidMtsPackets
	}
	pid, err := firstMediaPID(m.Clip)
	if err != nil {
		return ErrMediaNotFound
	}

	var st int64
	media := m
	for i := 0; i < len(m.Clip); i += len(media.Clip) {
		media.Clip = m.Clip[i:]
		media.FramePTS = m.FramePTS
		err := media.split()
		if err != nil {
			return fmt.Errorf("could not split MTS media: %w", err)
		}
		media.Date = time.Now()
		key := store.IDKey(typeMtsMedia, datastore.IDKey(media.MID, media.Timestamp, st))
		_, err = store.Put(ctx, key, media)
		if err != nil {
			return fmt.Errorf("error writing MTS media with PID %d and length %d bytes: %w", pid, len(media.Clip), err)
		}
		st++
		if st == 1<<datastore.SubTimeBits {
			return ErrIdenticalTimestamps
		}
	}
	return nil
}

// gotsPacket takes a byte slice and returns a Packet as defined in github.com/comcast/gots/packet.
// TODO: replace this with a type conversion as described here: https://github.com/golang/go/issues/395
// when it becomes available in Go 1.17.
func gotsPacket(b []byte) *packet.Packet {
	if len(b) != packet.PacketSize {
		panic("invalid packet size")
	}
	return *(**packet.Packet)(unsafe.Pointer(&b))
}

// firstMediaPID will iterate over the MTS packets of the given clip and find and return the first
// PID that is not for a PAT or PMT packet.
func firstMediaPID(clip []byte) (pid uint16, err error) {
	if len(clip) == 0 {
		return 0, ErrNoMtsPackets
	}
	if (len(clip) % mts.PacketSize) != 0 {
		return 0, ErrInvalidMtsPackets
	}
	for i := 0; i < len(clip); i += mts.PacketSize {
		pid, err = mts.PID(clip[i : i+mts.PacketSize])
		if err != nil {
			return 0, fmt.Errorf("could not get media PID: %w", err)
		}
		if pid != mts.PatPid && pid != mts.PmtPid {
			return pid, nil
		}
	}
	return 0, ErrNoMediaPID
}

// GetMtsMedia retrieves MTS media data for a given Media ID,
// optionally filtered by timestamp(s) and geohash(es). One timestamp
// represents an instant in time whereas two represents a time
// range. Similarly one geohash represents a single location whereas
// two represents a neighborhood of locations. Results are ordered by
// geohash, then timestamp, and then creation time. It is invalid to
// specify ranges for both geohashes and timestamps, since the
// datastore prohibits inequality filters on different properties.
//
// NB: FileStore queries are are limited to information encoded in the
// key, namely MID and Timestamp.
func GetMtsMedia(ctx context.Context, store datastore.Store, mid int64, gh []string, ts []int64) ([]MtsMedia, error) {
	q, err := newMtsMediaQuery(store, mid, gh, ts, false)
	if err != nil {
		return nil, err
	}
	var clips []MtsMedia
	_, err = store.GetAll(ctx, q, &clips)
	return clips, err
}

// GetMtsMediaKeys retrieves MtsMedia keys for a given Media ID,
// optionally filtered by timestamp(s) and geohash(es).
func GetMtsMediaKeys(ctx context.Context, store datastore.Store, mid int64, gh []string, ts []int64) ([]*datastore.Key, error) {
	q, err := newMtsMediaQuery(store, mid, gh, ts, true)
	if err != nil {
		return nil, err
	}
	return store.GetAll(ctx, q, nil)
}

// newMtsMediaQuery constructs an MtsMedia query.
func newMtsMediaQuery(store datastore.Store, mid int64, gh []string, ts []int64, keysOnly bool) (datastore.Query, error) {
	if len(gh) > 1 && len(ts) > 1 {
		return nil, ErrInvalidRanges
	}

	q := store.NewQuery(typeMtsMedia, keysOnly, "MID", "Timestamp")
	q.Filter("MID =", mid)

	if gh != nil {
		if len(gh) > 1 {
			q.Filter("Geohash >=", gh[0])
			q.Filter("Geohash <", gh[1])
			q.Order("Geohash")
		} else if len(gh) > 0 {
			q.Filter("Geohash =", gh[0])
		}
	}

	if ts != nil {
		if len(ts) > 1 {
			q.Filter("Timestamp >=", ts[0])
			if ts[1] < datastore.EpochEnd {
				q.Filter("Timestamp <", ts[1])
			}
			q.Order("Timestamp")
		} else if len(ts) > 0 {
			q.Filter("Timestamp =", ts[0])
		}
	}
	q.Order("Timestamp")
	return q, nil
}

// GetMtsMediaByKey retrieves a single MTS media entity by key ID.
func GetMtsMediaByKey(ctx context.Context, store datastore.Store, ky uint64) (*MtsMedia, error) {
	key := store.IDKey(typeMtsMedia, int64(ky))
	m := new(MtsMedia)
	err := store.Get(ctx, key, m)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	m.Key = key // Populate the key.
	return m, nil
}

// GetMtsMediaByKeys returns multiple MTS media given a range of key
// IDs, skipping over missing keys and fractional times. An error is
// returned only if nothing is found at all. This function is O(N)
// with the number of keys and should not be used with large numbers
// of keys. Use GetMtsMedia instead.
func GetMtsMediaByKeys(ctx context.Context, store datastore.Store, ky []uint64) ([]MtsMedia, error) {
	if len(ky) == 0 {
		return nil, ErrMediaNotFound
	}

	if len(ky) == 1 {
		m, err := GetMtsMediaByKey(ctx, store, ky[0])
		if err != nil {
			return nil, err
		}
		return []MtsMedia{*m}, nil
	}

	var media []MtsMedia
	step := uint64(1) << datastore.SubTimeBits
	for id := ky[0]; id < ky[len(ky)-1]; id += step {
		m, err := GetMtsMediaByKey(ctx, store, id)
		if err != nil {
			continue
		}
		media = append(media, *m)
	}
	if len(media) == 0 {
		return nil, ErrMediaNotFound
	}
	return media, nil
}

// FragmentMTSMedia returns a slice of MTSFragments from the given MtsMedia
// every period seconds or whenever there is a discontinutity.
// Tolerance, which is specified in MTS frequency units (90kHz), controls how
// strictly discontinuities are enforced and how strictly the period is adhered to.
func FragmentMTSMedia(in []MtsMedia, period, tolerance int64) (out []*MTSFragment) {
	if len(in) == 0 {
		return
	}

	// Convert seconds in MTS period units.
	period *= mts.PTSFrequency

	// Divide MtsMedia slice into fragments.
	var prev, start *MtsMedia
	frag := &MTSFragment{Continues: in[0].Continues}
	for i := 0; i < len(in); {
		m := &in[i]
		if frag.Duration == 0 {
			frag.Medias = append(frag.Medias, m)
			frag.Type = m.Type
			frag.Duration = m.Duration
			frag.TSRange[0] = m.Timestamp
			start = m
			prev = m
			i++
			continue
		}

		if m.Duration == 0 {
			i++
			continue
		}

		// Check for PTS roll over.
		if m.PTS < start.PTS {
			m.PTS += mts.MaxPTS
		}

		// Check for discontinuity.
		continues := true
		if m.PTS > prev.PTS+prev.Duration+tolerance || !m.Continues {
			continues = false
		}

		// If the period has elapsed, type has changed, or there is a discontinuity, add this fragment to the output.
		if (m.PTS > prev.PTS && frag.Duration+m.Duration > period+tolerance) || !continues || m.Type != start.Type {
			frag.DurationSec = frag.Duration / mts.PTSFrequency
			frag.TSRange[1] = frag.Medias[len(frag.Medias)-1].Timestamp + frag.Medias[len(frag.Medias)-1].Duration/mts.PTSFrequency
			out = append(out, frag)
			frag = &MTSFragment{Continues: continues}
			continue
		}
		frag.Medias = append(frag.Medias, m)
		frag.Duration += m.Duration
		prev = m
		i++
	}

	// Emit the remainder, if any.
	if frag.Duration > 0 {
		frag.DurationSec = frag.Duration / mts.PTSFrequency
		frag.TSRange[1] = frag.Medias[len(frag.Medias)-1].Timestamp + frag.Medias[len(frag.Medias)-1].Duration/mts.PTSFrequency
		out = append(out, frag)
	}

	return
}

// DeleteMtsMedia deletes all MTS media for a given Media ID.
func DeleteMtsMedia(ctx context.Context, store datastore.Store, mid int64) error {
	keys, err := GetMtsMediaKeys(ctx, store, mid, nil, nil)
	if err != nil {
		return err
	}
	return store.DeleteMulti(ctx, keys)
}

// UpdateMtsMedia deletes a MTS media and creates a new one in its place.
func UpdateMtsMedia(ctx context.Context, store datastore.Store, m MtsMedia, pid uint16) error {
	store.DeleteMulti(ctx, []*datastore.Key{m.Key})
	_, err := store.Put(ctx, m.Key, &m)
	return err
}

// ToMID returns a Media ID given a MAC address and a pin. It is
// formed from the 48-bit integer encoding of the mac (see MacEncode)
// followed by the 4-bit encoding of the pin.
func ToMID(mac, pin string) int64 {
	return MacEncode(mac)<<4 | int64(putMtsPin(pin))
}

// FromMID returns a MAC address and pin given a Media ID.
func FromMID(mid int64) (string, string) {
	return MacDecode(mid >> 4), getMtsPin(byte(mid & 0x0f))
}

// putMtsPin encodes a pin string, such as "V0" or "S3", into a nibble.
// Bits 0-1 represent the pin number and bits 2-3 represents the pin
// type.
func putMtsPin(pin string) byte {
	var b byte
	switch pin[0] {
	case 'V':
		// Zero.
	case 'S':
		b = 0x04
	case 'T':
		b = 0x08
	case 'B':
		b = 0x0C
	}
	pn := int(pin[1] - '0')
	b |= byte(pn)
	return b
}

// getMtsPin decodes a nibble into a pin string.
func getMtsPin(b byte) string {
	s := make([]byte, 2)
	pn := int(b & 0x03)
	s[1] = byte('0' + pn)
	switch (b >> 2) & 0x03 {
	case 0:
		s[0] = 'V'
	case 1:
		s[0] = 'S'
	case 2:
		s[0] = 'T'
	case 3:
		s[0] = 'B'
	}
	return string(s)
}

// putUint48 encodes a 48-bit integer into 6 bytes in big-endian order.
func putUint48(b []byte, val uint64) {
	b[0] = byte(val >> 40)
	b[1] = byte(val >> 32)
	b[2] = byte(val >> 24)
	b[3] = byte(val >> 16)
	b[4] = byte(val >> 8)
	b[5] = byte(val)
}

// getUint48 decodes 6 bytes in big-endian order into a 48-bit integer.
func getUint48(b []byte) uint64 {
	return uint64(b[0])<<40 | uint64(b[1])<<32 | uint64(b[2])<<24 | uint64(b[3])<<16 | uint64(b[4])<<8 | uint64(b[5])
}

// PTSToSeconds converts a duration in MPEG-TS Presentation Time Stamp (PTS) units to seconds.
func PTSToSeconds(pts int64) float64 {
	return float64(pts) / mts.PTSFrequency
}
