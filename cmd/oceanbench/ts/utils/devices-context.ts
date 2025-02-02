import { createContext } from "@lit/context";
import { Devices } from "../types/device";

export const devicesContext = createContext<Devices>(Symbol("devicesContext"));
