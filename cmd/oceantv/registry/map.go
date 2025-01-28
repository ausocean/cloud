/*
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

package registry

import "sync"

// SafeMap is a thread-safe map that stores any type of value.
type SafeMap struct {
	mu sync.RWMutex
	m  map[string]any
}

// NewSafeMap creates a new SafeMap.
func NewSafeMap() *SafeMap {
	return &SafeMap{
		m: make(map[string]any),
	}
}

// Get retrieves a value from the map.
func (s *SafeMap) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.m[key]
	return val, ok
}

// Set stores a value in the map.
func (s *SafeMap) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}
