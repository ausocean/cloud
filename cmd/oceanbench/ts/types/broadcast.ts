// Broadcast Config
export type Broadcast = {
  UUID: string; // The immutable unique key of the broadcast.
  SKey: number; // The key of the site this broadcast belongs to.
  Name: string; // The name of the broadcast.
  BID: string; // Broadcast identification.
  SID: string; // Stream ID for any currently associated stream.
  CID: string; // ID of associated chat.
  StreamName: string; // The name of the stream we'll bind to the broadcast.
  Description: string; // The broadcast description shown below viewing window.
  LivePrivacy: string; // Privacy of the broadcast whilst live i.e. public, private or unlisted.
  PostLivePrivacy: string; // Privacy of the broadcast after it has ended i.e. public, private or unlisted.
  Resolution: string; // Resolution of the stream e.g. 1080p.
  StartTimestamp: string; // Start time of the broadcast in unix format.
  Start: Date; // Start time in native go format for easy operations.
  EndTimestamp: string; // End time of the broadcast in unix format.
  End: Date; // End time in native go format for easy operations.
  VidforwardHost: string; // Host address of vidforward service.
  CameraMac: number; // Camera hardware's MAC address.
  ControllerMAC: number; // Controller hardware's MAC adress (controller used to power camera).
  OnActions: string; // A series of actions to be used for power up of camera hardware.
  ShutdownActions: string; // A series of actions to be used for shutdown of camera hardware.
  OffActions: string; // A series of actions to be used for power down of camera hardware.
  RTMPVar: string; // The variable name that holds the RTMP URL and key.
  Active: boolean; // This is true if the broadcast is currently active i.e. waiting for data or currently streaming.
  Slate: boolean; // This is true if the broadcast is currently in slate mode i.e. no camera.
  Issues: number; // The number of successive stream issues currently experienced. Reset when good health seen.
  SendMsg: boolean; // True if sensor data will be sent to the YouTube live chat.
  SensorList: object; // List of sensors which can be reported to the YouTube live chat.
  RTMPKey: string; // The RTMP key corresponding to the newly created broadcast.
  UsingVidforward: boolean; // Indicates if we're using vidforward i.e. doing long term broadcast.
  CheckingHealth: boolean; // Are we performing health checks for the broadcast? Having this false is useful for dodgy testing streams.
  AttemptingToStart: boolean; // Indicates if we're currently attempting to start the broadcast.
  Enabled: boolean; // Is the broadcast enabled? If not, it will not be started.
  Events: string[]; // Holds names of events that are yet to be handled.
  Unhealthy: boolean; // True if the broadcast is unhealthy.
  BroadcastState: string; // Holds the current state of the broadcast.
  HardwareState: string; // Holds the current state of the hardware.
  StartFailures: number; // The number of times the broadcast has failed to start.
  Transitioning: boolean; // If the broadcast is transition from live to slate, or vice versa.
  StateData: string; // States will be marshalled and their data stored here.
  HardwareStateData: string; // Hardware states will be marshalled and their data stored here.
  Account: string; // The YouTube account email that this broadcast is associated with.
  InFailure: boolean; // True if the broadcast is in a failure state.
  BatteryVoltagePin: string; // The pin that the battery voltage is read from.
  RecoveringVoltage: boolean; // True if the broadcast is currently recovering voltage.
  RequiredStreamingVoltage: number; // The required battery voltage for the camera to stream.
  VoltageRecoveryTimeout: number; // Max allowable hours for voltage recovery before failure.
  RegisterOpenFish: boolean; // True if the video should be registered with openfish for annotation.
  OpenFishCaptureSource: string; // The capture source to register the stream to.
  NotifySuppressRules: string; // Suppression rules for notifications.
};
