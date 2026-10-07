package manager

import (
	"context"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
)

// CronManager keeps scheduled checks in sync with broadcast configurations.
type CronManager interface {
	Sync(context.Context, *broadcast.Config) error
	SyncSecondary(context.Context, *broadcast.Config, *broadcast.Config) error
}

// Option configures an OceanBroadcast manager.
type Option func(*OceanBroadcast)

// WithCronManager couples creation and Enabled changes to scheduled checks.
func WithCronManager(crons CronManager) Option {
	return func(m *OceanBroadcast) { m.crons = crons }
}

// WithCronSyncOnSave makes explicit saves repair failed scheduler requests.
func WithCronSyncOnSave() Option {
	return func(m *OceanBroadcast) { m.syncCronOnSave = true }
}
