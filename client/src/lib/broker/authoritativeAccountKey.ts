/**
 * Single authoritative account key for all Paper Desk MongoDB reads/writes.
 *
 * Every subsystem MUST use resolveAuthoritativeAccountKey() instead of
 * DESK_WORKER_ACCOUNT_KEY or client-supplied keys. The Go engine uses the
 * same value via OWNER_ACCOUNT_KEY env (default: mock_trading_default).
 */

import { OWNER_ACCOUNT_KEY } from "./ownerAccountKey";

/** Must match engine/internal/paperpersist/accountkey.go FrontendAccountKey */
export const FRONTEND_OWNER_ACCOUNT_KEY = OWNER_ACCOUNT_KEY;

export type AccountKeyValidation = {
  ok: boolean;
  accountKey: string;
  errors: string[];
  warnings: string[];
};

/**
 * Returns the one account key all server-side code must use.
 * Throws if env vars point at a retired anon_* key or a mismatched OWNER_ACCOUNT_KEY.
 */
export function resolveAuthoritativeAccountKey(): string {
  const result = validateAccountKeyAlignment();
  if (!result.ok) {
    throw new Error(result.errors.join("; "));
  }
  return result.accountKey;
}

/**
 * Non-throwing validation for health endpoints and startup logs.
 */
export function validateAccountKeyAlignment(): AccountKeyValidation {
  const errors: string[] = [];
  const warnings: string[] = [];

  const envOwner = process.env.OWNER_ACCOUNT_KEY?.trim() ?? "";
  const legacyWorker = process.env.DESK_WORKER_ACCOUNT_KEY?.trim() ?? "";

  if (envOwner && envOwner !== FRONTEND_OWNER_ACCOUNT_KEY) {
    errors.push(
      `OWNER_ACCOUNT_KEY="${envOwner}" must match frontend constant "${FRONTEND_OWNER_ACCOUNT_KEY}"`,
    );
  }

  if (legacyWorker.startsWith("anon_")) {
    errors.push(
      `DESK_WORKER_ACCOUNT_KEY="${legacyWorker}" is a retired anonymous key — use "${FRONTEND_OWNER_ACCOUNT_KEY}"`,
    );
  } else if (legacyWorker && legacyWorker !== FRONTEND_OWNER_ACCOUNT_KEY) {
    warnings.push(
      `DESK_WORKER_ACCOUNT_KEY="${legacyWorker}" is deprecated — all paths now use "${FRONTEND_OWNER_ACCOUNT_KEY}"`,
    );
  }

  return {
    ok: errors.length === 0,
    accountKey: FRONTEND_OWNER_ACCOUNT_KEY,
    errors,
    warnings,
  };
}

/** @deprecated Anonymous account keys are retired. Always returns false for legacy keys. */
export function isRetiredAnonAccountKey(key: string | null | undefined): boolean {
  return typeof key === "string" && key.startsWith("anon_");
}
