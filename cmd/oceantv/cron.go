package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
	"github.com/google/uuid"
)

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

func deleteBroadcastConfig(ctx Ctx, skey int64, id string) error {
	if err := uuid.Validate(id); err != nil {
		return fmt.Errorf("invalid broadcast UUID: %w", err)
	}
	_, err := getBroadcastConfig(ctx, store, skey, id)
	exists := err == nil
	if err != nil && !errors.Is(err, datastore.ErrNoSuchEntity) {
		return fmt.Errorf("could not load broadcast %s before deletion: %w", id, err)
	}
	if !exists {
		return nil
	}
	if err := model.DeleteVariable(ctx, store, skey, broadcast.Scope+"."+id); err != nil {
		return fmt.Errorf("could not delete broadcast %s configuration: %w", id, err)
	}
	return nil
}

type broadcastCheckRequest struct{ UUID string }
