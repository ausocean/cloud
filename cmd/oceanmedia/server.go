/*
DESCRIPTION
  OceanMedia server and dependencies.

AUTHORS
  Elliot Shine <elliot@ausocean.org>

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
	"context"
	"log"
	"sync"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/storage"
)

const (
	// defaultBlockPoll is how often the datastore is polled while
	// waiting for a segment during a blocking reload.
	defaultBlockPoll = 500 * time.Millisecond

	// defaultBlockTimeout is how long a blocking reload will wait for a
	// segment before returning the current playlist.
	defaultBlockTimeout = 15 * time.Second
)

// urlSigner signs object storage URIs. It is satisfied by storage.Provider.
type urlSigner interface {
	SignURL(ctx context.Context, rawURI string, ttl time.Duration) (string, error)
}

type cachedURL struct {
	url     string
	expires time.Time
}

// server holds the dependencies used by OceanMedia's HTTP handlers.
type server struct {
	store        datastore.Store
	signer       urlSigner
	jwtSecret    []byte
	signedURLTTL time.Duration

	blockPoll    time.Duration
	blockTimeout time.Duration

	// now is a field so that time can be controlled in tests.
	now func() time.Time

	// Signed URLs are cached until half their lifetime has elapsed, so a
	// URL handed to a client always has at least half its lifetime left.
	signMu    sync.Mutex
	signCache map[string]cachedURL

	// loadEvent and loadSegments are fields so that they can be
	// overridden in tests.
	loadEvent    func(ctx context.Context, broadcastID string) (*model.BroadcastEvent, error)
	loadSegments func(ctx context.Context, broadcastID string) ([]model.MtsMediaV2, error)

	log func(string, ...any)
}

// newServer returns a new server with the supplied dependencies.
func newServer(store datastore.Store, signer urlSigner, jwtSecret []byte, signedURLTTL time.Duration) *server {
	if signedURLTTL <= 0 {
		signedURLTTL = storage.DefaultSignedURLExpiry
	}
	s := &server{
		store:        store,
		signer:       signer,
		jwtSecret:    jwtSecret,
		signedURLTTL: signedURLTTL,
		blockPoll:    defaultBlockPoll,
		blockTimeout: defaultBlockTimeout,
		now:          time.Now,
		signCache:    make(map[string]cachedURL),
		log:          log.Printf,
	}
	s.loadEvent = func(ctx context.Context, broadcastID string) (*model.BroadcastEvent, error) {
		return model.GetBroadcastEvent(ctx, s.store, broadcastID)
	}
	s.loadSegments = func(ctx context.Context, broadcastID string) ([]model.MtsMediaV2, error) {
		return model.GetMtsMediaV2ByBroadcast(ctx, s.store, broadcastID, nil, 0)
	}
	return s
}

// signURL returns a pre-signed URL for the given storage URI, caching the
// result until half the signed URL time to live has elapsed.
func (s *server) signURL(ctx context.Context, rawURI string) (string, error) {
	now := s.now()

	s.signMu.Lock()
	if c, ok := s.signCache[rawURI]; ok && now.Before(c.expires) {
		s.signMu.Unlock()
		return c.url, nil
	}
	s.signMu.Unlock()

	signed, err := s.signer.SignURL(ctx, rawURI, s.signedURLTTL)
	if err != nil {
		return "", err
	}

	s.signMu.Lock()
	s.signCache[rawURI] = cachedURL{url: signed, expires: now.Add(s.signedURLTTL / 2)}
	s.signMu.Unlock()

	return signed, nil
}
