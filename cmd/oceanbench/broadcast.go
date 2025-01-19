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
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/gauth"
	"github.com/ausocean/cloud/model"
	"github.com/ausocean/cloud/utils"
	"github.com/ausocean/openfish/datastore"
	"google.golang.org/api/youtube/v3"
)

type Action int

type (
	Cfg   = BroadcastConfig
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
	broadcastSelect

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
	BroadcastVars      []model.Variable // Holds prior saved broadcast configs.
	CurrentBroadcast   BroadcastConfig  // Holds configuration data for broadcast config in form.
	Cameras            []model.Device   // Slice of all the cameras on the site.
	Controllers        []model.Device   // Slice of all the controllers on the site.
	Settings           Settings         // A struct containing options for some settings that have limited options.
	Action             string           // Holds value of any button pressed.
	ListingSecondaries bool             // Are we listing secondary broadcasts?
	Site               *model.Site
	commonData
}

// Settings contains constant values to be used to populate the form with limited options.
type Settings struct {
	Resolution []string
	Privacy    []string
}

// BroadcastConfig holds configuration data for a YouTube broadcast.
type BroadcastConfig struct {
	SKey                     int64         // The key of the site this broadcast belongs to.
	Name                     string        // The name of the broadcast.
	ID                       string        // Broadcast identification.
	SID                      string        // Stream ID for any currently associated stream.
	CID                      string        // ID of associated chat.
	StreamName               string        // The name of the stream we'll bind to the broadcast.
	Description              string        // The broadcast description shown below viewing window.
	Privacy                  string        // Privacy of the broadcast i.e. public, private or unlisted.
	Resolution               string        // Resolution of the stream e.g. 1080p.
	StartTimestamp           string        // Start time of the broadcast in unix format.
	Start                    time.Time     // Start time in native go format for easy operations.
	EndTimestamp             string        // End time of the broadcast in unix format.
	End                      time.Time     // End time in native go format for easy operations.
	VidforwardHost           string        // Host address of vidforward service.
	CameraMac                int64         // Camera hardware's MAC address.
	ControllerMAC            int64         // Controller hardware's MAC adress (controller used to power camera).
	OnActions                string        // A series of actions to be used for power up of camera hardware.
	OffActions               string        // A series of actions to be used for power down of camera hardware.
	RTMPVar                  string        // The variable name that holds the RTMP URL and key.
	Active                   bool          // This is true if the broadcast is currently active i.e. waiting for data or currently streaming.
	Slate                    bool          // This is true if the broadcast is currently in slate mode i.e. no camera.
	Issues                   int           // The number of successive stream issues currently experienced. Reset when good health seen.
	SendMsg                  bool          // True if sensor data will be sent to the YouTube live chat.
	SensorList               []SensorEntry // List of sensors which can be reported to the YouTube live chat.
	RTMPKey                  string        // The RTMP key corresponding to the newly created broadcast.
	UsingVidforward          bool          // Indicates if we're using vidforward i.e. doing long term broadcast.
	CheckingHealth           bool          // Are we performing health checks for the broadcast? Having this false is useful for dodgy testing streams.
	AttemptingToStart        bool          // Indicates if we're currently attempting to start the broadcast.
	Enabled                  bool          // Is the broadcast enabled? If not, it will not be started.
	Events                   []string      // Holds names of events that are yet to be handled.
	Unhealthy                bool          // True if the broadcast is unhealthy.
	BroadcastState           string        // Holds the current state of the broadcast.
	HardwareState            string        // Holds the current state of the hardware.
	StartFailures            int           // The number of times the broadcast has failed to start.
	Transitioning            bool          // If the broadcast is transition from live to slate, or vice versa.
	StateData                []byte        // States will be marshalled and their data stored here.
	HardwareStateData        []byte        // Hardware states will be marshalled and their data stored here.
	Account                  string        // The YouTube account email that this broadcast is associated with.
	InFailure                bool          // True if the broadcast is in a failure state.
	RecoveringVoltage        bool          // True if the broadcast is currently recovering voltage.
	RequiredStreamingVoltage float64       // The required battery voltage for the camera to stream.
	VoltageRecoveryTimeout   int           // Max allowable hours for voltage recovery before failure.
	RegisterOpenFish         bool          // True if the video should be registered with openfish for annotation.
	OpenFishCaptureSource    string        // The capture source to register the stream to.
}

func (b *BroadcastConfig) PrettyHardwareStateData() string {
	return string(b.HardwareStateData)
}

// SensorEntry contains the information for each sensor.
type SensorEntry struct {
	SendMsg   bool
	Sensor    model.SensorV2
	Name      string
	DeviceMac int64
}

// parseStartEnd takes the start and end time unix strings from the broadcast
// and provides these as time.Time.
func (c *BroadcastConfig) parseStartEnd() error {
	sInt, err := strconv.ParseInt(c.StartTimestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse unix start time: %w", err)
	}
	eInt, err := strconv.ParseInt(c.EndTimestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse unix end time: %w", err)
	}
	c.Start, c.End = time.Unix(sInt, 0), time.Unix(eInt, 0)
	return nil
}

// broadcastHandler handles modification to broadcast configurations.
func broadcastHandler(w http.ResponseWriter, r *http.Request) {
	profile, err := getProfile(w, r)
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}
	sKey, _ := profileData(profile)

	req := broadcastRequest{
		commonData: commonData{
			Pages: pages("broadcast"),
		},
		CurrentBroadcast: BroadcastConfig{
			SKey:                  sKey,
			Name:                  r.FormValue("broadcast-name"),
			ID:                    r.FormValue("broadcast-id"),
			StreamName:            r.FormValue("stream-name"),
			Description:           r.FormValue("description"),
			Privacy:               r.FormValue("privacy"),
			Resolution:            r.FormValue("resolution"),
			StartTimestamp:        r.FormValue("start-timestamp"),
			EndTimestamp:          r.FormValue("end-timestamp"),
			RTMPVar:               r.FormValue("rtmp-key-var"),
			RTMPKey:               r.FormValue("rtmp-key"),
			VidforwardHost:        r.FormValue("vidforward-host"),
			CameraMac:             model.MacEncode(r.FormValue("camera-mac")),
			ControllerMAC:         model.MacEncode(r.FormValue("controller-mac")),
			OnActions:             r.FormValue("on-actions"),
			OffActions:            r.FormValue("off-actions"),
			SendMsg:               r.FormValue("report-sensor") == "Chat",
			UsingVidforward:       r.FormValue("use-vidforward") == "using-vidforward",
			CheckingHealth:        r.FormValue("check-health") == "checking-health",
			Enabled:               r.FormValue("enabled") == "enabled",
			InFailure:             r.FormValue("in-failure") == "in-failure",
			RegisterOpenFish:      r.FormValue("register-openfish") == "register-openfish",
			OpenFishCaptureSource: r.FormValue("openfish-capturesource"),
		},
		Action:             r.FormValue("action"),
		ListingSecondaries: r.FormValue("list-secondaries") == "listing-secondaries",
		Settings: Settings{
			Resolution: []string{"1080p"},
			Privacy:    []string{"unlisted", "private", "public"},
		},
	}

	ctx := r.Context()

	cfg := &req.CurrentBroadcast

	// This is how we populate the time.Time representations of the start and end
	// times.
	if cfg.StartTimestamp != "" {
		err = cfg.parseStartEnd()
		if err != nil {
			reportError(w, r, req, "could not parse start and end times: %v", err)
			return
		}
	}

	// Load config information for any existing broadcasts that have been saved.
	req.BroadcastVars, err = model.GetVariablesBySite(ctx, settingsStore, sKey, broadcastScope)
	switch err {
	case nil, datastore.ErrNoSuchEntity:
	default:
		reportError(w, r, req, "could not get broadcast configs variable: %v", err)
		return
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
		reportError(w, r, req, "could not get site to establish timezone: %v", err)
		return
	}

	// Try to load existing broadcast settings for newly selected broadcast.
	var loaded bool
	action := stringToAction(req.Action, req)
	if action == broadcastSelect {
		loaded, err = loadExistingSettings(r, &req)
		if err != nil {
			reportError(w, r, req, "could not load existing settings for broadcast: %v", err)
			return
		}
	}

	// Get all Cameras and Controllers that could be used by the broadcast.
	devices, err := model.GetDevicesBySite(ctx, settingsStore, sKey)
	if err != nil {
		reportError(w, r, req, "could not get sites devices: %v", err)
		return
	}

	for _, dev := range devices {
		if dev.Type == model.DevTypeCamera {
			req.Cameras = append(req.Cameras, dev)
		} else if dev.Type == model.DevTypeController {
			req.Controllers = append(req.Controllers, dev)
		}
	}

	// If we just loaded existing broadcast settings by selecting a broadcast, write the response.
	if loaded {
		writeTemplate(w, r, "broadcast.html", &req, "")
		return
	}

	// Populate sensor list that contains sensors that will display values in
	// live chat.
	err = updateSensorList(ctx, &req, r, settingsStore)
	if err != nil {
		reportError(w, r, req, "could not update sensor list: %v", err)
		return
	}

	var msg string
	switch action {
	case broadcastToken:
		tokenURI := utils.TokenURIFromAccount(profile.Email)
		err = broadcast.AuthChannel(ctx, w, r, youtube.YoutubeScope, tokenURI)
		if err != nil {
			reportError(w, r, req, "could not authenticate channel: %v", err)
			return
		}

		// Store the account email in the broadcast config.
		cfg.Account = profile.Email
		err := saveBroadcast(ctx, &req.CurrentBroadcast)
		if err != nil {
			reportError(w, r, req, "could not save broadcast: %v", err)
			return
		}
		msg = "channel authenticated successfully"
	case broadcastSave:
		// If we haven't just generated a token we should keep the same account
		// that the config previously had.
		cfg.Account, err = getExistingAccount(req.BroadcastVars, cfg)
		if err != nil {
			reportError(w, r, req, "could not get existing account for name: %s: %v", cfg.Name, err)
			return
		}
		err := saveBroadcast(ctx, &req.CurrentBroadcast)
		if err != nil {
			reportError(w, r, req, "could not save broadcast: %v", err)
			return
		}
		msg = "broadcast saved successfully"

		// Ensure that the CheckBroadcast cron exists.
		c := &model.Cron{Skey: cfg.SKey, ID: "Broadcast Check", TOD: "* * * * *", Action: "rpc", Var: tvURL + "/checkbroadcasts", Enabled: true}
		err = model.PutCron(context.Background(), settingsStore, c)
		if err != nil {
			reportError(w, r, req, "warning: failed to verify checkbroadcasts cron: %v", err)
			return
		}

		err = cronScheduler.Set(c)
		if err != nil {
			reportError(w, r, req, "could not automatically set broadcast check cron in the scheduler: %v", err)
			return
		}

	case broadcastDelete:
		err = deleteBroadcast(ctx, &req, settingsStore)
		if err != nil {
			reportError(w, r, req, "could not delete broadcast: %v", err)
			return
		}
		msg = "broadcast deleted successfully"

	case vidforwardSlateUpdate:
		const fieldName = "slate-file"
		file, header, err := r.FormFile(fieldName)
		if err != nil {
			reportError(w, r, req, "could not get file from request form: %v", err)
			return
		}
		defer file.Close()
		err = (NewVidforwardService()).UploadSlate(cfg, header.Filename, file)
		if err != nil {
			reportError(w, r, req, "could not upload slate: %v", err)
			return

		}
		msg = "slate uploaded successfully"
	}

	writeTemplate(w, r, "broadcast.html", &req, msg)
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
			"broadcast-select":        broadcastSelect,
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
func saveBroadcast(ctx context.Context, cfg *Cfg) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error marshalling BroadcastConfig: %w", err)
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

// deleteBroadcast deletes a broadcast from the datastore and also updates the BroadcastVars
// list and CurrentBroadcast config to clear the form on next page write.
func deleteBroadcast(ctx context.Context, req *broadcastRequest, store datastore.Store) error {
	cfg := &req.CurrentBroadcast
	err := model.DeleteVariable(ctx, store, cfg.SKey, broadcastScope+"."+cfg.Name)
	if err != nil {
		return fmt.Errorf("could not delete broadcast: %v", err)
	}

	req.BroadcastVars, err = model.GetVariablesBySite(ctx, store, cfg.SKey, broadcastScope)
	switch err {
	case nil, datastore.ErrNoSuchEntity:
	default:
		return fmt.Errorf("could not get broadcast variables: %v", err)
	}

	req.CurrentBroadcast = BroadcastConfig{}
	return nil
}

func loadExistingSettings(r *http.Request, req *broadcastRequest) (bool, error) {
	// First check if a broadcast has been selected.
	selected := r.FormValue("broadcast-select")
	// If the selected value is nil, this means that we have selected the new
	// broadcast option. This should return a blank request.
	if selected == "" {
		req.CurrentBroadcast = BroadcastConfig{}
		return true, nil
	}
	log.Printf("existing broadcast selected: %s", selected)
	// Check that the broadcast name selected on the UI matches one of the
	// broadcast configs that we have loaded, and set current broadcast as that.
	cfg, err := broadcastFromVars(req.BroadcastVars, selected)
	if err != nil {
		return false, fmt.Errorf("could not get broadcast from vars: %w", err)
	}
	req.CurrentBroadcast = *cfg

	return true, nil
}

// getExistingAccount will return the current associated account of the broadcast with the current config
// name. This should be used to ensure that the associated account is only updated using the generate token method.
// If no broadcast/account is found, then an empty string will be returned
func getExistingAccount(broadcasts []model.Variable, cfg *BroadcastConfig) (string, error) {
	cfg, err := broadcastFromVars(broadcasts, cfg.Name)
	if errors.Is(err, ErrBroadcastNotFound{}) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return cfg.Account, nil
}

func updateSensorList(ctx context.Context, req *broadcastRequest, r *http.Request, store datastore.Store) error {
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
			entry := SensorEntry{
				SendMsg:   r.FormValue(strings.ToLower(sensor.Name)) == sensor.Name,
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

// saveLinkFunc provides a closure for saving a broadcast link with a given key.
func saveLinkFunc() func(string, string) error {
	return func(key, link string) error {
		key = removeDate(key)
		return model.PutVariable(context.Background(), settingsStore, -1, liveScope+"."+key, link)
	}
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
func liveHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	ctx := r.Context()
	setup(ctx)

	key := strings.ReplaceAll(r.URL.Path, r.URL.Host+"/live/", "")
	v, err := model.GetVariable(ctx, settingsStore, -1, liveScope+"."+key)
	if err != nil {
		fmt.Fprintf(w, "livestream %s does not exist", key)
		return
	}

	redirectURL := v.Value

	// Provide embed link if requested.
	if _, ok := r.URL.Query()["embed"]; ok {
		redirectURL = strings.ReplaceAll(redirectURL, "watch?v=", "embed/")
	}

	log.Printf("redirecting to livestream link, link: %s", redirectURL)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// writeHttpErrorAndLog is a wrapper for writeHttpError that adds logging.
func writeHttpErrorAndLog(w http.ResponseWriter, code int, err error) {
	writeHttpError(w, code, err.Error())
	log.Printf(err.Error())
}
