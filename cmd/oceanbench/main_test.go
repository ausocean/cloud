/*
AUTHORS
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2018-2024 the Australian Ocean Lab (AusOcean)

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

package main

import (
	"testing"

	"github.com/ausocean/cloud/model"
)

func TestConfigJSON(t *testing.T) {
	// Test case 1: device key is not provided.
	dev1 := &model.Device{
		Mac:           model.MacEncode("00:11:22:33:44:55"),
		Wifi:          "SSID,PASS",
		Inputs:        "S0,T0",
		Outputs:       "D1,D2",
		MonitorPeriod: 60,
		ActPeriod:     60,
		Version:       "1.0.0",
	}
	var vs1 int64 = 1
	dk1 := ""
	expectedJSON1 := `{"ma":"00:11:22:33:44:55","wi":"SSID,PASS","ip":"S0,T0","op":"D1,D2","mp":60,"ap":60,"cv":"1.0.0","vs":1}`

	result1, err := configJSON(dev1, vs1, dk1)
	if err != nil {
		t.Errorf("Test case 1 failed. Error: %v", err)
	} else if result1 != expectedJSON1 {
		t.Errorf("Test case 1 failed. Expected %s, but got %s", expectedJSON1, result1)
	}

	// Test case 2: device key is provided.
	dev2 := &model.Device{
		Mac:           model.MacEncode("00:11:22:33:44:55"),
		Wifi:          "SSID,PASS",
		Inputs:        "V0,T0",
		Outputs:       "X1,D2",
		MonitorPeriod: 120,
		ActPeriod:     120,
		Version:       "12.2.2",
	}
	var vs2 int64 = 2
	dk2 := "10"
	expectedJSON2 := `{"ma":"00:11:22:33:44:55","wi":"SSID,PASS","ip":"V0,T0","op":"X1,D2","mp":120,"ap":120,"cv":"12.2.2","vs":2,"dk":"10"}`

	result2, err := configJSON(dev2, vs2, dk2)
	if err != nil {
		t.Errorf("Test case 2 failed. Error: %v", err)
	} else if result2 != expectedJSON2 {
		t.Errorf("Test case 2 failed. Expected %s, but got %s", expectedJSON2, result2)
	}
}
