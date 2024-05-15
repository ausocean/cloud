/*
DESCRIPTION
  health.go provides functionality for handling health related broadcast tasks.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Russell Stanley <russell@ausocean.org>

LICENSE
  Copyright (C) 2021 the Australian Ocean Lab (AusOcean)

  This file is part of VidGrind. VidGrind is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  VidGrind is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with NetReceiver in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

package main

import (
	"fmt"
	"log"
	"time"

	"bitbucket.org/ausocean/vidgrind/broadcast"
	"golang.org/x/net/context"
	"google.golang.org/api/youtube/v3"
)

// opsHealthNotify sends a health email notification for site with skey and message
// msg.
func opsHealthNotify(ctx context.Context, sKey int64, msg string) error {
	return notifyOps(ctx, sKey, "health", msg)
}

// opsHealthNotifyFunc returns a closure using opsHealthNotify to be given to the
// broadcast.BroadcastStream function for notifications.
func opsHealthNotifyFunc(ctx context.Context, cfg *BroadcastConfig) func(string) error {
	return func(msg string) error {
		return opsHealthNotify(ctx, cfg.SKey, msg)
	}
}

// handleHealth handles the checking of broadcast health and any required actions
// to resolve problems. Number of successive issues are stored in the broadcast
// config, and if a maximum is reached the streaming hardware is power cycled
// in an attempt to resolve issues.
func handleHealth(ctx context.Context, cfg *BroadcastConfig) error {
	log.Printf("broadcast: %s, ID: %s, handling health check", cfg.Name, cfg.ID)
	hasIssue, err := checkIssues(ctx, cfg)
	if err != nil {
		return fmt.Errorf("could not check for stream issues: %w", err)
	}

	if !hasIssue {
		cfg.Issues = 0
		return nil
	}
	cfg.Issues++

	const maxHealthIssues = 4
	if cfg.Issues > maxHealthIssues {
		// We don't want to restart the hardware if slate is enabled, but we
		// should notify ops given that this will be a problem with vidforward.
		if cfg.Slate {
			msg := fmt.Sprintf("Broadcast: %s, ID: %s, exceeded allowable successive issues, but slate is enabled, so not restarting hardware", cfg.Name, cfg.ID)
			log.Println(msg)
			err = opsHealthNotify(ctx, cfg.SKey, msg)
			if err != nil {
				return fmt.Errorf("could not send notification for poor stream health: %w", err)
			}

			cfg.Issues = 0
			return nil
		}

		log.Printf("Broadcast: %s, ID: %s, exceeded allowable successive issues, restarting hardware", cfg.Name, cfg.ID)
		err = extStop(ctx, cfg)
		if err != nil {
			return fmt.Errorf("external hardware stop error: %w", err)
		}

		// We'll wait 2 minutes for the hardware to register the var changes
		// and shutdown.
		const stopWait = 2 * time.Minute
		time.Sleep(stopWait)

		err = extStart(ctx, cfg, &YouTubeBroadcastService{})
		if err != nil {
			return fmt.Errorf("external hardware start error: %w", err)
		}

		// We'll wait 2 minutes for the hardware to register the var changes.
		time.Sleep(stopWait)

		cfg.Issues = 0
	}

	return nil
}

// handleHealthWithCallback handles the checking of broadcast health. The number of
// successive issues are stored in the broadcast config, and if a maximum is reached
// the badHealthCallback is called. The goodHealthCallback is called when the health
// check is successful.
func handleHealthWithCallback(ctx context.Context, cfg *BroadcastConfig, store Store, svc Svc, badHealthCallback, goodHealthCallback BroadcastCallback) error {
	log.Printf("broadcast: %s, ID: %s, handling health check", cfg.Name, cfg.ID)
	hasIssue, err := checkIssues(ctx, cfg)
	if err != nil {
		return fmt.Errorf("could not check for stream issues: %w", err)
	}

	if !hasIssue {
		cfg.Issues = 0
		goodHealthCallback(ctx, cfg, store, svc)
		return nil
	}
	cfg.Issues++

	const maxHealthIssues = 4
	if cfg.Issues > maxHealthIssues {
		err := badHealthCallback(ctx, cfg, store, svc)
		if err != nil {
			return fmt.Errorf("bad health callback error: %w", err)
		}
		cfg.Issues = 0
	}

	return nil
}

// checkIssues checks for any broadcast issues and returns true if issues are found
// that are considered severe and/or might eventually require a hardware restart
// in an attempt to resolve. We first check for configuration issues e.g. incorrect
// resolution and then we check for basic issues, e.g. insufficient data.
func checkIssues(ctx context.Context, cfg *BroadcastConfig) (bool, error) {
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
		log.Printf("broadcast: %s, ID: %s, configuration issue, reason: %s, severity: %s, type: %s, last updated (seconds): %d", cfg.Name, cfg.ID, v.Reason, v.Severity, v.Type, health.LastUpdateTimeSeconds)

		if v.Severity == "error" {
			msg := "broadcast: %s\n ID: %s\n, configuration issue:\n \tdescription: %s, \treason: %s, \tseverity: %s, \ttype: %s, \tlast updated (seconds): %d"
			err = opsHealthNotify(ctx, cfg.SKey, fmt.Sprintf(msg, cfg.Name, cfg.ID, v.Description, v.Reason, v.Severity, v.Type, health.LastUpdateTimeSeconds))
			if err != nil {
				return true, fmt.Errorf("could not send notification for configuration issue of error severity: %w", err)
			}
			foundIssue = true
		}
	}

	log.Printf("broadcast: %s, ID: %s, stream health check, status: %s", cfg.Name, cfg.ID, health.Status)
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
