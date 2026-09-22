/*
DESCRIPTION
  broadcast.go provides YouTube broadcast scheduling request handling.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Russell Stanley <russell@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2021-2024 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean Bench. Ocean Bench is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean Bench is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/utils"
	"github.com/ausocean/cloud/ytclient"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"google.golang.org/api/youtube/v3"
)

type Action int

type (
	Ctx   = context.Context
	Store = datastore.Store
	Key   = datastore.Key
	Ety   = datastore.Entity
)

const (
	none Action = iota

	// Actions related to vidgrind broadcast control.
	broadcastStart
	broadcastStop
	broadcastSave
	broadcastToken
	broadcastDelete
	broadcastResetState

	// Vidforward control API request actions.
	vidforwardCreate
	vidforwardPlay
	vidforwardSlate
	vidforwardDelete
	vidforwardSlateUpdate
)

// Datastore broadcast and live scopes.
const (
	broadcastScope            = "Broadcast"                           // Scope under which broadcast configs are stored.
	liveScope                 = "Live"                                // Scope under which live stream URLs are stored.
	defaultMessage            = "Welcome to the AusOcean livestream!" // Default message to be sent to the YouTube live chat.
	tempPin                   = "X60"                                 // Standard temperature pin value.
	scalar                    = 0.1                                   // Scalar for temperature conversions from int to float.
	absZero                   = -273.15                               // Offset for temperature conversions from int to float.
	rtmpDestinationAddress    = "rtmp://a.rtmp.youtube.com/live2/"    // Base address for RTMP destination (RTMP key is appended).
	secondaryBroadcastPostfix = "(Secondary)"                         // Post fix used on end of secondary broadcast names.
	longTermBroadcastDuration = 1                                     // The duration of the long term broadcast in years.
)

// broadcastRequest is used by the broadcastHandler to hold broadcast information.
type broadcastRequest struct {
	BroadcastVars      []model.Variable    // Holds prior saved broadcast configs.
	ParsedBroadcasts   []*broadcast.Config // Holds broadcast configs, in the struct, parsed from the JSON in the BroadcastVars.
	CurrentBroadcast   broadcast.Config    // Holds configuration data for broadcast config in form.
	Cameras            []model.Device      // Slice of all the cameras on the site.
	Controllers        []model.Device      // Slice of all the controllers on the site.
	Settings           Settings            // A struct containing options for some settings that have limited options.
	Action             string              // Holds value of any button pressed.
	ListingSecondaries bool                // Are we listing secondary broadcasts?
	Site               *model.Site
	commonData
}

// Settings contains constant values to be used to populate the form with limited options.
type Settings struct {
	Resolution []string
	Privacy    []string
}

// broadcastHandler handles modification to broadcast configurations.
func broadcastHandler(c *fiber.Ctx) error {
	profile, err := getProfile(c)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		return c.Redirect("/", fiber.StatusUnauthorized)
	}
	sKey, err := getCurrentSkey(c, profile)
	if err != nil {
		log.Printf("unable to get current skey, redirecting: %v", err)
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	ctx := c.UserContext()
	if !isAdmin(ctx, sKey, profile.Email) {
		return c.Redirect("/", fiber.StatusUnauthorized)
	}

	req := broadcastRequest{
		commonData: commonData{
			Pages: pages(c, "broadcast"),
		},
		CurrentBroadcast: broadcast.Config{
			UUID:                  c.FormValue("broadcast-uuid"),
			SKey:                  sKey,
			Name:                  c.FormValue("broadcast-name"),
			BID:                   c.FormValue("broadcast-id"),
			StreamName:            c.FormValue("stream-name"),
			Description:           c.FormValue("description"),
			LivePrivacy:           c.FormValue("live-privacy"),
			PostLivePrivacy:       c.FormValue("post-live-privacy"),
			Resolution:            c.FormValue("resolution"),
			StartTimestamp:        c.FormValue("start-timestamp"),
			EndTimestamp:          c.FormValue("end-timestamp"),
			RTMPVar:               c.FormValue("rtmp-key-var"),
			RTMPKey:               c.FormValue("rtmp-key"),
			AuthKeyVar:            c.FormValue("auth-key-var"),
			StorageConfigVar:      c.FormValue("storage-config-var"),
			CameraOutputVar:       c.FormValue("camera-output-var"),
			AuthKey:               c.FormValue("auth-key"),
			BroadcastHost:         c.FormValue("broadcast-host"),
			VidforwardHost:        c.FormValue("vidforward-host"),
			CameraMac:             model.MacEncode(c.FormValue("camera-mac")),
			ControllerMAC:         model.MacEncode(c.FormValue("controller-mac")),
			SendMsg:               c.FormValue("report-sensor") == "Chat",
			UsingVidforward:       c.FormValue("use-vidforward") == "using-vidforward",
			CheckingHealth:        c.FormValue("check-health") == "checking-health",
			Enabled:               c.FormValue("enabled") == "enabled",
			InFailure:             c.FormValue("in-failure") == "in-failure",
			RegisterOpenFish:      c.FormValue("register-openfish") == "register-openfish",
			OpenFishCaptureSource: c.FormValue("openfish-capturesource"),
			BatteryVoltagePin:     c.FormValue("battery-voltage-pin"),
			NotifySuppressRules:   c.FormValue("notify-suppress-rules"),
		},
		Action:             c.FormValue("action"),
		ListingSecondaries: c.FormValue("list-secondaries") == "listing-secondaries",
		Settings: Settings{
			Resolution: []string{"1080p"},
			Privacy:    []string{"unlisted", "private", "public"},
		},
	}

	// The action fields are JSON arrays (or legacy CSV) in the form; parse them
	// into their typed representation before they are saved.
	req.CurrentBroadcast.OnActions, err = broadcast.ParseActionVars(c.FormValue("on-actions"))
	if err != nil {
		reportError(c, req, "could not parse on-actions: %v", err)
		return nil
	}
	req.CurrentBroadcast.OffActions, err = broadcast.ParseActionVars(c.FormValue("off-actions"))
	if err != nil {
		reportError(c, req, "could not parse off-actions: %v", err)
		return nil
	}
	req.CurrentBroadcast.ShutdownActions, err = broadcast.ParseActionVars(c.FormValue("shutdown-actions"))
	if err != nil {
		reportError(c, req, "could not parse shutdown-actions: %v", err)
		return nil
	}

	// Only set the storage config if any of its fields have been provided,
	// otherwise leave it nil so that it is omitted when marshalled.
	provider := c.FormValue("storage-provider")
	bucket := c.FormValue("storage-bucket")
	prefix := c.FormValue("storage-prefix")
	if provider != "" || bucket != "" || prefix != "" {
		req.CurrentBroadcast.StorageConfig = &broadcast.StorageConfig{
			Bucket:   bucket,
			Prefix:   prefix,
			Provider: provider,
		}
	}

	streamVoltage := c.FormValue("required-streaming-voltage")
	if streamVoltage == "" {
		req.CurrentBroadcast.RequiredStreamingVoltage = 0
	} else {
		req.CurrentBroadcast.RequiredStreamingVoltage, err = strconv.ParseFloat(streamVoltage, 64)
		if err != nil {
			reportError(c, req, "could not parse required streaming voltage: %v", err)
			return nil
		}
	}

	voltageTimeout := c.FormValue("voltage-recovery-timeout")
	if voltageTimeout == "" {
		req.CurrentBroadcast.VoltageRecoveryTimeout = 0
	} else {
		req.CurrentBroadcast.VoltageRecoveryTimeout, err = strconv.Atoi(c.FormValue("voltage-recovery-timeout"))
		if err != nil {
			reportError(c, req, "could not parse voltage recovery timeout: %v", err)
			return nil
		}
	}

	cfg := &req.CurrentBroadcast

	// This is how we populate the time.Time representations of the start and end
	// times.
	if cfg.StartTimestamp != "" {
		err = cfg.ParseStartEnd()
		if err != nil {
			reportError(c, req, "could not parse start and end times: %v", err)
			return nil
		}
	}

	// Load config information for any existing broadcasts that have been saved.
	req.BroadcastVars, err = model.GetVariablesBySite(ctx, settingsStore, sKey, broadcastScope)
	switch err {
	case nil, datastore.ErrNoSuchEntity:
	default:
		reportError(c, req, "could not get broadcast configs variable: %v", err)
		return nil
	}

	for _, v := range req.BroadcastVars {
		cfg := &broadcast.Config{}
		err := json.Unmarshal([]byte(v.Value), cfg)
		if err != nil {
			reportError(c, req, "could not unmarshal broadcast variables: %v", err)
			return nil
		}

		// Handle older version broadcasts which don't have a UUID.
		if cfg.UUID == "" {
			cfg.UUID = cfg.Name
		}
		req.ParsedBroadcasts = append(req.ParsedBroadcasts, cfg)
	}

	// If we're not listing secondaries, we need to filter out any secondary broadcasts.
	if !req.ListingSecondaries {
		var filteredVars []model.Variable
		for _, v := range req.BroadcastVars {
			if !strings.Contains(v.Name, "secondary") && !strings.Contains(v.Name, "Secondary") {
				filteredVars = append(filteredVars, v)
			}
		}
		req.BroadcastVars = filteredVars
	}

	// Get site to use the site's timezone.
	req.Site, err = model.GetSite(ctx, settingsStore, sKey)
	if err != nil {
		reportError(c, req, "could not get site to establish timezone: %v", err)
		return nil
	}

	action := stringToAction(req.Action, req)

	// Get all Cameras and Controllers that could be used by the broadcast.
	devices, err := model.GetDevicesBySite(ctx, settingsStore, sKey)
	if err != nil {
		reportError(c, req, "could not get sites devices: %v", err)
		return nil
	}

	for _, dev := range devices {
		if dev.Type == model.DevTypeCamera {
			req.Cameras = append(req.Cameras, dev)
		} else if dev.Type == model.DevTypeController {
			req.Controllers = append(req.Controllers, dev)
		}
	}

	// Populate sensor list that contains sensors that will display values in
	// live chat.
	err = updateSensorList(ctx, &req, c, settingsStore)
	if err != nil {
		reportError(c, req, "could not update sensor list: %v", err)
		return nil
	}

	var msg string
	switch action {
	case broadcastToken:
		tokenURI := utils.TokenURIFromAccount(profile.Email)

		var err error
		adaptErr := adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err = ytclient.AuthChannel(r.Context(), w, r, youtube.YoutubeScope, tokenURI)
		})(c)
		if adaptErr != nil {
			reportError(c, req, "internal adapter error: %v", adaptErr)
			return nil
		}

		if err != nil {
			reportError(c, req, "could not authenticate channel: %v", err)
			return nil
		}

		// Store the account email in the broadcast config.
		cfg.Account = profile.Email
		err = saveBroadcast(ctx, &req.CurrentBroadcast)
		if err != nil {
			reportError(c, req, "could not save broadcast: %v", err)
			return nil
		}
		msg = "channel authenticated successfully"
	case broadcastSave:
		// Check if we've just pulled the hardware out of a failure state.
		// We do this by checking if the hardware was in a failure state and
		// now it's not.
		curBroadcast, err := broadcastFromVars(req.BroadcastVars, cfg.UUID)
		if errors.Is(err, ErrBroadcastNotFound{}) {
			// Assume the broadcast is newly saved.
		} else if err != nil {
			reportError(c, req, "could not get broadcast from vars to check hardware state: %v", err)
			return nil
		} else if c.FormValue("in-failure") == "false" && curBroadcast.HardwareState == "hardwareFailure" {
			cfg.HardwareState = "hardwareOff"
		}

		// If we haven't just generated a token we should keep the same account
		// that the config previously had.
		cfg.Account, err = getExistingAccount(req.BroadcastVars, cfg)
		if errors.Is(err, ErrBroadcastNotFound{}) {
			// If the broadcast doesn't exist, we will be creating a new broadcast.
			// Do nothing.
		} else if err != nil {
			reportError(c, req, "could not get existing account for name: %s: %v", cfg.Name, err)
			return nil
		}

		err = saveBroadcast(ctx, &req.CurrentBroadcast)
		if err != nil {
			reportError(c, req, "could not save broadcast: %v", err)
			return nil
		}
		msg = "broadcast saved successfully"

		// Ensure that the CheckBroadcast cron exists.
		const broadcastCheckCronID = "Broadcast Check"
		_, err = model.GetCron(ctx, settingsStore, cfg.SKey, broadcastCheckCronID)
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			cr := &model.Cron{Skey: cfg.SKey, ID: broadcastCheckCronID, TOD: "* * * * *", Action: "rpc", Var: tvURL + "/checkbroadcasts", Enabled: true}
			err = model.PutCron(context.Background(), settingsStore, cr)
			if err != nil {
				reportError(c, req, "warning: failed to failed to put checkbroadcasts cron in datastore: %v", err)
				return nil
			}

			err = cronScheduler.Set(cr)
			if err != nil {
				reportError(c, req, "could not automatically set broadcast check cron in the scheduler: %v", err)
				return nil
			}
		} else if err != nil {
			reportError(c, req, "unexpected error when checking for the broadcast check cron: %v", err)
			return nil
		}

	case broadcastDelete:
		err = deleteBroadcast(ctx, &req, settingsStore)
		if err != nil {
			reportError(c, req, "could not delete broadcast: %v", err)
			return nil
		}
		msg = "broadcast deleted successfully"

	case vidforwardSlateUpdate:
		const fieldName = "slate-file"
		fh, err := c.FormFile(fieldName)
		if err != nil {
			reportError(c, req, "could not get file from request form: %v", err)
			return nil
		}
		file, err := fh.Open()
		if err != nil {
			reportError(c, req, "could not open file: %v", err)
			return nil
		}
		defer file.Close()
		err = (NewVidforwardService()).UploadSlate(cfg, fh.Filename, file)
		if err != nil {
			reportError(c, req, "could not upload slate: %v", err)
			return nil

		}
		msg = "slate uploaded successfully"
	case broadcastResetState:
		err = resetState(ctx, &req.CurrentBroadcast)
		if err != nil {
			reportError(c, req, "could not reset state: %v", err)
			return nil
		}

		v, err := model.GetVariable(ctx, settingsStore, sKey, broadcastScope+"."+cfg.UUID)
		if err != nil {
			reportError(c, req, "could not load saved broadcast: %v", err)
			return nil
		}
		err = json.Unmarshal([]byte(v.Value), cfg)
		if err != nil {
			reportError(c, req, "could not unmarshal broadcast: %v", err)
			return nil
		}
	}

	writeTemplate(c, "broadcast.html", &req, msg)
	return nil
}

func stringToAction(s string, req broadcastRequest) Action {
	buttonPress := func(s string) Action {
		res, ok := map[string]Action{
			"":                        none,
			"broadcast-start":         broadcastStart,
			"broadcast-stop":          broadcastStop,
			"broadcast-save":          broadcastSave,
			"broadcast-token":         broadcastToken,
			"broadcast-delete":        broadcastDelete,
			"broadcast-reset-state":   broadcastResetState,
			"vidforward-create":       vidforwardCreate,
			"vidforward-play":         vidforwardPlay,
			"vidforward-slate":        vidforwardSlate,
			"vidforward-delete":       vidforwardDelete,
			"vidforward-slate-update": vidforwardSlateUpdate,
		}[s]
		if !ok {
			panic("button string not recognised")
		}
		return res
	}(req.Action)
	return buttonPress
}

// saveBroadcast sends a request to save a broadcast to the broadcast manager service (oceantv).
// TODO: Add JWT signing.
func saveBroadcast(ctx context.Context, cfg *broadcast.Config) error {
	if cfg.UUID == "" {
		// The config is new, and should be assigned a UUID.
		cfg.UUID = uuid.NewString()
	} else if err := uuid.Validate(cfg.UUID); err != nil {
		return fmt.Errorf("broadcast config has invalid UUID: %w", err)
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error marshalling broadcast config: %w", err)
	}

	const saveMethod = "/broadcast/save"
	url := tvURL + saveMethod
	reader := bytes.NewReader(data)
	req, err := http.NewRequest("POST", url, reader)
	if err != nil {
		return fmt.Errorf("error creating %s request: %w", saveMethod, err)
	}
	req.Header.Set("Content-Type", "application/json")

	clt := &http.Client{}
	resp, err := clt.Do(req)
	if err != nil {
		return fmt.Errorf("error sending %s request: %w", saveMethod, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s request failed with status code: %s", saveMethod, http.StatusText(resp.StatusCode))
	}

	log.Printf("%s OK", saveMethod)
	return nil
}

// resetState sends a request to reset the state of a broadcast to the broadcast manager service (oceantv).
// TODO: Add JWT signing.
func resetState(ctx context.Context, cfg *broadcast.Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error marshalling broadcast config: %w", err)
	}

	const resetStateEndpoint = "/broadcast/reset-state"
	url := tvURL + resetStateEndpoint
	reader := bytes.NewReader(data)
	req, err := http.NewRequest("POST", url, reader)
	if err != nil {
		return fmt.Errorf("error creating %s request: %w", resetStateEndpoint, err)
	}
	req.Header.Set("Content-Type", "application/json")

	clt := &http.Client{}
	resp, err := clt.Do(req)
	if err != nil {
		return fmt.Errorf("error sending %s request: %w", resetStateEndpoint, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s request failed with status code: %s", resetStateEndpoint, http.StatusText(resp.StatusCode))
	}

	log.Printf("%s OK", resetStateEndpoint)
	return nil
}

// deleteBroadcast deletes a broadcast from the datastore and also updates the BroadcastVars
// list and CurrentBroadcast config to clear the form on next page write.
func deleteBroadcast(ctx context.Context, req *broadcastRequest, store datastore.Store) error {
	cfg := &req.CurrentBroadcast
	err := model.DeleteVariable(ctx, store, cfg.SKey, broadcastScope+"."+cfg.UUID)
	if err != nil {
		return fmt.Errorf("could not delete broadcast: %v", err)
	}

	req.BroadcastVars, err = model.GetVariablesBySite(ctx, store, cfg.SKey, broadcastScope)
	switch err {
	case nil, datastore.ErrNoSuchEntity:
	default:
		return fmt.Errorf("could not get broadcast variables: %v", err)
	}

	req.CurrentBroadcast = broadcast.Config{}
	return nil
}

// getExistingAccount will return the current associated account of the broadcast with the current config
// name. This should be used to ensure that the associated account is only updated using the generate token method.
// If no broadcast/account is found, then an empty string will be returned, along with an error.
func getExistingAccount(broadcasts []model.Variable, cfg *broadcast.Config) (string, error) {
	_cfg, err := broadcastFromVars(broadcasts, cfg.UUID)
	if err != nil {
		return "", err
	}
	return _cfg.Account, nil
}

func updateSensorList(ctx context.Context, req *broadcastRequest, c *fiber.Ctx, store datastore.Store) error {
	devices, err := model.GetDevicesBySite(ctx, store, req.CurrentBroadcast.SKey)
	if err != nil {
		return fmt.Errorf("could no get devices: %w", err)
	}

	// Load the sensor entries for the ESP device.
	for _, dev := range devices {
		if dev.Type != "esp" && dev.Type != "Controller" {
			continue
		}
		sensors, err := model.GetSensorsV2(ctx, store, dev.Mac)
		if err != nil {
			return fmt.Errorf("could not get sensors: %w", err)
		}
		for _, sensor := range sensors {
			entry := broadcast.SensorEntry{
				SendMsg:   c.FormValue(strings.ToLower(sensor.Name)) == sensor.Name,
				Sensor:    sensor,
				Name:      strings.ToLower(sensor.Name),
				DeviceMac: dev.Mac,
			}
			req.CurrentBroadcast.SensorList = append(req.CurrentBroadcast.SensorList, entry)
		}
	}
	return nil
}

type ErrInvalidEndTime struct {
	start, end time.Time
}

func (e ErrInvalidEndTime) Error() string {
	return fmt.Sprintf("end time (%v) is invalid relative to start time (%v)", e.end, e.start)
}

func performRequestWithRetries(dest string, data any, maxRetries int) error {
	var retries int
retry:
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(data)
	if err != nil {
		return fmt.Errorf("could not encode data struct: %w", err)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	httpReq, err := http.NewRequest(http.MethodPut, dest, &buf)
	if err != nil {
		return fmt.Errorf("could not create new http request: %w", err)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("could not do http request, but retrying: %v", err)
		if retries <= maxRetries {
			retries++
			goto retry
		}
		return fmt.Errorf("could not do http request: %w, resp: %v", err, resp)
	}

	return nil
}

// liveHandler handles requests to /live/<broadcast name>. This redirects to the
// livestream URL stored in a variable with name corresponding to the given broadcast name.
// A counter for link visits is also kept and incremented on each visit.
func liveHandler(c *fiber.Ctx) error {
	logRequest(c)

	ctx := c.UserContext()
	setup(ctx)

	key := c.Params("broadcastName")
	v, err := model.GetVariable(ctx, settingsStore, -1, liveScope+"."+key)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"error": fmt.Sprintf("livestream %s does not exist: %v", key, err)})
	}

	// Increment the link visit count.
	go func() {
		bgCtx := context.Background()
		if err := incrementVisitCount(bgCtx, settingsStore, key); err != nil {
			log.Printf("failed to increment counter for livestream %s: %v", key, err)
		}
	}()

	// Transform the YouTube URL based on options in query parameters.
	redirectURL := v.Value
	redirectURL, err = transformYouTubeURL(redirectURL, c)
	if err != nil {
		_err := fmt.Errorf("error transforming YouTube URL: %v", err)
		log.Print(_err)
		c.Status(fiber.StatusInternalServerError)
		return c.JSON(fiber.Map{"error": _err})
	}

	log.Printf("redirecting to livestream link: %s", redirectURL)
	return c.Redirect(redirectURL, fiber.StatusFound)
}

// incrementVisitCount increments the visits counter for the given stream name.
func incrementVisitCount(ctx context.Context, store datastore.Store, streamName string) error {
	variableName := fmt.Sprintf("visits.%s", streamName)

	// Put the incremented count. A site key of -1 indicates a global variable.
	return model.PutVariableInTransaction(ctx, store, -1, variableName, func(currentValue string) string {
		// Parse the current count or default to 0 if the variable doesn't exist.
		visitCount := 0
		if currentValue != "" {
			var err error
			visitCount, err = strconv.Atoi(currentValue)
			if err != nil {
				log.Printf("could not parse visit count: %v", err)
				return ""
			}
		}

		// Increment the count.
		visitCount++
		return strconv.Itoa(visitCount)
	})
}

// transformYouTubeURL transforms the YouTube URL based on options in the query parameters.
// The options are autoplay, mute, and embed. The embed option will also cause rel=0 to be added.
// rel=0 means that only videos from your channel will be suggested when the video is stopped.
func transformYouTubeURL(rawURL string, c *fiber.Ctx) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("could not parse YouTube watch URL: %w", err)
	}

	query := u.Query()
	videoID := query.Get("v")
	if videoID == "" {
		return "", errors.New("invalid YouTube watch URL, missing video ID")
	}

	// Update path if embed is requested, otherwise keep the video ID in the query.
	newQuery := url.Values{}
	if c.Query("embed") != "" {
		u.Path = fmt.Sprintf("/embed/%s", videoID)
		u.RawQuery = "" // Reset query parameters.
		// Always set rel=0 for embedded videos.
		newQuery.Set("rel", "0")

		// Conditionally set mute and autoplay if requested.
		if c.Query("mute") != "" {
			newQuery.Set("mute", "1")
		}
		if c.Query("autoplay") != "" {
			newQuery.Set("autoplay", "1")
		}
	} else {
		newQuery.Set("v", videoID)
	}

	u.RawQuery = newQuery.Encode()
	return u.String(), nil
}
