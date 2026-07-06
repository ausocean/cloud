import { apiRequest, method } from "../shared/api";

// Log represents a log input regarding a device and/or site.
export type Log = {
  UUID: string; // Log ID.
  Skey: number; // Site key.
  DeviceMAC: number; // Encoded MAC address of a device.
  Note: string; // Notes made about device or site.
  Created: Date; // Time the log was written.
  Level: string; // Log level of importance.
};

// getLogsByDevice fetches all logs related to the passed
// encoded MAC of a device.
export function getLogsByDevice(MAC: number): Promise<Log[]> {
  let getLogsURL = new URL(`/api/v1/logs/${MAC}`, window.location.origin);
  return apiRequest(getLogsURL.toString(), method.GET);
}

export function putNewLog(MAC: number, data: FormData): Promise<Log> {
  let putLogsURL = new URL(`/api/v1/log/${MAC}`, window.location.origin);
  return apiRequest(putLogsURL.toString(), method.PUT, data);
}
