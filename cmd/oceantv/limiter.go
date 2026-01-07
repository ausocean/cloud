/*
DESCRIPTION
	limiter.go provides an interface and implementation for a rate limiter using a token bucket algorithm.

AUTHORS
	Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
	Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

	This file is part of Ocean TV. Ocean TV is free software: you can
	redistribute it and/or modify it under the terms of the GNU
	General Public License as published by the Free Software
	Foundation, either version 3 of the License, or (at your option)
	any later version.

	Ocean TV is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
)

// RateLimiter is an interface for a rate limiter.
type RateLimiter interface {
	RequestOK() bool
}

// OceanTokenBucketLimiter is a rate limiter that uses a token bucket algorithm.
// Its state is stored in a Store, and it is identified by a unique ID.
type OceanTokenBucketLimiter struct {
	ID             string
	Store          `json:"-"`
	Tokens         float64
	MaxTokens      float64
	RefillRate     float64 // Tokens per hour.
	LastRefillTime time.Time
}

// OceanTokenBucketLimiter consts.
const (
	sharedSKey       = -1
	tokenBucketScope = "token_bucket"
)

// GetOceanTokenBucketLimiter gets a token bucket limiter from a Store. If the
// limiter does not exist, it is created with the given maxTokens and refillRate (tokens per hour).
func GetOceanTokenBucketLimiter(maxTokens, refillRate float64, id string, store Store) (*OceanTokenBucketLimiter, error) {
	var tokenBucketLimiter OceanTokenBucketLimiter
	bucketVar, err := model.GetVariable(context.Background(), store, sharedSKey, tokenBucketScope+"."+id)
	switch {
	case errors.Is(err, datastore.ErrNoSuchEntity):
		log.Printf("token bucket limiter not found, creating new one")
		tokenBucketLimiter = OceanTokenBucketLimiter{
			ID:             id,
			Store:          store,
			Tokens:         maxTokens,
			MaxTokens:      maxTokens,
			RefillRate:     refillRate,
			LastRefillTime: time.Now(),
		}
		err := tokenBucketLimiter.store()
		if err != nil {
			return nil, fmt.Errorf("could not store token bucket limiter: %w", err)
		}
		return &tokenBucketLimiter, nil
	case err != nil:
		return nil, fmt.Errorf("could not get token bucket limiter: %w", err)
	default: // Do nothing.
	}
	log.Printf("token bucket limiter found")

	// If we're here it means we've found the token bucket limiter.
	// Unmarshal the value and return it.
	err = json.Unmarshal([]byte(bucketVar.Value), &tokenBucketLimiter)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal token bucket limiter: %w", err)
	}
	tokenBucketLimiter.Store = store
	return &tokenBucketLimiter, nil
}

// RequestOK returns true if a request is allowed (we have enough tokens), and
// false otherwise.
func (l *OceanTokenBucketLimiter) RequestOK() bool {
	elapsed := time.Since(l.LastRefillTime)
	toAdd := elapsed.Hours() * l.RefillRate
	l.Tokens = math.Min(l.MaxTokens, l.Tokens+toAdd)
	l.LastRefillTime = time.Now()

	ok := false
	if l.Tokens >= 1 {
		l.Tokens--
		ok = true
	}
	err := l.store()
	if err != nil {
		log.Printf("could not store token bucket limiter: %v", err)
		ok = false
	}
	return ok
}

func (l *OceanTokenBucketLimiter) store() error {
	data, err := json.Marshal(l)
	if err != nil {
		return fmt.Errorf("could not marshal token bucket limiter: %w", err)
	}
	err = model.PutVariable(context.Background(), l.Store, sharedSKey, tokenBucketScope+"."+l.ID, string(data))
	if err != nil {
		return fmt.Errorf("could not put token bucket limiter: %w", err)
	}
	return nil
}
