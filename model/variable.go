/*
DESCRIPTION
  Variable datastore type and functions.

AUTHORS
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2019 the Australian Ocean Lab (AusOcean).

  This is free software: you can redistribute it and/or modify it
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

package model

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/openfish/datastore"
)

const typeVariable = "Variable" // Variable datastore type.

// Variable represents a cloud variable, which stores abitrary
// string information. When the variable name includes a period, the
// portion to the left of the dot represents the scope. The scope is
// used to create namespaces, e.g., <MAC>.<var> represents a
// variable for a given device, whereas <var> represents a global
// variable for the entire site. Variables that start with an underscore,
// e.g., _<var>, are system variables which are typically hidden from
// users.
type Variable struct {
	Skey    int64     // Site key.
	Scope   string    // Scope, if any.
	Name    string    // Variable name, which is any ID.
	Value   string    `datastore:",noindex"` // Variable value.
	Updated time.Time // Date/time last updated.
}

// VarType holds the type information about a variable with a given name.
type VarType struct {
	Name string
	Type string
}

// Variable types.
const (
	VarTypeInt    = "int"
	VarTypeUint   = "uint"
	VarTypeFloat  = "float"
	VarTypeBool   = "bool"
	VarTypeString = "string"
)

// Encode serializes a Variable into tab-separated values.
func (v *Variable) Encode() []byte {
	return []byte(fmt.Sprintf("%d\t%s\t%s\t%s\t%d", v.Skey, v.Scope, v.Name, v.Value, v.Updated.Unix()))
}

// Decode deserializes a Variable from tab-separated values.
func (v *Variable) Decode(b []byte) error {
	p := strings.Split(string(b), "\t")
	if len(p) != 5 {
		return datastore.ErrDecoding
	}
	var err error
	v.Skey, err = strconv.ParseInt(p[0], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	v.Scope = p[1]
	v.Name = p[2]
	v.Value = p[3]
	ts, err := strconv.ParseInt(p[4], 10, 64)
	if err != nil {
		return datastore.ErrDecoding
	}
	v.Updated = time.Unix(ts, 0)
	return nil
}

// Copy is not currently implemented.
func (v *Variable) Copy(datastore.Entity) (datastore.Entity, error) {
	return nil, datastore.ErrUnimplemented
}

// GetCache returns nil, indicating no caching.
func (v *Variable) GetCache() datastore.Cache {
	return nil
}

// Basename returns the name of the variable without the scope.
func (v *Variable) Basename() string {
	parts := strings.SplitN(v.Name, ".", 2)
	return parts[len(parts)-1]
}

// IsSystemVariable returns true if the variable is a system variable, false otherwise.
func (v *Variable) IsSystemVariable() bool {
	if v.Name[0] == '_' {
		return true
	} else {
		return false
	}
}

// IsLink returns true if the variable is an HTTP or HTTPS URL.
func (v *Variable) IsLink() bool {
	if strings.HasPrefix(v.Value, "http://") || strings.HasPrefix(v.Value, "https://") {
		return true
	} else {
		return false
	}
}

// PutVariable creates or updates a variable
// Strip any colons from the scope.
func PutVariable(ctx context.Context, store datastore.Store, skey int64, name, value string) error {
	sep := strings.Index(name, ".")
	scope := ""
	if sep >= 0 {
		scope = strings.ReplaceAll(name[:sep], ":", "")
		name = scope + name[sep:]
	}
	v := &Variable{Skey: skey, Name: name, Scope: scope, Value: value, Updated: time.Now()}
	key := store.NameKey(typeVariable, strconv.FormatInt(skey, 10)+"."+name)
	_, err := store.Put(ctx, key, v)
	if err == nil {
		invalidateVarSum(ctx, store, skey, name)
	}
	return err
}

// PutVariableInTransaction updates or creates a variable in the datastore atomically.
// First it will try to update, if that fails, it will create, then try to update again.
//
// Argument updateFunc is a function that takes the current value of the variable and returns a new value.
// It is used to atomically update the variable's value in the datastore. The function should return both the updated value and any error
// encountered during the update logic.
func PutVariableInTransaction(ctx context.Context, store datastore.Store, skey int64, name string, updateFunc func(currentValue string) string) error {
	// If a dot exists in the name, that indicates scope, remove any colons from the scope.
	sep := strings.Index(name, ".")
	scope := ""
	if sep >= 0 {
		scope = strings.ReplaceAll(name[:sep], ":", "")
		name = scope + name[sep:]
	}

	key := store.NameKey(typeVariable, strconv.FormatInt(skey, 10)+"."+name)
	var variable Variable

update:
	err := store.Update(ctx, key, func(entity datastore.Entity) {
		if v, ok := entity.(*Variable); ok {
			v.Value = updateFunc(v.Value)
			v.Updated = time.Now()
		}
	}, &variable)
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		// The variable doesn't exist, initialize it with an empty value.
		variable = Variable{
			Skey:    skey,
			Name:    name,
			Scope:   scope,
			Value:   "",
			Updated: time.Now(),
		}

		// Create the variable in the datastore.
		err := store.Create(ctx, key, &variable)
		if errors.Is(err, datastore.ErrEntityExists) {
			// Do nothing.
		} else if err != nil {
			return fmt.Errorf("failed to create variable: %w", err)
		}
		goto update
	}

	if err != nil {
		return fmt.Errorf("failed to update variable: %w", err)
	}

	if cache := variable.GetCache(); cache != nil {
		cache.Set(key, &variable)
	}

	return nil
}

// GetVariable gets a variable.
// Ignore colons in the scope.
func GetVariable(ctx context.Context, store datastore.Store, skey int64, name string) (*Variable, error) {
	sep := strings.Index(name, ".")
	if sep >= 0 {
		name = strings.ReplaceAll(name[:sep], ":", "") + name[sep:]
	}
	key := store.NameKey(typeVariable, strconv.FormatInt(skey, 10)+"."+name)
	v := new(Variable)
	return v, store.Get(ctx, key, v)
}

// GetVariablesBySite returns all the variables for a given site, optionally filtered by scope.
// Ignore colons in the scope.
func GetVariablesBySite(ctx context.Context, store datastore.Store, skey int64, scope string) ([]Variable, error) {
	var q datastore.Query
	if scope != "" {
		scope = strings.ReplaceAll(scope, ":", "")
		q = store.NewQuery(typeVariable, false, "Skey", "Scope", "Name")
		q.Filter("Skey =", skey)
		q.Filter("Scope =", scope)
	} else {
		q = store.NewQuery(typeVariable, false, "Skey", "Name")
		q.Filter("Skey =", skey)
	}
	q.Order("Name")
	var vars []Variable
	_, err := store.GetAll(ctx, q, &vars)
	return vars, err
}

// DeleteVariable deletes a variable.
// Ignore colons in the scope.
func DeleteVariable(ctx context.Context, store datastore.Store, skey int64, name string) error {
	sep := strings.Index(name, ".")
	if sep >= 0 {
		scope := strings.ReplaceAll(name[:sep], ":", "")
		name = scope + name[sep:]
	}
	key := store.NameKey(typeVariable, strconv.FormatInt(skey, 10)+"."+name)
	err := store.DeleteMulti(ctx, []*datastore.Key{key})
	if err == nil {
		invalidateVarSum(ctx, store, skey, name)
	}
	return err
}

// DeleteVariables deletes all variables for a given scope.
// Ignore colons in the scope.
func DeleteVariables(ctx context.Context, store datastore.Store, skey int64, scope string) error {
	scope = strings.ReplaceAll(scope, ":", "")
	q := store.NewQuery(typeVariable, true, "Skey", "Scope", "Name")
	q.Filter("Skey =", skey)
	q.Filter("Scope =", scope)
	keys, err := store.GetAll(ctx, q, nil)
	if err != nil {
		return err
	}
	return store.DeleteMulti(ctx, keys)
}

// GetBroadcastVarByUUID gets the variable associated with a given broadcast UUID.
func GetBroadcastVarByUUID(ctx context.Context, store datastore.Store, uuid string) (*Variable, error) {
	const broadcastScope = "Broadcast"
	q := store.NewQuery(typeVariable, false)
	q.FilterField("Scope", "=", broadcastScope)
	q.FilterField("Name", "=", broadcastScope+"."+uuid)

	var vars []Variable
	_, err := store.GetAll(ctx, q, &vars)
	if err != nil {
		return nil, fmt.Errorf("error getting variables: %w", err)
	}

	if len(vars) > 1 {
		return nil, fmt.Errorf("duplicate broadcasts with uuid: %s", uuid)
	} else if len(vars) <= 0 {
		return nil, datastore.ErrNoSuchEntity
	}

	return &vars[0], nil
}

// ComputeVarSum computes the var sum from a slice of variables. The
// var sum is a IEEE CRC checksum 32-bit signed integer of the
// name/value variable pairs concanentated with ampersands, i.e.,
// var1=val1&var2=val2&var3=val3...
func ComputeVarSum(vars []Variable) int64 {
	s := ""
	for _, v := range vars {
		if v.Name[0] == '_' {
			continue // Ignore system variables.
		}
		s += v.Name + "=" + v.Value + "&"
	}
	s = strings.TrimRight(s, "&")
	return (int64(crc32.Checksum([]byte(s), crc32.MakeTable(crc32.IEEE))) ^ 0x80000000) - 0x80000000
}

// GetVarSum gets the varsum for a given scope, and saves it as the
// system variable named "_varsum.<scope>". Note that the varsum is
// itself stored in the datastore not in memory, since it can be
// mutated any time by another datastore client. If the var sum is not
// found it is recomputed and saved.
func GetVarSum(ctx context.Context, store datastore.Store, skey int64, scope string) (int64, error) {
	name := "_varsum." + scope
	v, err := GetVariable(ctx, store, skey, name)
	if err == nil && v.Value != "" {
		vs, err := strconv.ParseInt(v.Value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("could not parse int from varsum: %w", err)
		}
		return vs, nil
	}

	if err != nil && err != datastore.ErrNoSuchEntity {
		return 0, fmt.Errorf("unexpected error getting varsum: %w", err)
	}

	vars, err := GetVariablesBySite(ctx, store, skey, scope)
	if err != nil {
		return 0, fmt.Errorf("could not get variables for site: %w", err)
	}

	vs := ComputeVarSum(vars)
	err = PutVariable(ctx, store, skey, name, strconv.Itoa(int(vs)))
	if err != nil {
		return 0, fmt.Errorf("could not put new varsum: %w", err)
	}

	return vs, nil
}

// invalidateVarSum invalidates varsum(s) resulting from a change to a
// variable (unless the variable is a system variable). If the
// variable is scoped, we delete the varsum just for that scope, else
// we delete all variables for the site.
func invalidateVarSum(ctx context.Context, store datastore.Store, skey int64, name string) error {
	if name[0] == '_' {
		return nil // Ignore system variables.
	}

	sep := strings.Index(name, ".")
	if sep >= 0 {
		scope := name[:sep]
		err := PutVariable(ctx, store, skey, "_varsum."+scope, "")
		if err != nil {
			return fmt.Errorf("could not clear varsum for: %s: %w", scope, err)
		}
		return nil
	}

	// Else clear all varsums for this site.
	vars, err := GetVariablesBySite(ctx, store, skey, "_varsum")
	if err != nil {
		return fmt.Errorf("could not get varsums for site: %w", err)
	}

	for _, v := range vars {
		err = PutVariable(ctx, store, skey, v.Name, "")
		if err != nil {
			return fmt.Errorf("could not clear varsum: %s: %w", v.Name, err)
		}
	}

	return nil
}
