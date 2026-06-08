import type { Time } from "lightweight-charts";

/** Normalize epoch ms or seconds (or ISO string) to finite epoch ms, else 0. */
export function coerceEpochMs(value: unknown): number {
  if (typeof value === "number" && Number.isFinite(value) && value > 0) {
    return value < 1e12 ? value * 1000 : value;
  }
  if (typeof value === "string" && value.trim()) {
    const parsed = Date.parse(value);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
  }
  return 0;
}

/** lightweight-charts UTCTimestamp (seconds). Returns null when invalid. */
export function toUtcChartTime(epochMs: unknown): Time | null {
  const ms = coerceEpochMs(epochMs);
  if (ms <= 0) return null;
  const seconds = Math.floor(ms / 1000);
  return seconds > 0 ? (seconds as Time) : null;
}

/** Minute-aligned UTCTimestamp for markers and intraday series. */
export function toMinuteUtcChartTime(epochMs: unknown): Time | null {
  const ms = coerceEpochMs(epochMs);
  if (ms <= 0) return null;
  const seconds = Math.floor(ms / 60_000) * 60;
  return seconds > 0 ? (seconds as Time) : null;
}
