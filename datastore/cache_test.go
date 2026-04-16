/*
AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  It is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package datastore

import (
	"errors"
	"testing"
)

func Test(t *testing.T) {
	tests := []struct {
		action, name, value, want string
		ok                        bool // true if action returns an error and is expected to succeed.
	}{
		{
			action: "get",
			name:   "a",
			ok:     false,
		},
		{
			action: "set",
			name:   "a",
			value:  "aa",
		},
		{
			action: "get",
			name:   "a",
			want:   "aa",
			ok:     true,
		},
		{
			action: "set",
			name:   "b",
			value:  "bb",
		},
		{
			action: "delete",
			name:   "a",
		},
		{
			action: "get",
			name:   "a",
			ok:     false,
		},
		{
			action: "get",
			name:   "b",
			want:   "bb",
			ok:     true,
		},
		{
			action: "reset",
		},
		{
			action: "get",
			name:   "b",
			ok:     false,
		},
	}

	var cache Cache = NewEntityCache()

	for i, test := range tests {
		var k Key = Key{Name: test.name}

		switch test.action {
		case "get":
			var v NameValue
			err := cache.Get(&k, &v)
			if err != nil {
				if test.ok {
					t.Errorf("Test %d: Get(%s) returned unexpected error: %v", i, test.name, err)
				}
				var errCacheMiss ErrCacheMiss
				if !errors.As(err, &errCacheMiss) {
					t.Errorf("Test %d: Get(%s) returned wrong error: %v", i, test.name, err)
				}
				continue // Got expected type of error.
			}
			if !test.ok {
				t.Errorf("Test %d: Get(%s) did not return error", i, test.name)
			}
			if test.want != v.Value {
				t.Errorf("Test %d: Get(%s) returned wrong value: %s", i, test.name, v.Value)
			}

		case "set":
			v := NameValue{Name: test.name, Value: test.value}
			err := cache.Set(&k, &v)
			if err != nil {
				t.Errorf("Test %d: Set(%s,%s) returned unexpected error: %v", i, test.name, test.value, err)
			}

		case "delete":
			cache.Delete(&k)

		case "reset":
			cache.Reset()

		default:
			panic("unexpected test action: " + test.action)
		}

	}
}
