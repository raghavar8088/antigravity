/**
 * Worker lease utilities.
 *
 * Synchronous helpers (take a stored timestamp) — used by UI components and
 * the cron route, which already have the timestamp from MongoDB state.
 *
 * Async MongoDB helpers — re-exported from deskWorkerLease for the worker
 * script and the cron route when it needs the raw lease doc.
 */

export const WORKER_LEASE_TTL_MS = 45_000;

/** Synchronous — returns true if the stored timestamp is within TTL. */
export function isWorkerHeartbeatFresh(
  workerLastPollAt: number | null | undefined,
  ttlMs = WORKER_LEASE_TTL_MS,
): boolean {
  if (workerLastPollAt == null) return false;
  return Date.now() - workerLastPollAt < ttlMs;
}

/** Synchronous — returns age in whole seconds, or null if never set. */
export function workerHeartbeatAgeSeconds(
  workerLastPollAt: number | null | undefined,
): number | null {
  if (workerLastPollAt == null) return null;
  return Math.floor((Date.now() - workerLastPollAt) / 1000);
}

// Async MongoDB helpers (for the worker script)
export {
  acquireWorkerLease,
  workerHeartbeat,
  releaseWorkerLease,
} from "@/lib/mongoTradesClient";
