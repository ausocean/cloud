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
  along with Ocean Cron in gpl.txt. If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
)

var cronSpecTests = []struct {
	cron     *model.Cron
	lat, lon float64
	want     string
	wantErr  error
}{
	{
		cron:    &model.Cron{},
		want:    "",
		wantErr: nil,
	},
	{
		cron:    &model.Cron{Enabled: true},
		want:    "",
		wantErr: errNoTimeSpec,
	},
	{
		cron: &model.Cron{TOD: "@sunrise"},
		lat:  1, lon: 1,
		want:    "",
		wantErr: nil,
	},
	{
		cron: &model.Cron{TOD: "@sunrise", Enabled: true},
		lat:  math.NaN(), lon: math.NaN(),
		want:    "",
		wantErr: errNoLocation,
	},
	{
		cron: &model.Cron{TOD: "@sunrise", Enabled: true},
		lat:  1, lon: 1,
		want:    "@sunrise 1 1",
		wantErr: nil,
	},
	{
		cron: &model.Cron{TOD: "@sunrise+1h", Enabled: true},
		lat:  1, lon: 1,
		want:    "@sunrise+1h 1 1",
		wantErr: nil,
	},
	{
		cron: &model.Cron{TOD: "@noon", Enabled: true},
		lat:  1, lon: 1,
		want:    "@noon 1 1",
		wantErr: nil,
	},
	{
		cron: &model.Cron{TOD: "@midnight", Enabled: true},
		lat:  1, lon: 1,
		want:    "@midnight",
		wantErr: nil,
	},
}

func TestCronSpec(t *testing.T) {
	for _, test := range cronSpecTests {
		got, err := cronSpec(test.cron, test.lat, test.lon)
		if fmt.Sprint(err) != fmt.Sprint(test.wantErr) {
			t.Errorf("unexpected error: got:%v want:%v", err, test.wantErr)
		}
		if err != nil {
			continue
		}
		if got != test.want {
			t.Errorf("unexpected cron spec: got:%s want:%s", got, test.want)
		}
	}
}

func TestRun(t *testing.T) {
	// We can't use the normal setup function, which would load production cron jobs.
	ctx := context.Background()
	var err error
	settingsStore, err = datastore.NewStore(ctx, "cloud", "netreceiver", "")
	if err != nil {
		t.Errorf("could not set up datastore: %v", err)
	}
	cronSecret, err = gauth.GetHexSecret(ctx, projectID, "cronSecret")
	if err != nil {
		t.Errorf("could not get cronSecret: %v", err)
	}
	testScheduler, err := newScheduler()
	if err != nil {
		t.Errorf("newScheduler returned error: %v", err)
	}

	// Create and run some crons.
	const url = "https://tv.cloudblue.org/checkbroadcasts"
	cron1 := model.Cron{Skey: 1, ID: "cron1", TOD: "* * * * *", Action: "rpc", Var: url, Enabled: true}
	err = testScheduler.Set(&cron1)
	if err != nil {
		t.Errorf("cronScheduler.Set(%s) returned error: %v", cron1.ID, err)
	}
	cron2 := model.Cron{Skey: 1, ID: "cron2", TOD: "* * * * *", Action: "call", Var: "check", Data: "00:00:00:00:00:01", Enabled: true}
	err = testScheduler.Set(&cron2)
	if err != nil {
		t.Errorf("cronScheduler.Set(%s) returned error: %v", cron2.ID, err)
	}

	testScheduler.run()
}
