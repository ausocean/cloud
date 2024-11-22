/*
DESCRIPTION
  dsadmin is a program for performing various datastore admin tasks.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2019-2024 the Australian Ocean Lab (AusOcean).

  This file is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
  or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
  License for more details.

  You should have received a copy of the GNU General Public License in
  gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

// dsadmin is a utility for performing various datastore admin tasks.
//
// Examples:
// To count Site entities:
// - dsadmin --task count --kind Site
//
// To dump Site entities:
// - dsadmin --task dump --kind Site --output sites.json
//
// To copy Site to SiteV2 (preserving the ID key), i.e, to make a backup:
// - dsadmin --task copy --idkey --kind1 Site --kind2 SiteV2
//
// To migrate Site entities (which results in creation of SiteV3 entities):
// - dsadmin --task migrate --kind Site
//
// To delete Site entities.
// - dsadmin --task delete --kind Site
//
// To copy SiteV3 to Site (preserving the ID key), i.e, to complete a migration:
// - dsadmin --task copy --idkey --kind1 SiteV3 --kind2 Site

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/model"
	"github.com/ausocean/openfish/datastore"
	"github.com/ausocean/utils/sliceutils"
)

func main() {
	var task, kind, kind2, ds, ds2, input, output string
	var key int64
	var idKey bool

	flag.StringVar(&task, "task", "", "Datastore task (count, dump, delete, extract, copy or migrate)")
	flag.StringVar(&kind, "kind", "", "Datastore kind")
	flag.StringVar(&kind, "kind1", "", "Datastore kind 1 (same as --kind)")
	flag.StringVar(&kind2, "kind2", "", "Datastore kind 2")
	flag.StringVar(&ds, "ds", "netreceiver", "Datastore (netreceiver or vidgrind)")
	flag.StringVar(&ds2, "ds2", "", "Datastore (netreceiver or vidgrind)")
	flag.StringVar(&input, "input", "", "Input file or file store.")
	flag.StringVar(&output, "output", "output", "Output file or file store")
	flag.Int64Var(&key, "key", 0, "Datastore key, e.g., Skey, MID, etc.")
	flag.BoolVar(&idKey, "idkey", false, "True for and ID key, false for a name key")
	flag.Parse()

	log.SetFlags(0) // Minimise log messages.
	log.SetPrefix("ERROR: ")

	switch ds {
	case "netreceiver", "vidgrind":
		// Do nothing
	default:
		log.Fatal("datastore (-ds) missing or invalid")
	}
	switch ds2 {
	case "netreceiver", "vidgrind", "":
		// Do nothing
	default:
		log.Fatal("datastore (-ds2) invalid")
	}

	if kind == "" {
		log.Fatal("kind missing")
	}

	// Register standard entities.
	model.RegisterEntities()

	// Register non-standard entities used during migrations.
	datastore.RegisterEntity(typeCronV1, func() datastore.Entity { return new(CronV1) })
	datastore.RegisterEntity(typeCronV2, func() datastore.Entity { return new(CronV2) })
	datastore.RegisterEntity(typeSiteV2, func() datastore.Entity { return new(SiteV2) })
	datastore.RegisterEntity(typeSiteV3, func() datastore.Entity { return new(SiteV3) })
	datastore.RegisterEntity(typeSignal, func() datastore.Entity { return new(Signal) })

	var store, store2 datastore.Store
	var err error
	ctx := context.Background()
	if input == "" {
		ev := strings.ToUpper(ds) + "_CREDENTIALS"
		if os.Getenv(ev) == "" {
			log.Fatalf("%s required to access %s", ev, ds)
		}
		fmt.Printf("Reading from cloudstore %s\n", ds)
		store, err = datastore.NewStore(ctx, "cloud", ds, "")

		if err == nil && ds2 != "" {
			fmt.Printf("Writing to cloudstore %s\n", ds2)
			store2, err = datastore.NewStore(ctx, "cloud", ds2, "")
		}
	} else {
		fmt.Printf("Reading from filestore %s\n", input)
		store, err = datastore.NewStore(ctx, "file", ds, input)
	}
	if err != nil {
		log.Fatalf("datastore.NewStore failed with error %v", err)
	}

	switch task {
	case "count":
		err = count(store, kind)

	case "dump":
		err = dump(store, kind, output)

	case "extract":
		switch kind {
		case "MtsMedia":
			err = extractMtsMedia(store, key, output)
		case "Variable":
			err = extractVars(store, key, output)
		default:
			log.Fatalf("invalid kind %s", kind)
		}

	case "delete":
		err = delete(store, kind)

	case "copy":
		if kind == "" || kind2 == "" {
			log.Fatal("copy requires kind and kind2 options")
		}
		err = copy(store, kind, kind2, idKey, key)

	case "migrate":
		// Functions for one-time datastore migrations.
		// Enties an be found in entities.go.
		// Code is retained as a template for future migrations.
		switch kind {
		case "Variable":
			err = migrateDeviceVariables(store)
			if err != nil {
				log.Fatalf("migrateDeviceVariables failed with error: %v", err)
			}
		case "User":
			err = migrateUsers()
			if err != nil {
				log.Fatalf("migrateUsers failed with error: %v", err)
			}
		case "Cron":
			err = migrateCrons()
			if err != nil {
				log.Fatalf("migrateCrons failed with error: %v", err)
			}
		case "Site", "SiteV2", "SiteV3":
			err = migrateSites(store)
			if err != nil {
				log.Fatalf("migrateSites failed with error: %v", err)
			}
		case "Actuator":
			err = migrateActuators(store)
			if err != nil {
				log.Fatalf("migrateActuators failed with error: %v", err)
			}
		case "Sensor":
			err = migrateSensors(store)
			if err != nil {
				log.Fatalf("migrateSensors failed with error: %v", err)
			}
		case "Device":
			err = migrateDevices(store)
			if err != nil {
				log.Fatalf("migrateDevices failed with error: %v", err)
			}
		case "Signal":
			err = migrateSignals(store, store2)
			if err != nil {
				log.Fatalf("migrateSignals failed with error: %v", err)
			}
		default:
			log.Fatalf("invalid kind %s", kind)
		}

	default:
		log.Fatal("invalid task")
	}

	if err != nil {
		log.Fatalf("%s failed with error: %v", task, err)
	}
}

// count counts entities of the given kind.
func count(store datastore.Store, kind string) error {
	ctx := context.Background()

	q := store.NewQuery(kind, true)
	keys, err := store.GetAll(ctx, q, nil)
	if err != nil {
		return err
	}
	fmt.Printf("Counted %d entities of kind %s\n", len(keys), kind)
	return nil
}

// dump dumps entities of the given kind to the supplied file.
func dump(store datastore.Store, kind, file string) error {
	ctx := context.Background()

	q := store.NewQuery(kind, true)
	keys, err := store.GetAll(ctx, q, nil)
	if err != nil {
		return err
	}

	// Create a dump file with one encoded entity per line.
	n := 0
	var data []byte
	for _, k := range keys {
		e, err := datastore.NewEntity(kind)
		if err != nil {
			return err
		}
		err = store.Get(ctx, k, e)
		if err != nil {
			return err
		}

		var encoded []byte

		encodable, ok := e.(datastore.EntityEncoder)
		if ok {
			encoded = encodable.Encode()
		} else {
			encoded, _ = json.Marshal(e)
		}

		data = append(data, encoded...)
		data = append(data, '\n')
		n += 1
	}

	err = os.WriteFile(file, data, 0666)
	if err != nil {
		return err
	}
	fmt.Printf("Dumped %d entities of kind %s to file %s\n", len(keys), kind, file)

	return nil
}

// extractMtsMedia extracts data from MtsMedia entities for a given MID and writes merged data to the supplied file.
func extractMtsMedia(store datastore.Store, mid int64, output string) error {
	ctx := context.Background()

	keys, err := model.GetMtsMediaKeys(ctx, store, mid, nil, nil)
	if err != nil {
		return err
	}

	var data []byte
	for _, k := range keys {
		var m model.MtsMedia
		err := store.Get(ctx, k, &m)
		if err != nil {
			return err
		}
		data = append(data, m.Clip...)
	}

	err = os.WriteFile(output, data, 0666)
	if err != nil {
		return err
	}

	fmt.Printf("Wrote %d MtsMedia for MID %d to file %s\n", len(keys), mid, output)
	return nil
}

// extractVars extracts variables a given site key and writes to the supplied file.
// This is in contrast to dump which dumps all variables.
func extractVars(store datastore.Store, skey int64, output string) error {
	ctx := context.Background()

	vars, err := model.GetVariablesBySite(ctx, store, skey, "")
	if err != nil {
		return err
	}

	var data []byte
	for _, v := range vars {
		data = append(data, v.Encode()...)
		data = append(data, '\n')
	}

	err = os.WriteFile(output, data, 0666)
	if err != nil {
		return err
	}

	fmt.Printf("Wrote %d vars for Skey %d to file %s\n", len(vars), skey, output)
	return nil
}

// delete deletes all entities of the given kind.
func delete(store datastore.Store, kind string) error {
	ctx := context.Background()

	q := store.NewQuery(kind, true)
	keys, err := store.GetAll(ctx, q, nil)
	if err != nil {
		return err
	}
	n := 0
	for sz := len(keys); sz > 0; sz = len(keys) {
		if sz > datastore.MaxKeys {
			sz = datastore.MaxKeys
		}
		err = store.DeleteMulti(ctx, keys[:sz])
		if err != nil {
			return err
		}
		n += sz
		keys = keys[sz:]
	}
	fmt.Printf("Deleted %d entities of kind %s\n", n, kind)
	return nil
}

// copy copies all entities of type kind1 to type kind2. Corresponding
// types must be identical, except for their names. Both entity types
// must be registered with RegisterEntity.
func copy(store datastore.Store, kind1, kind2 string, idKey bool, key int64) error {
	ctx := context.Background()

	q := store.NewQuery(kind1, true)
	keys, err := store.GetAll(ctx, q, nil)
	if err != nil {
		return err
	}
	if idKey {
		fmt.Printf("Copying from %s to %s using ID key\n", kind1, kind2)
	} else {
		fmt.Printf("Copying from %s to %s using name key\n", kind1, kind2)
	}

	n := 0
	for _, k1 := range keys {
		e, err := datastore.NewEntity(kind1)
		if err != nil {
			return fmt.Errorf("NewEntity returned error: %w", err)
		}
		err = store.Get(ctx, k1, e)
		if err != nil {
			return err
		}

		var k2 *datastore.Key
		if idKey {
			k2 = store.IDKey(kind2, k1.ID)
			if key != 0 && key != k1.ID {
				continue
			}
			fmt.Printf("Matched ID key %d\n", key)
		} else {
			k2 = store.NameKey(kind2, k1.Name)
		}

		_, err = store.Put(ctx, k2, e)
		if err != nil {
			return err
		}
		n += 1
	}

	fmt.Printf("Copied %d %s to %s\n", n, kind1, kind2)
	return nil
}

// The following migration functions are retained as examples for how
// to implement future migrations.

// migrateVariables migrates NetReceiver datastore variables, as follows:
// - Rename from Var to Variable.
// - Make property names uppercase.
// - Migrate varsum.x to .varsum.x
//
// Don't bother migrating system vars, which will be repopulated.
// Note: This migration was performed on 24 July 2019.
func migrateVariables() error {
	ctx := context.Background()

	ds, err := datastore.NewStore(ctx, "cloud", "netreceiver", "")
	if err != nil {
		return nil
	}

	q := ds.NewQuery("Var", true) // "Var" is the original type name.
	keys, err := ds.GetAll(ctx, q, nil)
	if err != nil {
		return err
	}
	n := 0
	for _, k := range keys {
		// Don't bother migrating system vars, which will be repopulated.
		if strings.Contains(k.Name, "/dev") {
			continue
		}
		v := new(model.Variable)
		err := ds.Get(ctx, k, v)
		if err != nil {
			return err
		}

		// Migrate varsum variable names from /varsum.x to .varsum.x.
		name := v.Name
		if strings.HasPrefix(name, "/varsum.") {
			name = ".varsum." + name[8:]
			v.Name = name
			v.Scope = ""
			fmt.Printf("Fixed %s\n", name)
		}

		// Re-put the variable as the new type, "Variable".
		// We will delete the original variables later.
		newKey := ds.NameKey("Variable", strconv.Itoa(int(v.Skey))+"."+v.Name)
		v.Updated = time.Now()
		_, err = ds.Put(ctx, newKey, v)
		if err != nil {
			return err
		}
		n += 1
	}
	fmt.Printf("Migrated %d variables\n", n)
	return nil
}

// migrateDeviceVariables replaces device names with hexadecimal MAC addresses.
// The Variable schema is unchanged.
// We don't bother migrating system vars, which will be repopulated.
// Note: This migration was performed on 24 Nov 2023.
func migrateDeviceVariables(store datastore.Store) error {
	ctx := context.Background()

	devNames, devNameToMac, err := getDeviceInfo(ctx, store)
	if err != nil {
		return err
	}

	// Update device variables
	q := store.NewQuery("Variable", false)
	var vars []model.Variable
	_, err = store.GetAll(ctx, q, &vars)
	if err != nil {
		return err
	}

	nVars := 0
	nDeviceVars := 0
	nSysVars := 0
	for _, v := range vars {
		nVars++
		if strings.HasPrefix(v.Scope, "_") {
			// Delete sys var, since it will be re-populated automatically.
			//model.DeleteVariable(ctx, store, v.Skey, v.Name)
			nSysVars++
			continue
		}

		// For device variables, the scope is the device ID.
		// We'll upgrade any such such variables to use the hexadecimal MAC address.
		if sliceutils.ContainsString(devNames, v.Scope) {
			mac := devNameToMac[v.Scope]
			oldName := v.Name
			newName := strings.ReplaceAll(oldName, v.Scope, mac)
			// Update the variable with the new name.
			err := model.PutVariable(ctx, store, v.Skey, newName, v.Value)
			if err != nil {
				fmt.Printf("error putting variable %s: %v", newName, err)
			}
			// Delete the old variable.
			model.DeleteVariable(ctx, store, v.Skey, oldName)
			fmt.Printf("%s => %s\n", oldName, newName)
			nDeviceVars++
		}
	}

	fmt.Printf("Migrated %d device vars, deleted %d sys vars of %d vars\n", nDeviceVars, nSysVars, nVars)
	return nil
}

// getDeviceInfo returns a list of device names and map of device names to MAC addresses.
func getDeviceInfo(ctx context.Context, store datastore.Store) ([]string, map[string]string, error) {
	q := store.NewQuery("DeviceV1", false)
	var devs []DeviceV1
	_, err := store.GetAll(ctx, q, &devs)
	if err != nil {
		return nil, nil, err
	}

	devNames := []string{}
	devNameToMac := map[string]string{}
	nDevices := 0
	for _, dev := range devs {
		devNameToMac[dev.Did] = dev.Hex()
		fmt.Printf("%s => %s\n", dev.Did, devNameToMac[dev.Did])
		devNames = append(devNames, dev.Did)
		nDevices += 1
	}
	fmt.Printf("Found %d devices\n", nDevices)
	return devNames, devNameToMac, nil
}

// migrateUsers migrates Users to NewUsers.
// Note: This migration was performed on 24 Sep 2019.
type NewUser = model.User

func migrateUsers() error {
	ctx := context.Background()

	ds, err := datastore.NewStore(ctx, "cloud", "netreceiver", "")
	if err != nil {
		return nil
	}

	q := ds.NewQuery("User", true)
	keys, err := ds.GetAll(ctx, q, nil)
	if err != nil {
		return err
	}
	n := 0
	for _, k := range keys {
		u := new(model.User)
		err := ds.Get(ctx, k, u)
		if err != nil {
			return err
		}

		fmt.Printf("%d %s\n", u.Skey, u.Email)
		u2 := new(NewUser)
		u2.Skey = u.Skey
		u2.Email = u.Email
		u2.UserID = u.UserID
		u2.Perm = u.Perm
		u2.Created = u.Created
		var k2 = ds.NameKey("NewUser", k.Name)
		_, err = ds.Put(ctx, k2, u2)
		if err != nil {
			return err
		}
		n += 1
	}
	fmt.Printf("Migrated %d users\n", n)
	return nil
}

// migrateCrons migrates crons
// Note: This migration was performed on 23 April 2021.
func migrateCrons() error {
	ctx := context.Background()

	ds, err := datastore.NewStore(ctx, "cloud", "netreceiver", "")
	if err != nil {
		return nil
	}

	q := ds.NewQuery("Cron", true)
	keys, err := ds.GetAll(ctx, q, nil)
	if err != nil {
		return err
	}
	n := 0
	for _, k := range keys {
		c := new(CronV1)
		err := ds.Get(ctx, k, c)
		if err != nil {
			return err
		}

		fmt.Printf("%d %s\n", c.Skey, c.ID)
		c2 := new(model.Cron)
		c2.Skey = c.Skey
		c2.ID = c.ID
		c2.Time = c.Time
		c2.TOD = c.TOD
		c2.Repeat = c.Repeat
		c2.Minutes = c.Minutes
		c2.Action = c.Action
		c2.Var = c.Var
		c2.Action = c.Action
		c2.Data = c.Data
		c2.Enabled = c.Enabled
		var k2 = ds.NameKey("NewCron", k.Name)
		_, err = ds.Put(ctx, k2, c2)
		if err != nil {
			return err
		}
		n += 1
	}
	fmt.Printf("Migrated %d crons\n", n)
	return nil
}

// migrateSites migrates sites from kind1, usually Site, to kind2.
// Notea:
//   - The migration from SiteV1 to SiteV2 was performed on 31 July 2023.
//   - The migration from SiteV2 to SiteV3 was performed on 8 July 2024.
func migrateSites(store datastore.Store) error {
	ctx := context.Background()

	q := store.NewQuery("Site", true)
	keys, err := store.GetAll(ctx, q, nil)
	if err != nil {
		return err
	}
	n := 0
	for _, k := range keys {
		s := new(model.Site)
		err := store.Get(ctx, k, s)
		if err != nil {
			return err
		}

		fmt.Printf("%d %s\n", s.Skey, s.Name)
		s2 := new(SiteV3) // Change as required.

		// Customize the following as required.
		s2.Skey = s.Skey
		s2.Name = s.Name
		s2.Description = ""
		s2.OrgID = "AusOcean"
		s2.OwnerEmail = s.OwnerEmail
		s2.OpsEmail = "ops@ausocean.org"
		s2.YouTubeEmail = "social@ausocean.org"
		s2.Latitude = s.Latitude
		s2.Longitude = s.Longitude
		s2.Timezone = s.Timezone
		s2.NotifyPeriod = s.NotifyPeriod
		s2.Enabled = s.Enabled
		s2.Confirmed = s.Confirmed
		s2.Premium = s.Premium
		s2.Public = s.Public
		s2.Created = s.Created

		var k2 = store.IDKey("SiteV3", s2.Skey)
		_, err = store.Put(ctx, k2, s2)
		if err != nil {
			return err
		}
		n += 1
	}
	fmt.Printf("Migrated %d sites\n", n)
	return nil
}

// Actuator and Sensor migration.

// migrateActuators migrates Actuator entities to ActuatorV2.
func migrateActuators(store datastore.Store) error {
	ctx := context.Background()

	_, devNameToMac, err := getDeviceInfo(ctx, store)
	if err != nil {
		return err
	}

	q := store.NewQuery("Actuator", false)
	var acts []model.Actuator
	_, err = store.GetAll(ctx, q, &acts)
	if err != nil {
		return err
	}

	nActs := 0
	for _, act := range acts {
		nActs++

		v := strings.Split(act.Pin, ".")
		if len(v) != 2 {
			fmt.Printf("actuator %s has malformed pin %s", act.AID, act.Pin)
			continue
		}

		devID := v[0]
		pin := v[1]
		mac := devNameToMac[devID]

		act2 := new(model.ActuatorV2)
		act2.Name = act.AID
		act2.Mac = model.MacEncode(mac)
		act2.Pin = pin
		// Strip the device ID from the variable, since ActuatorV2 variable names are relative to the device.
		idx := strings.Index(act.Var, ".")
		if idx > 0 {
			act2.Var = act.Var[idx+1:]
		} else {
			act2.Var = act.Var
		}
		model.PutActuatorV2(ctx, store, act2)
		fmt.Printf("%s => %d.%s\n", act.AID, act2.Mac, act2.Pin)
	}

	fmt.Printf("Migrated %d actuators\n", nActs)
	return nil
}

// migrateSensors migrates Sensor entities to SensorV2.
func migrateSensors(store datastore.Store) error {
	ctx := context.Background()

	_, devNameToMac, err := getDeviceInfo(ctx, store)
	if err != nil {
		return err
	}

	q := store.NewQuery("Sensor", false)
	var sens []model.Sensor
	_, err = store.GetAll(ctx, q, &sens)
	if err != nil {
		return err
	}

	nSens := 0
	for _, sen := range sens {
		nSens++

		v := strings.Split(sen.Pin, ".")
		if len(v) != 2 {
			fmt.Printf("sensor %s has malformed pin %s", sen.SID, sen.Pin)
			continue
		}

		devID := v[0]
		pin := v[1]
		mac := devNameToMac[devID]

		sen2 := new(model.SensorV2)
		sen2.Name = sen.SID
		sen2.Mac = model.MacEncode(mac)
		sen2.Pin = pin
		sen2.Quantity = sen.Quantity
		sen2.Func = sen.Func
		sen2.Args = sen.Args
		sen2.Scale = sen.Scale
		sen2.Offset = sen.Offset
		sen2.Units = sen.Units
		sen2.Format = sen.Format
		model.PutSensorV2(ctx, store, sen2)
		fmt.Printf("%s => %d.%s\n", sen.SID, sen2.Mac, sen2.Pin)
	}

	fmt.Printf("Migrated %d sensors\n", nSens)
	return nil
}

// migrateDevices replaces Device.Did with Device.Name.
func migrateDevices(store datastore.Store) error {
	ctx := context.Background()

	q := store.NewQuery("DeviceV1", false)
	var devs []DeviceV1
	_, err := store.GetAll(ctx, q, &devs)
	if err != nil {
		return err
	}

	n := 0
	for _, dev := range devs {
		n += 1
		dev2 := new(DeviceV2)
		dev2.Mac = dev.Mac
		dev2.Name = dev.Did
		dev2.Skey = dev.Skey
		dev2.Dkey = dev.Dkey
		dev2.Inputs = dev.Inputs
		dev2.Outputs = dev.Outputs
		dev2.Wifi = dev.Wifi
		dev2.MonitorPeriod = dev.MonitorPeriod
		dev2.ActPeriod = dev.ActPeriod
		dev2.Type = dev.Type
		dev2.Version = dev.Version
		dev2.Protocol = dev.Protocol
		dev2.Status = dev.Status
		dev2.Latitude = dev.Latitude
		dev2.Longitude = dev.Longitude
		dev2.Enabled = dev.Enabled
		dev2.Updated = dev.Updated
		var k = store.IDKey(typeDeviceV2, dev2.Mac)
		_, err = store.Put(ctx, k, dev2)
		if err != nil {
			return err
		}

	}
	fmt.Printf("Migrated %d devices\n", n)
	return nil
}

// migrateSignals migrates a range of signals, specified below.
func migrateSignals(store, store2 datastore.Store) error {
	ctx := context.Background()

	ma := "BC:DD:C2:2B:AD:6D"
	mac := model.MacEncode(ma)
	pin := "A0"
	from := time.Time(time.Date(2023, 7, 1, 0, 0, 0, 0, time.UTC))
	to := time.Time(time.Date(2023, 7, 31, 0, 0, 0, 0, time.UTC))

	fmt.Printf("ma=%s, pin=%s, from=%v, to=%v\n", ma, pin, from, to)

	q := store.NewQuery(typeSignal, false)
	q.Filter("mac =", mac)
	q.Filter("pin =", pin)
	q.Filter("date >", from)
	q.Filter("date <=", to)

	var signals []Signal
	_, err := store.GetAll(ctx, q, &signals)
	if err != nil {
		return err
	}

	id := model.ToSID(ma, pin)
	n := 0
	for _, s := range signals {
		n += 1
		s2 := new(model.Scalar)
		s2.ID = id
		s2.Timestamp = s.Date.Unix()
		s2.Value = float64(s.Value)

		err = model.PutScalar(ctx, store2, s2)
		if err != nil {
			return err
		}
	}

	fmt.Printf("Migrated %d signals\n", n)
	return nil
}
