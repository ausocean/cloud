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
  along with NetReceiver in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kortschak/sun"
	cron "github.com/robfig/cron/v3"

	"bitbucket.org/ausocean/iotsvc/iotds"
	"github.com/ausocean/cloud/gauth"
)

// The location ID consistent with IANA Time Zone database convention.
const locationID = "Australia/Adelaide"

// scheduler implements a scheduler based on robfig/cron.
type scheduler struct {
	cron *cron.Cron

	mu sync.Mutex
	// ids is a mapping from site/cron to cron id.
	ids map[cronID]cron.EntryID
	// entries is a mapping from cron id to cron state.
	entries map[cron.EntryID]iotds.Cron
	// funcs is the mapping from function names to
	// extension functions.
	funcs map[string]func(string) error
}

// A cronID uniquely identifies a cron across the whole network.
type cronID struct {
	Site int64
	ID   string
}

// newScheduler returns a new scheduler.
func newScheduler() (*scheduler, error) {
	loc, err := time.LoadLocation(locationID)
	if err != nil {
		return nil, err
	}
	c := cron.New(cron.WithParser(sun.Parser{}), cron.WithLocation(loc))
	c.Start() // We will not stop the cron.
	return &scheduler{
		cron:    c,
		ids:     make(map[cronID]cron.EntryID),
		entries: make(map[cron.EntryID]iotds.Cron),
	}, nil
}

// Set installs, updates or removes a cron job from the scheduler based
// on the state of job. The job is removed from the scheduler when it
// is disabled. Such jobs are incomplete in that the only other
// properties guaranteed to be present are the site key and the cron's
// ID. Therefore, the scheduler cannot differentiate between deleted
// and disabled crons. Both are removed from the scheduler but the
// latter persist in the datastore (unbeknownst to the scheduler).
func (s *scheduler) Set(job *iotds.Cron) error {
	log.Printf("setting cron: %v", job.ID)
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.ids[cronID{Site: job.Skey, ID: job.ID}]

	// Check if we already have this job.
	// NB: Guaranteed to fail for disabled crons, since job is incomplete.
	if ok && isSameCron(s.entries[id], *job) {
		log.Printf("cron: %s with this spec, already exists, doing nothing", job.ID)
		// Do nothing since we already have this state.
		return nil
	}

	// Remove disabled crons.
	if !job.Enabled {
		if !ok {
			log.Printf("cron %s disabled", job.ID)
			return nil
		}
		s.cron.Remove(id)
		delete(s.ids, cronID{Site: job.Skey, ID: job.ID})
		delete(s.entries, id)
		log.Printf("removed cron %s", job.ID)
		return nil
	}

	// Remove existing cron if we are going to re-set it.
	// This will only happen if we are actually altering
	// an aspect of the job's scheduling or action.
	if ok {
		s.cron.Remove(id)
		delete(s.ids, cronID{Site: job.Skey, ID: job.ID})
		delete(s.entries, id)
	}

	// TODO: Get lat,lon from site.
	lat := math.NaN()
	lon := math.NaN()

	spec, err := cronSpec(job, lat, lon)
	if err != nil {
		return fmt.Errorf("could not get cron spec for job: %s: %w", job.ID, err)
	}

	log.Printf("cron: %s spec: %v", job.ID, spec)

	// Build a job from the action, var and data values.
	ctx := context.Background()
	var action func()
	notify := func(msg string) error { return notifier.SendOps(ctx, job.Skey, "health", msg) }
	switch strings.ToLower(job.Action) {
	case "set":
		action = func() {
			log.Printf("cron run: setting %s=%q for site=%d: %v", job.Var, job.Data, job.Skey, err)
			err = iotds.PutVariable(ctx, settingsStore, job.Skey, job.Var, job.Data)
			if err != nil {
				logAndNotify(notify, "cron: error setting %s=%q for site=%d: %v", job.Var, job.Data, job.Skey, err)
			}
		}

	case "del":
		action = func() {
			log.Printf("cron run: deleting %s for site=%d: %v", job.Var, job.Skey, err)
			err = iotds.DeleteVariable(ctx, settingsStore, job.Skey, job.Var)
			if err != nil {
				logAndNotify(notify, "cron: error deleting %s for site=%d: %v", job.Var, job.Skey, err)
			}
		}

	case "call":
		fn, ok := s.funcs[job.Var]
		if !ok {
			return fmt.Errorf("no function %q", job.Var)
		}
		action = func() {
			log.Printf("cron run: calling %q at site=%v with %q: %v", job.Var, job.Skey, job.Data, err)
			err = fn(job.Data)
			if err != nil {
				logAndNotify(notify, "cron: error calling %q at site=%v with %q: %v", job.Var, job.Skey, job.Data, err)
			}
		}

	case "rpc":
		_, err := url.Parse(job.Var)
		if err != nil {
			return fmt.Errorf("invalid cron rpc URL %s: %w", job.Var, err)
		}
		reader := bytes.NewReader([]byte(job.Data))
		req, err := http.NewRequest("POST", job.Var, reader)
		if err != nil {
			return fmt.Errorf("invalid cron rpc request %s: %w", job.Var, err)
		}
		req.Header.Set("Content-Type", "application/json")
		tokString, err := gauth.PutClaims(map[string]interface{}{"iss": cronServiceAccount, "skey": job.Skey}, cronSecret)
		if err != nil {
			return fmt.Errorf("error signing claims: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+tokString)
		action = func() {
			log.Printf("cron run: rpc %s at site=%v", job.Var, job.Skey)
			clt := &http.Client{}
			resp, err := clt.Do(req)
			if err != nil {
				logAndNotify(notify, "cron: rpc %s request error: %v", job.Var, err)
				return
			}
			if resp.StatusCode != http.StatusOK {
				logAndNotify(notify, "cron: rpc %s returned unexpected status: %s", job.Var, http.StatusText(resp.StatusCode))
			}
		}

	case "email":
		action = func() {
			log.Printf("cron run: email sent at %v\nvar=%s\ndata=%q", time.Now(), job.Var, job.Data)
			err := notifier.SendOps(ctx, job.Skey, "cron",
				fmt.Sprintf("cron email sent at %v\nvar=%s\ndata=%q",
					time.Now(), job.Var, job.Data))
			if err != nil {
				logAndNotify(notify, "cron: unable to notify ops: %v", err)
			}
		}

	case "sms":
		// TODO: Implement.
		return nil

	default:
		return fmt.Errorf("unknown action: %q", job.Action)
	}

	id, err = s.cron.AddFunc(spec, action)
	if err != nil {
		return fmt.Errorf("failed to add cron spec %s to the cron scheduler: %w", spec, err)
	}
	s.ids[cronID{Site: job.Skey, ID: job.ID}] = id
	s.entries[id] = *job

	return nil
}

// isSameCron returns true if two crons are completely identical.
func isSameCron(a, b iotds.Cron) bool {
	if !a.Time.Equal(b.Time) {
		return false
	}
	a.Time = time.Time{}
	b.Time = time.Time{}
	return a == b
}

// cronSpec returns the Cron rendered as a cron spec line for the given
// geographic location. The spec line makes use of cron predefined scheduling
// definitions implemented by github.com/robfig/cron/v3 and
// github.com/kortschak/sun.
func cronSpec(c *iotds.Cron, lat, lon float64) (string, error) {
	if !c.Enabled {
		return "", nil
	}

	if c.TOD == "" {
		return "", errors.New("no time spec specified for job")
	}

	if strings.HasPrefix(c.TOD, "@sunrise") || strings.HasPrefix(c.TOD, "@noon") || strings.HasPrefix(c.TOD, "@sunset") {
		if math.IsNaN(lat) || math.IsNaN(lon) {
			return "", fmt.Errorf("invalid solar cron: no coordinates")
		}

		return fmt.Sprintf("%s %v %v", c.TOD, lat, lon), nil
	}

	return c.TOD, nil
}

// logAndNotify will log and then call the notify func with the provided message
// (as a formattable string) and args. The notify function for example could
// send an email.
func logAndNotify(notify func(msg string) error, msg string, args ...interface{}) {
	log.Printf(msg, args...)
	err := notify(fmt.Sprintf(msg, args...))
	if err != nil {
		log.Printf("could not send notification: %v", err)
	}
}

// cronHandler handles cron requests originating from a cron client.
// These take the form: /cron/op/skey/id.
func cronHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	ctx := r.Context()
	setup(ctx)

	req := strings.Split(r.URL.Path, "/")
	if len(req) < 5 {
		writeError(w, http.StatusBadRequest, "invalid URL length")
		return
	}

	op := req[2]
	skey, err := strconv.ParseInt(req[3], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid site key: "+req[3])
		return
	}
	id := req[4]

	var cron *iotds.Cron
	switch op {
	case "set":
		cron, err = iotds.GetCron(ctx, settingsStore, skey, id)
		if err != nil {
			log.Printf("could not get cron %s: %v", id, err)
			writeError(w, http.StatusInternalServerError, "could not get cron "+id)
			return
		}

	case "unset":
		cron = &iotds.Cron{Skey: skey, ID: id, Enabled: false}

	default:
		writeError(w, http.StatusBadRequest, "invalid operation: "+op)
		return
	}

	err = cronScheduler.Set(cron)
	if err != nil {
		log.Printf("could not schedule cron %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "could not schedule cron "+id)
		return
	}
	resp := op + " cron " + id
	w.Write([]byte(resp))
}
