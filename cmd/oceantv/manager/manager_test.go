package manager

import "testing"

// TestRemoveDate tests the removeDate helper function.
func TestRemoveDate(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "A Broadcast 23/04/15", want: "A Broadcast "},
		{in: "A Broadcast 04/23/15", want: "A Broadcast "},
		{in: "ABroadcast04/23/15", want: "ABroadcast"},
		{in: "ABroadcast04/23/15AStream", want: "ABroadcastAStream"},
	}

	for i, test := range tests {
		got := removeDate(test.in)
		if got != test.want {
			t.Errorf("did not get expected result for test no. %d \ngot: %s \nwant: %s", i, got, test.want)
		}
	}
}
