/*
DESCRIPTION
  storage_test.go tests functionality in storage.go.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2021 the Australian Ocean Lab (AusOcean)

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

package broadcast

import "testing"

func TestGoogleStorageAddr(t *testing.T) {
	const (
		wantBkt = "ausocean"
		wantObj = "Secret-file.json"
		testURI = "gs://" + wantBkt + "/" + wantObj
	)

	bkt, obj, err := googleStorageAddr(testURI)
	if err != nil {
		t.Fatalf("did not expect error: %v from googleStorageAddr", err)
	}

	if bkt != wantBkt {
		t.Errorf("did not get expected bkt name, got: %s want: %s", bkt, wantBkt)
	}

	if obj != wantObj {
		t.Errorf("did not get expected obj name, got: %s want: %s", obj, wantObj)
	}
}
