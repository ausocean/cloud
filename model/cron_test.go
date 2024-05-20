package model

import (
	"testing"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{in: "03:34", want: "34 3 * * *"},
		{in: "03:02", want: "2 3 * * *"},
		{in: "12:04", want: "4 12 * * *"},
		{in: "30 5 4 3 1", want: "30 5 4 3 1"},
	}

	for i, test := range tests {
		var c Cron
		err := c.ParseTime(test.in, 0.0)
		if err != nil {
			t.Fatalf("did not expect error from ParseTime for test no. %d, err: %v", i, err)
		}

		if test.want != c.TOD {
			t.Errorf("unexpected result for test no. %d want: %s got: %s", i, test.want, c.TOD)
		}
	}
}
