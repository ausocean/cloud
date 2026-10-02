import { apiRequest, method } from "../shared/api";
import { type Device } from "./device";

// Variable represents a cloud variable, which stores abitrary
// string information. When the variable name includes a period, the
// portion to the left of the dot represents the scope. The scope is
// used to create namespaces, e.g., <MAC>.<var> represents a
// variable for a given device, whereas <var> represents a global
// variable for the entire site. Variables that start with an underscore,
// e.g., _<var>, are system variables which are typically hidden from
// users.
export class Variable {
  Skey: number = 0; // Site key.
  Scope: string = ""; // Scope, if any.
  Name: string = ""; // Variable name, which is any ID.
  Value: string = ""; // Variable value.
  Updated: Date = new Date(); // Date/time last updated.
}

// Gets device scoped (non-hidden) variables for a given site.
export function getVariablesBySite(skey: number): Promise<Variable[]> {
  return apiRequest<Variable[]>(`/api/get/${skey}/vars/site`, method.GET);
}

// Returns a map of each devices variables. The map key is the encoded MAC of the device, which
// maps to an array of Variables.
//
// NOTE: The variables are site scoped to the skey of the first device.
export async function getVariablesByDevices(devices: Device[]): Promise<Map<number, Variable[]>> {
  if (devices.length <= 0) {
    throw new Error("length of devices argument must be at least 1");
  }

  // Get all the variables for the site of the first device.
  let skey = devices[0].Skey;
  let vars: Variable[];
  try {
    vars = await getVariablesBySite(skey);
  } catch (e) {
    console.error(`unable to get variables for site (${skey}): ${e}`);
  }

  return new Map(
    devices.map((d) => {
      // Decode MAC to hexadecimal string.
      let mac = d.Mac.toString(16);
      return [d.Mac, vars.filter((v) => v.Scope == mac)];
    }),
  );
}
