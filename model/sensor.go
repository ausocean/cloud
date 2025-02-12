/*
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2022 the Australian Ocean Lab (AusOcean).

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
  or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
  License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Knetic/govaluate"
	"github.com/ausocean/openfish/datastore"
	"github.com/ausocean/utils/nmea"
)

// The entitiy type as named in the datastore.
const (
	typeSensor   = "Sensor"
	typeSensorV2 = "SensorV2"
)

// Number of arguments for each function type.
const (
	nArgsScale     = 1
	nArgsLinear    = 2
	nArgsQuadratic = 3
)

// Func defines the function used to calculate the value of the sensor.
type Func string

// The keys for the function types.
// See also SensorFuncs below.
const (
	funcNone      Func = "none"
	funcScale     Func = "scale"
	funcLinear    Func = "linear"
	funcQuadratic Func = "quadratic"
	funcCustom    Func = "custom"
)

// Format defines the formatting on a sensor.
type Format string

// The keys for sensor formats.
// See also SensorFormats below.
const (
	formBool    Format = "bool"
	formRound   Format = "round"
	formRound1  Format = "round1"
	formRound2  Format = "round2"
	formCompass Format = "compass"
	formHex     Format = "hex"
)

// Pin describes a physical or virtual pin on a device to
// attach a sensor.
type Pin string

// The pins used by sensors.
const (
	pinBatteryVoltage      Pin = "A0"
	pinAnalogValue         Pin = "X10"
	pinAirTemperature      Pin = "X50"
	pinHumidity            Pin = "X51"
	pinWaterTemperature    Pin = "X60"
	pinESP32BatteryVoltage Pin = "A4"
	pinESP32Power1Voltage  Pin = "A26"
	pinESP32Power2Voltage  Pin = "A27"
	pinESP32Power3Voltage  Pin = "A14"
	pinESP32NetworkVoltage Pin = "A15"
	pinESP32Current        Pin = "A2"
)

// Unit defines a unit of measurement used by a sensor.
type Unit string

// The units used by sensors.
const (
	unitCelsius   Unit = "C"
	unitPercent   Unit = "%"
	unitVoltage   Unit = "V"
	unitMilliamps Unit = "mA"
)

// Arg defines a float64 value which comprises the arguments to a sensor.
type Arg float64

// Exported errors.
var (
	ErrUnexpectedArgs      = errors.New("unexpected number of args")
	ErrArgParse            = errors.New("could not parse argument")
	ErrUnrecognisedFunc    = errors.New("unrecognised function key")
	ErrEvaluableExpression = &errEvaluableExpression{}
	ErrEvaluate            = &errEvaluate{}
)

// errEvaluableExpression is intended to wrap any error from govaluate.NewEvaluableExpression
// and implements the Is method so that we can check for this error using errors.Is.
type errEvaluableExpression struct{ err error }

func (e *errEvaluableExpression) Error() string {
	msg := "could not create evaluable expression from args string"
	if e.err == nil {
		return msg
	}
	return fmt.Sprintf("%s: %s", msg, e.err.Error())
}
func (e *errEvaluableExpression) Is(err error) bool {
	_, ok := err.(*errEvaluableExpression)
	return ok
}

// errEvaluate is intended to wrap any error from govaluate.EvaluableExpression.Evaluate
// and implements the Is method so that we can check for this kind of error using
// errors.Is.
type errEvaluate struct{ err error }

func (e *errEvaluate) Error() string {
	msg := "could not evaluate expression"
	if e.err == nil {
		return msg
	}
	return fmt.Sprintf("%s: %s", msg, e.err.Error())
}
func (e *errEvaluate) Is(err error) bool {
	_, ok := err.(*errEvaluate)
	return ok
}

// funcs holds a map of func names to the operations to be performed.
var funcs = map[string]func(x float64, args string) (float64, error){
	string(funcScale): func(x float64, args string) (float64, error) {
		argFlts, err := parseArgs(args, nArgsScale)
		if err != nil {
			return 0.0, err
		}
		return x * argFlts[0], nil
	},

	string(funcLinear): func(x float64, args string) (float64, error) {
		argFlts, err := parseArgs(args, nArgsLinear)
		if err != nil {
			return 0.0, err
		}
		return x*argFlts[0] + argFlts[1], nil
	},

	string(funcQuadratic): func(x float64, args string) (float64, error) {
		argFlts, err := parseArgs(args, nArgsQuadratic)
		if err != nil {
			return 0.0, err
		}
		return argFlts[0]*math.Pow(x, 2) + argFlts[1]*x + argFlts[2], nil
	},

	// It is expected that the args string contains the custom function, and only
	// the custom function, otherwise errors will likely follow from govaluate.
	// The value to be transformed is represented by x e.g. (x + 3)/2.
	string(funcCustom): func(x float64, args string) (float64, error) {
		exp, err := govaluate.NewEvaluableExpression(args)
		if err != nil {
			return 0.0, &errEvaluableExpression{err}
		}
		params := map[string]interface{}{"x": x}

		res, err := exp.Evaluate(params)
		if err != nil {
			return 0.0, &errEvaluate{err}
		}
		return res.(float64), nil
	},
}

// parseArgs is used in the func maps above to firstly check that the user has
// provided the correct number of CSV values given the type of function selected,
// and then secondly creates a float64 slice to house the parsed out numerical
// args to be passed into the selected function.
func parseArgs(args string, n int) ([]float64, error) {
	args = strings.ReplaceAll(args, " ", "")
	split := strings.Split(args, ",")
	if len(split) != n {
		return nil, fmt.Errorf("%w, got: %d, want: %d", ErrUnexpectedArgs, len(split), n)
	}
	argFlts := make([]float64, n)
	for i, v := range split {
		vf, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("could not parse arg no. %d: %w", i, err)
		}
		argFlts[i] = vf
	}
	return argFlts, nil
}

// Sensor repsents a sensor datastore type. Sensors take raw measured
// values and transform them based on a few predefined functions, or
// by a custom function specified by a string expression.
// NB: This type is deprecated. See SensorV2.
type Sensor struct {
	SKey     int64   `datastore:"skey"`
	SID      string  `datastore:"sid"`
	Pin      string  `datastore:"pin"`
	Quantity string  `datastore:"quantity"`
	Func     string  `datastore:"func"`
	Args     string  `datastore:"args"`
	Scale    float64 `datastore:"scale"`
	Offset   float64 `datastore:"offset"`
	Units    string  `datastore:"units"`
	Format   string  `datastore:"format"`
}

// Transform applies the transformation specified in s.Func to the passed value v.
func (s *Sensor) Transform(v float64) (float64, error) {
	if s.Func == "" || s.Func == "none" || s.Func == "None" {
		return v, nil
	}
	f, ok := funcs[s.Func]
	if !ok {
		return 0.0, ErrUnrecognisedFunc
	}
	res, err := f(v, s.Args)
	if err != nil {
		return 0.0, fmt.Errorf("could not transform value: %w", err)
	}
	return res, nil
}

// Encode serializes a Sensor into tab-separated values.
func (s *Sensor) Encode() []byte {
	return []byte(fmt.Sprintf("%d\t%s\t%s\t%s\t%s\t%s\t%f\t%f\t%s\t%s", s.SKey, s.SID, s.Pin, s.Quantity, s.Func, s.Args, s.Scale, s.Offset, s.Units, s.Format))
}

// Decode deserializes a Sensor from tab-separated values.
func (s *Sensor) Decode(b []byte) error {
	p := strings.Split(string(b), "\t")
	if len(p) != 10 {
		return datastore.ErrDecoding
	}
	var err error
	s.SKey, err = strconv.ParseInt(p[0], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	s.SID = p[1]
	s.Pin = p[2]
	s.Quantity = p[3]
	s.Func = p[4]
	s.Args = p[5]
	s.Scale, err = strconv.ParseFloat(p[6], 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	s.Offset, err = strconv.ParseFloat(p[7], 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	s.Units = p[8]
	s.Format = p[9]
	return nil
}

// Copy is not currently implemented.
func (s *Sensor) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

// GetCache returns nil, indicating no caching.
func (s *Sensor) GetCache() datastore.Cache {
	return nil
}

// SensorFuncs returns a list of the funcs that can be selected for value
// transformation through Sensor entities.
func SensorFuncs() []string {
	return []string{string(funcNone), string(funcScale), string(funcLinear), string(funcQuadratic), string(funcCustom)}
}

// SensorFormats returns a list of available formats for formatting sensor values.
func SensorFormats() []string {
	return []string{string(formBool), string(formRound), string(formRound1), string(formRound2), string(formCompass), string(formHex)}
}

// GetSensor gets a Sensor from the datastore given the site key and sensor id.
func GetSensor(ctx context.Context, store datastore.Store, sKey int64, sid string) (*Sensor, error) {
	key := store.NameKey(typeSensor, strconv.Itoa(int(sKey))+"."+sid)
	var sensor *Sensor
	err := store.Get(ctx, key, sensor)
	if err != nil {
		return nil, err
	}
	return sensor, nil
}

// PutSensor puts a Sensor entity into the datastore.
func PutSensor(ctx context.Context, store datastore.Store, s *Sensor) error {
	key := store.NameKey(typeSensor, strconv.FormatInt(s.SKey, 10)+"."+s.SID)
	_, err := store.Put(ctx, key, s)
	return err
}

// DeleteSensor deletes a Sensor entity given the site key and sensor ID.
func DeleteSensor(ctx context.Context, store datastore.Store, sKey int64, sid string) error {
	key := store.NameKey(typeSensor, strconv.FormatInt(sKey, 10)+"."+sid)
	err := store.DeleteMulti(ctx, []*datastore.Key{key})
	return err
}

// GetSensorByPin gets a Sensor from the datastore given the site key and the
// pin for which the sensor is applied.
func GetSensorByPin(ctx context.Context, store datastore.Store, sKey int64, did, pin string) (*Sensor, error) {
	q := store.NewQuery(typeSensor, false)
	q.Filter("skey =", sKey)
	q.Filter("pin =", did+"."+pin)
	var sensors []Sensor
	_, err := store.GetAll(ctx, q, &sensors)
	if err != nil {
		return nil, fmt.Errorf("could not get sensors: %w", err)
	}

	if len(sensors) == 0 {
		return nil, datastore.ErrNoSuchEntity
	}

	if len(sensors) > 1 {
		return nil, fmt.Errorf("unexpected number of sensors found, wanted 1 but got: %d", len(sensors))
	}

	return &sensors[0], nil
}

// GetSensorsBySite returns sensors associated with a site optionally filter by
// device with the provided device ID.
func GetSensorsBySite(ctx context.Context, store datastore.Store, skey int64, devID string) ([]Sensor, error) {
	q := store.NewQuery(typeSensor, false, "skey", "sid")
	q.Filter("skey =", skey)
	q.Order("sid")
	var all, toOut []Sensor
	_, err := store.GetAll(ctx, q, &all)
	if err != nil {
		return nil, err
	}

	if devID == "" {
		return all, nil
	}

	for _, s := range all {
		if strings.Split(s.Pin, ".")[0] == devID {
			toOut = append(toOut, s)
		}
	}
	return toOut, nil
}

// SensorV2 defines a version 2 sensor. A sensor formats the value
// obtained from a device input pin. The key is the MAC address
// concatenated with the pin. Version 2 sensors do not have a site
// key, but are linked to a site indirectly via their device.
type SensorV2 struct {
	Name     string  // Name of sensor (mutable).
	Mac      int64   // MAC address of associated device (immutable).
	Pin      string  // Pin of associated device (immutable).
	Quantity string  // NMEA quantity code.
	Func     string  // Transformation function.
	Args     string  // Transformation args.
	Scale    float64 // Deprecated.
	Offset   float64 // Deprecated.
	Units    string  // Units of transformed value.
	Format   string  // Format of transformed value.
}

// Encode encodes a sensor as JSON.
func (s *SensorV2) Encode() []byte {
	bytes, _ := json.Marshal(s)
	return bytes
}

// Decode decodes a sensor from JSON.
func (s *SensorV2) Decode(b []byte) error {
	return json.Unmarshal(b, s)
}

// Copy copies a sensor to dst, or returns a copy of the sensor when dst is nil.
func (s *SensorV2) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var s2 *SensorV2
	if dst == nil {
		s2 = new(SensorV2)
	} else {
		var ok bool
		s2, ok = dst.(*SensorV2)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*s2 = *s
	return s2, nil
}

var sensorCache datastore.Cache = datastore.NewEntityCache()

// GetCache returns the sensor cache.
func (s *SensorV2) GetCache() datastore.Cache {
	return sensorCache
}

// Transform applies the transformation specified in s.Func to the passed value v.
func (s *SensorV2) Transform(v float64) (float64, error) {
	if s.Func == "" || s.Func == "none" || s.Func == "None" {
		return v, nil
	}
	f, ok := funcs[s.Func]
	if !ok {
		return 0.0, ErrUnrecognisedFunc
	}
	res, err := f(v, s.Args)
	if err != nil {
		return 0.0, fmt.Errorf("could not transform value: %w", err)
	}
	return res, nil
}

// PutSensorV2 creates/updates a sensor.
func PutSensorV2(ctx context.Context, store datastore.Store, s *SensorV2) error {
	k := store.NameKey(typeSensorV2, strconv.FormatInt(s.Mac, 10)+"."+s.Pin)
	_, err := store.Put(ctx, k, s)
	return err
}

// GetSensorV2 gets a sensor.
func GetSensorV2(ctx context.Context, store datastore.Store, mac int64, pin string) (*SensorV2, error) {
	k := store.NameKey(typeSensorV2, strconv.FormatInt(mac, 10)+"."+pin)
	s := new(SensorV2)
	err := store.Get(ctx, k, s)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// GetSensorsV2 gets all sensors for a device.
func GetSensorsV2(ctx context.Context, store datastore.Store, mac int64) ([]SensorV2, error) {
	q := store.NewQuery(typeSensorV2, false, "Mac", "Pin")
	q.Filter("Mac =", mac)
	var sensors []SensorV2
	_, err := store.GetAll(ctx, q, &sensors)
	return sensors, err
}

// DeleteSensorV2 deletes a sensor.
func DeleteSensorV2(ctx context.Context, store datastore.Store, mac int64, pin string) error {
	k := store.NameKey(typeSensorV2, strconv.FormatInt(mac, 10)+"."+pin)
	return store.Delete(ctx, k)
}

// shim is a thin shim between typed values and a sensor.
func sensorShim(name string, pin Pin, qty nmea.Code, function Func, units Unit, format Format, args ...Arg) *SensorV2 {
	return &SensorV2{
		Name:     name,
		Pin:      string(pin),
		Quantity: string(qty),
		Func:     string(function),
		Args:     catArgs(args...),
		Units:    string(units),
		Format:   string(format),
	}
}

// catArgs concatenates the passed args in a comma separated string
// (using the least possible decimal places)
func catArgs(args ...Arg) string {
	str := ""
	for i, arg := range args {
		if i > 0 {
			str += ","
		}
		str += strconv.FormatFloat(float64(arg), 'f', -1, 64)
	}
	return str
}

// GetSensorValue gets the latest transformed value for a sensor.
func GetSensorValue(ctx context.Context, store datastore.Store, sensor *SensorV2) (float64, error) {
	id := ToSID(MacDecode(sensor.Mac), sensor.Pin)
	scalar, err := GetLatestScalar(ctx, store, id)
	if err != nil {
		return 0.0, fmt.Errorf("could not get latest scalar: %w", err)
	}
	value, err := sensor.Transform(scalar.Value)
	if err != nil {
		return 0.0, fmt.Errorf("could not transform scalar: %w", err)
	}
	return value, nil
}
