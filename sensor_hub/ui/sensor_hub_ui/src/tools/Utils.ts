import type {AuthUser} from "../providers/AuthContext.tsx";

export const hasPerm = (user: AuthUser | undefined, perm: string) => {
  if (!user) return false;
  if (user.permissions && user.permissions.includes(perm)) return true;
  // fallback to admin role for compatibility
  if (user.roles && user.roles.includes('admin')) return true;
  return false;
}
