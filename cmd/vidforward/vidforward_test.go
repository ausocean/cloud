/*
DESCRIPTION
  Provides testing for vidforward functionality.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package main

import "testing"

// TestIsMac tests the isMac function.
func TestIsMac(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{in: "00:00:00:00:00:00", want: false},
		{in: "00000:00:00:00:01", want: false},
		{in: "00:00:00:00000:01", want: false},
		{in: "15:b5:c7:cg:87:92", want: false},
		{in: "00:00:00:00:00", want: false},
		{in: "7d:ac:cf:84:e8:01", want: true},
	}

	for i, test := range tests {
		got := isMac(test.in)
		if test.want != got {
			t.Errorf("did not get expected result for test %d\ngot: %v\nwant: %v", i, got, test.want)
		}
	}
}
