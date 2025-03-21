import type { User } from "../types/user";
import { createContext } from "@lit/context";

export const userContext = createContext<User>(Symbol("current-user"));
