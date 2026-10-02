import { apiRequest, method } from "../shared/api";

// Device represents a NetSender Device.
export class Device {
  Skey: number = 0; // Site key.
  Dkey: number = 0; // Device key.
  Mac: number = 0; // Encoded MAC address (immutable).
  Name: string = ""; // Device name.
  Inputs: string = ""; // Input pins.
  Outputs: string = ""; // Output pins.
  Wifi: string = ""; // Wifi credentials, if any.
  MonitorPeriod: number = 60; // Monitor period (s).
  ActPeriod: number = 60; // Actuation period (s)
  Type: string = ""; // Client type.
  Version: string = ""; // Client version.
  Protocol: string = ""; // Client protocol.
  Status: number = 0; // Status code.
  Latitude: number = 0; // Device latitude.
  Longitude: number = 0; // Device longtitude.
  Enabled: boolean = false; // True if enabled, false otherwise.
  Updated: Date = new Date(); // Date/time last updated.

  // Gets the actuators for the device.
  getActuators(): Promise<Actuator[]> {
    return apiRequest<Actuator[]>(`/api/v1/get/actuators/${this.Mac}`, method.GET);
  }
}

// An actuator sends the value of a variable to a device's output pin.
export type Actuator = {
  Name: string; // Name of actuator (mutable).
  Mac: number; // MAC address of associated device (immutable).
  Pin: string; // Pin of associated device (immutable).
  Var: string; // Relative name of device variable (mutable).
};

// Gets all devices for a given site.
export function getDevicesBySite(skey: number): Promise<Device[]> {
  return apiRequest<Device[]>(`/api/get/${skey}/devices/site`, method.GET);
}
