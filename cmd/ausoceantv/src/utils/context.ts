import type { User } from "../types/user";
import { createContext } from "@lit/context";

export const userContext = createContext<User | null>(Symbol("current-user"));
