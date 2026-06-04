package broadcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/model"
)

const Scope = "Broadcast" // Scope under which broadcast configs are stored.

// Config holds configuration data for a YouTube broadcast.
type Config struct {
	UUID                     string        // The immutable unique key of the broadcast.
	SKey                     int64         // The key of the site this broadcast belongs to.
	Name                     string        // The name of the broadcast.
	BID                      string        // Broadcast identification.
	SID                      string        // Stream ID for any currently associated stream.
	CID                      string        // ID of associated chat.
	StreamName               string        // The name of the stream we'll bind to the broadcast.
	Description              string        // The broadcast description shown below viewing window.
	LivePrivacy              string        // Privacy of the broadcast whilst live i.e. public, private or unlisted.
	PostLivePrivacy          string        // Privacy of the broadcast after it has ended i.e. public, private or unlisted.
	Resolution               string        // Resolution of the stream e.g. 1080p.
	StartTimestamp           string        // Start time of the broadcast in unix format.
	Start                    time.Time     // Start time in native go format for easy operations.
	EndTimestamp             string        // End time of the broadcast in unix format.
	End                      time.Time     // End time in native go format for easy operations.
	VidforwardHost           string        // Host address of vidforward service.
	CameraMac                int64         // Camera hardware's MAC address.
	ControllerMAC            int64         // Controller hardware's MAC adress (controller used to power camera).
	OnActions                string        // A series of actions to be used for power up of camera hardware.
	ShutdownActions          string        // A series of actions to be used for shutdown of camera hardware.
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
	BatteryVoltagePin        string        // The pin that the battery voltage is read from.
	RecoveringVoltage        bool          // True if the broadcast is currently recovering voltage.
	RequiredStreamingVoltage float64       // The required battery voltage for the camera to stream.
	VoltageRecoveryTimeout   int           // Max allowable hours for voltage recovery before failure.
	RegisterOpenFish         bool          // True if the video should be registered with openfish for annotation.
	OpenFishCaptureSource    string        // The capture source to register the stream to.
	NotifySuppressRules      string        // Suppression rules for notifications.
}

func (b *Config) PrettyHardwareStateData() string {
	return string(b.HardwareStateData)
}

// parseStartEnd takes the start and end time unix strings from the broadcast
// and provides these as time.Time.
func (c *Config) parseStartEnd() error {
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

// LogForBroadcast logs a message with the broadcast name and ID.
// This is useful to keep track of logs for different broadcasts.
func LogForBroadcast(cfg *Config, output func(v ...any), msg string, args ...interface{}) {
	output(FmtForBroadcastLog(cfg, msg, args...))
}

func FmtForBroadcastLog(cfg *Config, msg string, args ...interface{}) string {
	idArgs := []interface{}{cfg.Name, cfg.BroadcastState, cfg.HardwareState, cfg.BID}
	idArgs = append(idArgs, args...)
	return fmt.Sprintf("(name: %s, broadcast state: %s, hardware state: %s, id: %s) "+msg, idArgs...)
}

// TODO: document this.
func UpdateConfigWithTransaction(ctx context.Context, store datastore.Store, skey int64, broadcast string, update func(cfg *Config)) error {
	name := Scope + "." + broadcast
	sep := strings.Index(name, ".")
	if sep >= 0 {
		name = strings.ReplaceAll(name[:sep], ":", "") + name[sep:]
	}
	const typeVariable = "Variable"
	key := store.NameKey(typeVariable, strconv.FormatInt(skey, 10)+"."+name)

	var callBackErr error
	updateConfig := func(ety datastore.Entity) {
		v, ok := ety.(*model.Variable)
		if !ok {
			callBackErr = errors.New("could not cast entity to type Variable")
			return
		}

		var cfg Config
		err := json.Unmarshal([]byte(v.Value), &cfg)
		if err != nil {
			callBackErr = fmt.Errorf("could not unmarshal selected broadcast config: %v", err)
			return
		}

		update(&cfg)

		d, err := json.Marshal(cfg)
		if err != nil {
			callBackErr = fmt.Errorf("could not marshal JSON for broadcast save: %w", err)
			return
		}

		v.Value = string(d)
		v.Updated = time.Now()
	}

	err := store.Update(ctx, key, updateConfig, &model.Variable{})
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		err = store.Create(ctx, key, &model.Variable{})
		if err != nil {
			return fmt.Errorf("could not create broadcast variable: %w", err)
		}

		// Since the entity doesn't already exist, we need to change the updateConfig function to update
		// a blank config.
		updateConfig = func(ety datastore.Entity) {
			v, ok := ety.(*model.Variable)
			if !ok {
				callBackErr = errors.New("could not cast entity to type Variable")
				return
			}

			cfg := &Config{}
			update(cfg)

			v.Skey = skey
			v.Name = name
			v.Scope = Scope

			d, err := json.Marshal(cfg)
			if err != nil {
				callBackErr = fmt.Errorf("could not marshal JSON for broadcast save: %w", err)
				return
			}

			v.Value = string(d)
			v.Updated = time.Now()
		}

		err = store.Update(ctx, key, updateConfig, &model.Variable{})
		if err != nil {
			return fmt.Errorf("could not update broadcast variable after creation: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("could not update variable: %w", err)
	}

	if callBackErr != nil {
		return fmt.Errorf("error from broadcast update callback: %w", callBackErr)
	}

	return nil
}

// SensorEntry contains the information for each sensor.
type SensorEntry struct {
	SendMsg   bool
	Sensor    model.SensorV2
	Name      string
	DeviceMac int64
}
