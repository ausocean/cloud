package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/manager"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/google/uuid"
)

const legacyBroadcastCronID = "Broadcast Check"

type broadcastCheckRequest struct {
	UUID string
}

func broadcastCronID(id string) string { return legacyBroadcastCronID + " " + id }

type broadcastCronScheduler interface {
	Set(Ctx, *model.Cron) error
}

type httpCronScheduler struct {
	url string
}

func (s httpCronScheduler) Set(ctx Ctx, job *model.Cron) error {
	op := "set"
	if !job.Enabled {
		op = "unset"
	}
	endpoint := s.url + "/cron/" + op + "/" + strconv.FormatInt(job.Skey, 10) + "/" + url.PathEscape(job.ID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("could not create cron %s request for %s: %w", op, job.ID, err)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("could not send cron %s request for %s: %w", op, job.ID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cron %s failed: %s", op, resp.Status)
	}
	return nil
}

// broadcastCronManager owns the cron lifecycle for broadcast configurations.
// Internal state saves only call Sync when creation or Enabled changes; normal
// state-machine ticks do not read or write Cron entities.
type broadcastCronManager struct {
	store     Store
	scheduler broadcastCronScheduler
	endpoint  string
}

func (m *broadcastCronManager) withEndpoint(endpoint string) *broadcastCronManager {
	copy := *m
	copy.endpoint = endpoint
	return &copy
}

func (m *broadcastCronManager) Sync(ctx Ctx, cfg *Cfg) error {
	if err := uuid.Validate(cfg.UUID); err != nil {
		return fmt.Errorf("cannot schedule broadcast without a UUID: %w", err)
	}
	data, err := json.Marshal(broadcastCheckRequest{UUID: cfg.UUID})
	if err != nil {
		return fmt.Errorf("could not marshal check payload for broadcast %s: %w", cfg.UUID, err)
	}
	job := &model.Cron{Skey: cfg.SKey, ID: broadcastCronID(cfg.UUID), TOD: fmt.Sprintf("@every %s", broadcast.CheckInterval), Action: "rpc", Var: m.endpoint, Data: string(data), Enabled: cfg.Enabled}
	existing, err := model.GetCron(ctx, m.store, job.Skey, job.ID)
	if err != nil && !errors.Is(err, datastore.ErrNoSuchEntity) {
		return fmt.Errorf("could not get cron %s: %w", job.ID, err)
	}
	if err == nil && existing.Var != "" {
		// Preserve the deployment/custom target selected for an existing job.
		job.Var = existing.Var
	}
	if err := model.PutCron(ctx, m.store, job); err != nil {
		return fmt.Errorf("could not save cron %s: %w", job.ID, err)
	}
	// Always notify the scheduler so retrying a save repairs a failed RPC.
	if err := m.scheduler.Set(ctx, job); err != nil {
		return fmt.Errorf("could not sync cron %s with scheduler: %w", job.ID, err)
	}
	return nil
}

func (m *broadcastCronManager) Delete(ctx Ctx, skey int64, id string) error {
	if err := m.deleteJob(ctx, skey, broadcastCronID(id)); err != nil {
		return fmt.Errorf("could not delete cron for broadcast %s: %w", id, err)
	}
	return nil
}

// SyncSecondary preserves the deployment selected for the primary broadcast.
func (m *broadcastCronManager) SyncSecondary(ctx Ctx, parent, secondary *Cfg) error {
	job, err := model.GetCron(ctx, m.store, parent.SKey, broadcastCronID(parent.UUID))
	if err != nil {
		return fmt.Errorf("could not get primary broadcast cron: %w", err)
	}
	if err := m.withEndpoint(job.Var).Sync(ctx, secondary); err != nil {
		return fmt.Errorf("could not sync secondary broadcast %s cron: %w", secondary.UUID, err)
	}
	return nil
}

func (m *broadcastCronManager) deleteJob(ctx Ctx, skey int64, id string) error {
	job, err := model.GetCron(ctx, m.store, skey, id)
	exists := err == nil
	if err != nil && !errors.Is(err, datastore.ErrNoSuchEntity) {
		return fmt.Errorf("could not get cron %s before deletion: %w", id, err)
	}
	if err == nil {
		job.Enabled = false
		// A scheduler restart must not re-enable the job while deletion retries.
		if err := model.PutCron(ctx, m.store, job); err != nil {
			return fmt.Errorf("could not disable cron %s in datastore: %w", id, err)
		}
	} else {
		job = &model.Cron{Skey: skey, ID: id, Enabled: false}
	}
	if err := m.scheduler.Set(ctx, job); err != nil {
		return fmt.Errorf("could not unset cron %s in scheduler: %w", id, err)
	}
	if !exists {
		return nil
	}
	if err := model.DeleteCron(ctx, m.store, skey, id); err != nil {
		return fmt.Errorf("could not delete cron %s from datastore: %w", id, err)
	}
	return nil
}

// migrateSite is invoked once by an old site-level cron. Install all replacement
// jobs successfully before removing the old job; a failed migration can retry.
func (m *broadcastCronManager) migrateSite(ctx Ctx, skey int64) error {
	legacy, err := model.GetCron(ctx, m.store, skey, legacyBroadcastCronID)
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not get legacy cron for site %d: %w", skey, err)
	}
	crons := m.withEndpoint(legacy.Var)
	vars, err := model.GetVariablesBySite(ctx, m.store, skey, broadcast.Scope)
	if err != nil {
		return fmt.Errorf("could not list broadcasts for site %d: %w", skey, err)
	}
	for _, variable := range vars {
		var cfg Cfg
		if err := json.Unmarshal([]byte(variable.Value), &cfg); err != nil {
			return fmt.Errorf("could not decode broadcast %s during migration: %w", variable.Name, err)
		}
		if cfg.SKey != skey {
			return fmt.Errorf("broadcast %s belongs to a different site", cfg.Name)
		}
		if uuid.Validate(cfg.UUID) != nil {
			man := newOceanBroadcastManager(nil, &cfg, m.store, func(string, ...interface{}) {}, manager.WithCronManager(crons))
			if err := man.Save(ctx, nil); err != nil {
				return fmt.Errorf("could not migrate legacy broadcast %s: %w", cfg.Name, err)
			}
		} else if err := crons.Sync(ctx, &cfg); err != nil {
			return fmt.Errorf("could not schedule broadcast %s during migration: %w", cfg.UUID, err)
		}
	}
	if err := m.deleteJob(ctx, skey, legacyBroadcastCronID); err != nil {
		return fmt.Errorf("could not remove legacy cron for site %d: %w", skey, err)
	}
	return nil
}

func getBroadcastConfig(ctx Ctx, db Store, skey int64, id string) (*Cfg, error) {
	variable, err := model.GetVariable(ctx, db, skey, broadcast.Scope+"."+id)
	if err != nil {
		return nil, fmt.Errorf("could not get broadcast %s for site %d: %w", id, skey, err)
	}
	var cfg Cfg
	if err := json.Unmarshal([]byte(variable.Value), &cfg); err != nil {
		return nil, fmt.Errorf("could not decode broadcast %s for site %d: %w", id, skey, err)
	}
	if cfg.SKey != skey || cfg.UUID != id {
		return nil, fmt.Errorf("broadcast configuration does not match requested site and UUID")
	}
	return &cfg, nil
}

func broadcastCheckEndpoint(r *http.Request) string {
	scheme := r.URL.Scheme
	if scheme == "" {
		scheme = "https"
		if standalone && r.TLS == nil {
			scheme = "http"
		}
	}
	return (&url.URL{Scheme: scheme, Host: r.Host, Path: "/checkbroadcasts"}).String()
}

func deleteBroadcastConfig(ctx Ctx, skey int64, id string) error {
	if err := uuid.Validate(id); err != nil {
		return fmt.Errorf("invalid broadcast UUID: %w", err)
	}
	cfg, err := getBroadcastConfig(ctx, store, skey, id)
	exists := err == nil
	if err != nil && !errors.Is(err, datastore.ErrNoSuchEntity) {
		return fmt.Errorf("could not load broadcast %s before deletion: %w", id, err)
	}
	if err == nil {
		man := newOceanBroadcastManager(nil, cfg, store, func(string, ...interface{}) {})
		if err := man.Save(ctx, func(cfg *Cfg) { cfg.Enabled = false }); err != nil {
			return fmt.Errorf("could not disable broadcast %s before deletion: %w", id, err)
		}
		if err := performChecks(ctx, cfg, store, nil, nil); err != nil {
			return fmt.Errorf("could not clean up broadcast %s before deletion: %w", id, err)
		}
	}
	if broadcastCrons != nil {
		if err := broadcastCrons.Delete(ctx, skey, id); err != nil {
			return fmt.Errorf("could not delete cron for broadcast %s: %w", id, err)
		}
	}
	if !exists {
		return nil
	}
	if err := model.DeleteVariable(ctx, store, skey, broadcast.Scope+"."+id); err != nil {
		return fmt.Errorf("could not delete broadcast %s configuration: %w", id, err)
	}
	return nil
}
