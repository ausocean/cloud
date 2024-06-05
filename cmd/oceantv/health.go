/*
DESCRIPTION
  health.go provides functionality for handling health related broadcast tasks.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Russell Stanley <russell@ausocean.org>

LICENSE
  Copyright (C) 2021 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"context"
	"fmt"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"google.golang.org/api/youtube/v3"
)

// opsHealthNotify sends a health email notification for site with skey and message
// msg.
func opsHealthNotify(ctx context.Context, sKey int64, msg string) error {
	return notifier.Send(ctx, sKey, "health", msg)
}

// opsHealthNotifyFunc returns a closure using opsHealthNotify to be given to the
// broadcast.BroadcastStream function for notifications.
func opsHealthNotifyFunc(ctx context.Context, cfg *BroadcastConfig) func(string) error {
	return func(msg string) error {
		return opsHealthNotify(ctx, cfg.SKey, msg)
	}
}

// checkIssues checks for any broadcast issues and returns true if issues are found
// that are considered severe and/or might eventually require a hardware restart
// in an attempt to resolve. We first check for configuration issues e.g. incorrect
// resolution and then we check for basic issues, e.g. insufficient data.
func checkIssues(ctx context.Context, cfg *BroadcastConfig, log func(string, ...interface{})) (bool, error) {
	svc, err := broadcast.GetService(ctx, youtube.YoutubeScope)
	if err != nil {
		return false, fmt.Errorf("could not get youtube service: %w", err)
	}

	health, err := broadcast.GetHealthStatus(svc, cfg.SID)
	if err != nil {
		return false, fmt.Errorf("could not get health status: %w", err)
	}

	var foundIssue bool
	for _, v := range health.ConfigurationIssues {
		log("configuration issue, reason: %s, severity: %s, type: %s, last updated (seconds): %d", v.Reason, v.Severity, v.Type, health.LastUpdateTimeSeconds)

		if v.Severity == "error" {
			msg := "broadcast: %s\n ID: %s\n, configuration issue:\n \tdescription: %s, \treason: %s, \tseverity: %s, \ttype: %s, \tlast updated (seconds): %d"
			err = opsHealthNotify(ctx, cfg.SKey, fmt.Sprintf(msg, cfg.Name, cfg.ID, v.Description, v.Reason, v.Severity, v.Type, health.LastUpdateTimeSeconds))
			if err != nil {
				return true, fmt.Errorf("could not send notification for configuration issue of error severity: %w", err)
			}
			foundIssue = true
		}
	}

	log("stream health check, status: %s", health.Status)
	switch health.Status {
	case "noData", "revoked":
		foundIssue = true
		err = opsHealthNotify(ctx, cfg.SKey, fmt.Sprintf("broadcast: %s\n ID: %s\n, poor stream health, status: %s", cfg.Name, cfg.ID, health.Status))
		if err != nil {
			return true, fmt.Errorf("could not send notification for poor stream health: %w", err)
		}
	}

	return foundIssue, nil
}
