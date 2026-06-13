/**
 * Anonymous account key — DISABLED.
 *
 * This module previously generated per-device localStorage UUIDs that caused
 * account fragmentation across desktop / mobile / laptop. All functions now
 * return null so every code path is forced to use the authenticated session
 * (account_key = "owner_admin" via JWT cookie).
 *
 * Do not re-enable. If you need the old behaviour, see git history.
 */

export function getOrCreateAnonAccountKey(): null {
  return null;
}

export function isAnonAccountKey(_key: string | null | undefined): boolean {
  return false;
}
