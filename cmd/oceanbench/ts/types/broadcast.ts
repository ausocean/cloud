import { getSkey } from "../shared/skey";
import { apiRequest, method } from "../shared/api";

// Broadcast Config
export type ActionVar = { name: string; value: string };
export class Broadcast {
  // __cont
  UUID: string = ""; // The immutable unique key of the broadcast.
  SKey: number = getSkey(); // The key of the site this broadcast belongs to.
  Name: string = ""; // The name of the broadcast.
  BID: string = ""; // Broadcast identification.
  SID: string = ""; // Stream ID for any currently associated stream.
  CID: string = ""; // ID of associated chat.
  StreamName: string = ""; // The name of the stream we'll bind to the broadcast.
  Description: string = ""; // The broadcast description shown below viewing window.
  LivePrivacy: string = ""; // Privacy of the broadcast whilst live i.e. public, private or unlisted.
  PostLivePrivacy: string = ""; // Privacy of the broadcast after it has ended i.e. public, private or unlisted.
  Resolution: string = ""; // Resolution of the stream e.g. 1080p.
  StartTimestamp: string = ""; // Start time of the broadcast in unix format.
  Start: Date = new Date(); // Start time in native go format for easy operations.
  EndTimestamp: string = ""; // End time of the broadcast in unix format.
  End: Date = new Date(); // End time in native go format for easy operations.
  VidforwardHost: string = ""; // Host address of vidforward service.
  CameraMac: number = 0; // Camera hardware's MAC address.
  ControllerMAC: number = 0; // Controller hardware's MAC adress (controller used to power camera).
  OnActions: ActionVar[] = []; // Ordered actions to be used for power up of camera hardware.
  ShutdownActions: ActionVar[] = []; // Ordered actions to be used for shutdown of camera hardware.
  OffActions: ActionVar[] = []; // Ordered actions to be used for power down of camera hardware.
  RTMPVar: string = ""; // The variable name that holds the RTMP URL and key.
  AuthKeyVar: string = ""; // The variable name that holds the authentication key for the broadcast host.
  StorageConfigVar: string = ""; // The variable name that holds the storage configuration for OceanMedia broadcasts.
  CameraOutputVar: string = ""; // The variable name that sets the output mode of the camera.
  Active: boolean = false; // This is true if the broadcast is currently active i.e. waiting for data or currently streaming.
  Slate: boolean = false; // This is true if the broadcast is currently in slate mode i.e. no camera.
  Issues: number = 0; // The number of successive stream issues currently experienced. Reset when good health seen.
  SendMsg: boolean = false; // True if sensor data will be sent to the YouTube live chat.
  SensorList: object = []; // List of sensors which can be reported to the YouTube live chat.
  RTMPKey: string = ""; // The RTMP key corresponding to the newly created broadcast.
  AuthKey: string = ""; // The authentication key for the broadcast host.
  StorageConfig?: StorageConfig; // The storage configuration for OceanMedia broadcasts.
  BroadcastHost: string = ""; // The broadcast host (e.g. "youtube", "oceanmedia").
  UsingVidforward: boolean = false; // Indicates if we're using vidforward i.e. doing long term broadcast.
  CheckingHealth: boolean = false; // Are we performing health checks for the broadcast? Having this false is useful for dodgy testing streams.
  AttemptingToStart: boolean = false; // Indicates if we're currently attempting to start the broadcast.
  Enabled: boolean = false; // Is the broadcast enabled? If not, it will not be started.
  Events: string[] = []; // Holds names of events that are yet to be handled.
  Unhealthy: boolean = false; // True if the broadcast is unhealthy.
  BroadcastState: string = ""; // Holds the current state of the broadcast.
  HardwareState: string = ""; // Holds the current state of the hardware.
  StartFailures: number = 0; // The number of times the broadcast has failed to start.
  Transitioning: boolean = false; // If the broadcast is transition from live to slate, or vice versa.
  StateData: string = ""; // States will be marshalled and their data stored here.
  HardwareStateData: string = ""; // Hardware states will be marshalled and their data stored here.
  Account: string = ""; // The YouTube account email that this broadcast is associated with.
  InFailure: boolean = false; // True if the broadcast is in a failure state.
  BatteryVoltagePin: string = ""; // The pin that the battery voltage is read from.
  RecoveringVoltage: boolean = false; // True if the broadcast is currently recovering voltage.
  RequiredStreamingVoltage: number = 0; // The required battery voltage for the camera to stream.
  VoltageRecoveryTimeout: number = 0; // Max allowable hours for voltage recovery before failure.
  RegisterOpenFish: boolean = false; // True if the video should be registered with openfish for annotation.
  OpenFishCaptureSource: string = ""; // The capture source to register the stream to.
  NotifySuppressRules: string = ""; // Suppression rules for notifications.
}

// YouTube privacy levels for live and post live streams.
export const privacyLevels = ["unlisted", "private", "public"];

export async function getBroadcastsBySite(skey: number): Promise<Map<string, Broadcast>> {
  const ids = await apiRequest<string[]>(`/api/v1/${skey}/broadcasts`, method.GET);
  return getBroadcastsByIDs(ids);
}

async function getBroadcastsByIDs(ids: string[]): Promise<Map<string, Broadcast>> {
  try {
    const broadcasts = ids.map(async (id) => {
      const parts = id.split(".");
      const skey = Number(parts[0]);
      if (skey == 0) {
        throw new Error(`unable to parse skey from broadcast ID: ${id}`);
      }
      const uuid = parts[2];
      return getBroadcast(skey, uuid);
    });
    let broadcastArray = await Promise.all(broadcasts);
    return new Map(broadcastArray.map((b) => [b.UUID, b]));
  } catch (e) {
    console.error("unable to get broadcasts:", e);
    return new Map();
  }
}

export async function getBroadcast(skey: number, uuid: string): Promise<Broadcast> {
  if (skey == 0) {
    throw new Error(`skey must not be 0`);
  }
  return await apiRequest<Broadcast>(`/api/v1/${skey}/broadcasts/${uuid}`, method.GET);
}

// StorageConfig contains the information for the storage of an oceanmedia broadcast.
export type StorageConfig = {
  Bucket: string; // The destination bucket.
  Prefix: string; // The prefix (directory) to save the segments to.
  Provider: string; // The storage provider (e.g. "cloudflare").
  Endpoint: string; // The endpoint of the storage service.
};
