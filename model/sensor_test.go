package model

import (
	"bytes"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/ausocean/openfish/datastore"
)

// TestParseArgs tests the helper function parseArgs.
func TestParseArgs(t *testing.T) {
	tests := []struct {
		args string
		n    int
		want []float64
		err  error
	}{
		{args: "3.0,num,2.0", n: 3, want: nil, err: strconv.ErrSyntax},
		{args: "3.0,2.0", n: 3, want: nil, err: ErrUnexpectedArgs},
		{args: "3.0,2.1,6.7", n: 3, want: []float64{3.0, 2.1, 6.7}, err: nil},
		{args: "3.0, 2.1,6.7", n: 3, want: []float64{3.0, 2.1, 6.7}, err: nil},
	}

	for i, test := range tests {
		fltArgs, err := parseArgs(test.args, test.n)
		if !errors.Is(err, test.err) {
			t.Errorf("did not get expected error for test no. %d, \ngot: %v, \nwant: %v", i, err, test.err)
		}

		if !equal(fltArgs, test.want) {
			t.Errorf("did not get expected result for test no. %d, \ngot: %v, \nwant: %v", i, fltArgs, test.want)
		}
	}
}

func equal(a, b []float64) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestTransform tests the Sensor.Transform method.
func TestTransform(t *testing.T) {
	tests := []struct {
		sensor Sensor
		val    float64
		want   float64
		err    error
	}{
		{sensor: Sensor{Func: "afunction", Args: "0.5"}, err: ErrUnrecognisedFunc},
		{sensor: Sensor{Func: keyScale, Args: "0.5"}, val: 20.0, want: 10.0},
		{sensor: Sensor{Func: keyScale, Args: "0.5 "}, val: 20.0, want: 10.0},
		{sensor: Sensor{Func: keyScale, Args: "0.5,9.5"}, err: ErrUnexpectedArgs},
		{sensor: Sensor{Func: keyScale, Args: "num"}, err: strconv.ErrSyntax},
		{sensor: Sensor{Func: keyLinear, Args: "2.0,3.0"}, val: 10.0, want: 23.0},
		{sensor: Sensor{Func: keyLinear, Args: "2.0 ,3.0 "}, val: 10.0, want: 23.0},
		{sensor: Sensor{Func: keyLinear, Args: "2.0,3.0,4.0"}, err: ErrUnexpectedArgs},
		{sensor: Sensor{Func: keyLinear, Args: "2.0,num"}, err: strconv.ErrSyntax},
		{sensor: Sensor{Func: keyQuadratic, Args: "2.0,3.0,4.0"}, val: 2.0, want: 18.0},
		{sensor: Sensor{Func: keyQuadratic, Args: "2.0,3.0 ,4.0"}, val: 2.0, want: 18.0},
		{sensor: Sensor{Func: keyQuadratic, Args: "2.0,3.0"}, err: ErrUnexpectedArgs},
		{sensor: Sensor{Func: keyQuadratic, Args: "2.0,3.0,num"}, err: strconv.ErrSyntax},
		{sensor: Sensor{Func: keyCustom, Args: "(x+10.0)/2.0"}, val: 20.0, want: 15.0},
		{sensor: Sensor{Func: keyCustom, Args: "+_FHK+-="}, err: ErrEvaluableExpression},
		{sensor: Sensor{Func: keyCustom, Args: "asdfasdf"}, err: ErrEvaluate},
	}

	for i, test := range tests {
		got, err := test.sensor.Transform(test.val)
		if !errors.Is(err, test.err) {
			t.Errorf("did not get expected error for test no. %d, \ngot: %v, \nwant: %v", i, err, test.err)
		}

		if got != test.want {
			t.Errorf("did not get expected result for test no. %d, \ngot: %v, \nwant: %v", i, got, test.want)
		}
	}
}

func TestSensorEncode(t *testing.T) {
	tests := []struct {
		Name   string
		Sensor *Sensor
		want   []byte
	}{
		{
			Name:   "default",
			Sensor: &Sensor{SKey: 0, SID: "Test", Pin: "localdevice.S0", Quantity: "AUD", Func: "scale", Args: "0.1, 0.5", Scale: 0.000000, Offset: 0.000000, Units: "unit", Format: "hex"},
			want:   []byte("0\tTest\tlocaldevice.S0\tAUD\tscale\t0.1, 0.5\t0.000000\t0.000000\tunit\thex"),
		},
		{
			Name:   "empty",
			Sensor: &Sensor{},
			want:   []byte("0\t\t\t\t\t\t0.000000\t0.000000\t\t"),
		},
		{
			Name:   "skey only",
			Sensor: &Sensor{SKey: 64},
			want:   []byte("64\t\t\t\t\t\t0.000000\t0.000000\t\t"),
		},
		{
			Name:   "tab seperated args",
			Sensor: &Sensor{SKey: 64, Args: "0.1,	0.5"},
			want:   []byte("64\t\t\t\t\t0.1,	0.5\t0.000000\t0.000000\t\t"),
		},
	}

	for _, test := range tests {
		got := test.Sensor.Encode()
		if !bytes.Equal(got, test.want) {
			t.Errorf("Got\n%s \nWanted\n%s", got, test.want)
		}
	}
}

func TestSensorDecode(t *testing.T) {
	tests := []struct {
		Name    string
		Input   []byte
		Want    *Sensor
		WantErr error
	}{
		{
			Name:    "default",
			Input:   []byte("0\tTest\tlocaldevice.S0\tAUD\tscale\t0.1, 0.5\t0.000000\t0.000000\tunit\thex"),
			Want:    &Sensor{SKey: 0, SID: "Test", Pin: "localdevice.S0", Quantity: "AUD", Func: "scale", Args: "0.1, 0.5", Scale: 0.000000, Offset: 0.000000, Units: "unit", Format: "hex"},
			WantErr: nil,
		},
		{
			Name:    "invalid input",
			Input:   []byte("invalid input"),
			Want:    new(Sensor),
			WantErr: datastore.ErrDecoding,
		},
		{
			Name:    "invalid scale",
			Input:   []byte("0\tTest\tlocaldevice.S0\tAUD\tscale\t0.1, 0.5\tinvalid\t0.000000\tunit\thex"),
			Want:    nil,
			WantErr: datastore.ErrDecoding,
		},
		{
			Name:    "missing value",
			Input:   []byte("0\tTest\tlocaldevice.S0\t\tscale\t0.1, 0.5\t0.000000\t0.000000\tunit\thex"),
			Want:    &Sensor{SKey: 0, SID: "Test", Pin: "localdevice.S0", Quantity: "", Func: "scale", Args: "0.1, 0.5", Scale: 0.000000, Offset: 0.000000, Units: "unit", Format: "hex"},
			WantErr: nil,
		},
		{
			Name:    "missing term",
			Input:   []byte("0\tTest\tlocaldevice.S0\tscale\t0.1, 0.5\t0.000000\t0.000000\tunit\thex"),
			Want:    nil,
			WantErr: datastore.ErrDecoding,
		},
		{
			Name:    "extra term",
			Input:   []byte("0\tTest\tlocaldevice.S0\tscale\t0.1, 0.5\t0.000000\t0.000000\tunit\thex\textra term"),
			Want:    nil,
			WantErr: datastore.ErrDecoding,
		},
		{
			Name:    "empty",
			Input:   []byte(""),
			Want:    nil,
			WantErr: datastore.ErrDecoding,
		},
	}

	for _, test := range tests {
		var got = new(Sensor)
		err := got.Decode(test.Input)
		if err != test.WantErr {
			t.Errorf("%s: unexpected error, wanted: %v got: %v", test.Name, test.WantErr, err)
		} else if !reflect.DeepEqual(got, test.Want) && err == nil {
			t.Errorf("%s: error decoding\nwanted:\n%v\ngot\n%v", test.Name, test.Want, got)
		}
	}

}
