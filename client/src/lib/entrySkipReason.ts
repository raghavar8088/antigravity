/**
 * Lightweight ring-buffer skip-reason registry for the paper desk entry pipeline.
 * Callers log why a candidate was rejected; the UI reads a frequency summary.
 * Module-level singleton — reset on session clear via resetSkipReasonLog().
 */

export type SkipReason =
  | "THRESHOLD_NOT_MET"
  | "CORRELATED_CAP"
  | "ATR_TOO_LOW"
  | "REGIME_BLOCKED"
  | "COOLDOWN"
  | "OCCUPIED"
  | "DISABLED"
  | "SPREAD"
  | "SESSION_WINDOW"
  | "CATEGORY_CAP"
  | "SAME_DIR_CAP"
  | "MARGIN"
  | string;

export type SkipReasonEntry = {
  stratId: number;
  reason: SkipReason;
  meta?: Record<string, unknown>;
  ts: number;
};

const RING_CAP = 500;
const _log: SkipReasonEntry[] = [];

export function logSkipReason(
  stratId: number,
  reason: SkipReason,
  meta?: Record<string, unknown>,
): void {
  if (_log.length >= RING_CAP) _log.shift();
  _log.push({ stratId, reason, meta, ts: Date.now() });
}

export type SkipReasonSummaryRow = { reason: string; count: number };

export function getSkipReasonSummary(windowMs = 60_000): SkipReasonSummaryRow[] {
  const cutoff = Date.now() - windowMs;
  const counts = new Map<string, number>();
  for (const e of _log) {
    if (e.ts < cutoff) continue;
    counts.set(e.reason, (counts.get(e.reason) ?? 0) + 1);
  }
  return [...counts.entries()]
    .map(([reason, count]) => ({ reason, count }))
    .sort((a, b) => b.count - a.count);
}

export function resetSkipReasonLog(): void {
  _log.length = 0;
}
