package main

import (
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/notification"
)

type hardwareRecoveringVoltage struct {
	stateFields
	stateWithTimeoutFields
}

var _ = register(hardwareRecoveringVoltage{})

func (s hardwareRecoveringVoltage) Name() string { return "hardwareRecoveringVoltage" }

func (s hardwareRecoveringVoltage) New(args ...interface{}) (any, error) {
	return newableWithContext(func(ctx *broadcastContext) any { return newHardwareRecoveringVoltage(ctx) }, args...)
}

func newHardwareRecoveringVoltage(ctx *broadcastContext) *hardwareRecoveringVoltage {
	s := newStateWithTimeoutFields(ctx)
	s.Timeout = time.Duration(sanatisedVoltageRecoveryTimeout(ctx)) * time.Hour
	return &hardwareRecoveringVoltage{
		stateWithTimeoutFields: s,
	}
}

func (s *hardwareRecoveringVoltage) enter() {
	s.LastEntered = time.Now()
}

func sanatisedVoltageRecoveryTimeout(ctx *broadcastContext) int {
	// If VoltageRecoveryTimeout is not set, default to 4 hours.
	if ctx.cfg.VoltageRecoveryTimeout == 0 {
		const defaultRechargeTimeoutHours = 4
		ctx.log("recharge timeout hours is not set, defaulting to %d", defaultRechargeTimeoutHours)
		try(
			ctx.man.Save(nil, func(_cfg *Cfg) { _cfg.VoltageRecoveryTimeout = defaultRechargeTimeoutHours }),
			"could not save default recharge timeout hours to config",
			func(msg string, args ...interface{}) { ctx.logAndNotify(notification.KindSoftware, msg, args...) },
		)
	}
	return ctx.cfg.VoltageRecoveryTimeout
}
