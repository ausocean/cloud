import { User } from "../types/user";

export type Permissions = {
  broadcastEditor: boolean;
};

const ADMIN_PERMISSIONS: Permissions = { broadcastEditor: true };
const USER_PERMISSIONS: Permissions = { broadcastEditor: false };

const permissionMap: Map<string, Permissions> = new Map([
  ["admin", ADMIN_PERMISSIONS],
  ["user", USER_PERMISSIONS],
]);

export function hasPermission(user: User, requiredPermissions: string): boolean {
  const permissions = permissionMap.get(user.role);
  if (!permissions) return false;

  console.log("Permissions:", permissions);

  const requiredPermissionsArray = requiredPermissions.split(",");
  console.log("Required Permissions:", requiredPermissions);
  if (requiredPermissionsArray.length == 0 || requiredPermissionsArray[0] == "") return true;

  return requiredPermissionsArray.every((perm) => Boolean(permissions[perm as keyof Permissions]));
}
