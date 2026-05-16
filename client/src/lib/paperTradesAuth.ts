/**
 * Paper-trades Supabase account key resolution and API auth guards (P1-L).
 */

export type PaperTradesAuthFailureStatus = 401 | 503;

export type RequireAuthenticatedUserResult =
  | { ok: true; userId: string }
  | { ok: false; status: PaperTradesAuthFailureStatus; error: string };

export type ResolvePaperTradesAccountKeyInput = {
  /** Authenticated Supabase user id (auth.users). */
  supabaseUserId?: string | null;
  /** localStorage namespace for desk UI state (not used for cloud when logged out). */
  storageNamespace: string;
};

/**
 * Cloud Supabase `account_key`: authenticated user id only.
 * Logged out → `null` (local paper continues; cloud APIs return 401).
 */
export function resolveCloudPaperTradesAccountKey(
  input: ResolvePaperTradesAccountKeyInput,
): string | null {
  const id = input.supabaseUserId?.trim();
  return id ? id : null;
}

/** @deprecated Use resolveCloudPaperTradesAccountKey — kept for tests naming clarity. */
export function resolvePaperTradesAccountKeyForCloud(
  input: ResolvePaperTradesAccountKeyInput,
): string | null {
  return resolveCloudPaperTradesAccountKey(input);
}

export function requireAuthenticatedPaperUser(
  userId: string | null | undefined,
): RequireAuthenticatedUserResult {
  const id = userId?.trim();
  if (!id) {
    return { ok: false, status: 401, error: "Sign in required" };
  }
  return { ok: true, userId: id };
}

export function assertCloudAccountMatchesSession(
  sessionUserId: string,
  requestedAccountKey: string | null | undefined,
): RequireAuthenticatedUserResult {
  if (!requestedAccountKey?.trim()) {
    return { ok: true, userId: sessionUserId };
  }
  if (requestedAccountKey.trim() !== sessionUserId) {
    return { ok: false, status: 401, error: "account_key does not match session" };
  }
  return { ok: true, userId: sessionUserId };
}
