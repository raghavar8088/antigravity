/**
 * Anonymous device account key for sign-in-less paper trade persistence.
 *
 * Gated by NEXT_PUBLIC_ALLOW_ANON_PAPER_TRADES=1 (client) and ALLOW_ANON_PAPER_TRADES=1
 * (server, in route.ts). Both must be set; client decides whether to send, server
 * decides whether to accept.
 *
 * Security trade-off: when enabled, ANY caller can POST to /api/paper-trades and
 * write rows under a chosen account_key. For personal/research use on an obscure
 * Vercel URL this is acceptable; for any multi-tenant deployment do NOT enable.
 */

const STORAGE_KEY = "anon_paper_trade_account_key";
const ANON_PREFIX = "anon_";

function newId(): string {
  // crypto.randomUUID is available in modern browsers and Node 19+
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return ANON_PREFIX + crypto.randomUUID();
  }
  // Fallback: timestamp + random
  return ANON_PREFIX + Date.now().toString(36) + "-" + Math.random().toString(36).slice(2, 10);
}

/**
 * Returns the persistent anonymous account key for this browser when the feature
 * flag is enabled. Creates and stores one on first call. Returns null when the
 * flag is off OR when localStorage is unavailable (SSR / private mode).
 */
export function getOrCreateAnonAccountKey(): string | null {
  if (process.env.NEXT_PUBLIC_ALLOW_ANON_PAPER_TRADES !== "1") return null;
  if (typeof localStorage === "undefined") return null;
  try {
    const existing = localStorage.getItem(STORAGE_KEY);
    if (existing && existing.startsWith(ANON_PREFIX)) return existing;
    const fresh = newId();
    localStorage.setItem(STORAGE_KEY, fresh);
    return fresh;
  } catch {
    return null;
  }
}

/** Server-side check: is the supplied key shaped like our anonymous key? */
export function isAnonAccountKey(key: string | null | undefined): boolean {
  return typeof key === "string" && key.startsWith(ANON_PREFIX) && key.length > ANON_PREFIX.length;
}
