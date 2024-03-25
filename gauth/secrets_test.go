/*
LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Cron. Ocean Cron is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean Cron is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with NetReceiver in gpl.txt. If not, see
  <http://www.gnu.org/licenses/>.
*/

package gauth

import (
	"context"
	"os"
	"testing"
)

const (
	projectID = "oceancron"
	secretKey = "cronSecret"
)

func TestGetSecrets(t *testing.T) {
	if os.Getenv("OCEANCRON_SECRETS") == "" {
		t.Skipf("skipping TestGetSecrets")
	}
	ctx := context.Background()
	var err error
	_, err = GetSecret(ctx, projectID, secretKey)
	if err != nil {
		t.Errorf("GetSecret failed: %v", err)
	}
	_, err = GetHexSecret(ctx, projectID, secretKey)
	if err != nil {
		t.Errorf("GetHexSecret failed: %v", err)
	}
}
