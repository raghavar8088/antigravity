const PLACE_ORDER_MAX_PER_HOUR = 10;
const WINDOW_MS = 60 * 60 * 1000;

const placeOrderTimestamps = new Map<string, number[]>();

export type RateLimitResult =
  | { allowed: true; remaining: number }
  | { allowed: false; remaining: 0; retryAfterSec: number };

function pruneWindow(timestamps: number[], now: number): number[] {
  const cutoff = now - WINDOW_MS;
  return timestamps.filter((t) => t >= cutoff);
}

/** In-memory: max 10 place-order requests per user per rolling hour. */
export function checkTestnetPlaceOrderRateLimit(
  userId: string,
  now = Date.now(),
): RateLimitResult {
  const key = userId.trim();
  const prev = placeOrderTimestamps.get(key) ?? [];
  const windowed = pruneWindow(prev, now);
  if (windowed.length >= PLACE_ORDER_MAX_PER_HOUR) {
    const oldest = windowed[0] ?? now;
    const retryAfterSec = Math.max(1, Math.ceil((oldest + WINDOW_MS - now) / 1000));
    return { allowed: false, remaining: 0, retryAfterSec };
  }
  return { allowed: true, remaining: PLACE_ORDER_MAX_PER_HOUR - windowed.length };
}

export function recordTestnetPlaceOrder(userId: string, now = Date.now()): void {
  const key = userId.trim();
  const windowed = pruneWindow(placeOrderTimestamps.get(key) ?? [], now);
  windowed.push(now);
  placeOrderTimestamps.set(key, windowed);
}

/** Test helper */
export function resetTestnetPlaceOrderRateLimitForTests(): void {
  placeOrderTimestamps.clear();
}

export const TESTNET_PLACE_ORDER_LIMIT_PER_HOUR = PLACE_ORDER_MAX_PER_HOUR;
