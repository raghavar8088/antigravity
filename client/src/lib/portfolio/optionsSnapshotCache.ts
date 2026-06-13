const KEY_BUY = "raig.options.buy.snapshot.v1";
const KEY_SELL = "raig.options.sell.snapshot.v1";
const MAX_TRADES = 500;

export type OptionsSnapshotCache = {
  positions: unknown[];
  trades: unknown[];
  strategies: unknown[];
  stats: unknown | null;
};

function empty(): OptionsSnapshotCache {
  return { positions: [], trades: [], strategies: [], stats: null };
}

function readKey(key: string): OptionsSnapshotCache {
  if (typeof window === "undefined") {
    return empty();
  }
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return empty();
    const j = JSON.parse(raw) as Record<string, unknown>;
    return {
      positions: Array.isArray(j.positions) ? j.positions : [],
      trades: Array.isArray(j.trades) ? (j.trades as unknown[]).slice(0, MAX_TRADES) : [],
      strategies: Array.isArray(j.strategies) ? j.strategies : [],
      stats: j.stats && typeof j.stats === "object" ? j.stats : null,
    };
  } catch {
    return empty();
  }
}

function writeKey(
  key: string,
  payload: { positions: unknown[]; trades: unknown[]; strategies: unknown[]; stats: unknown | null },
) {
  if (typeof window === "undefined") return;
  try {
    const body = {
      positions: payload.positions,
      trades: (payload.trades ?? []).slice(0, MAX_TRADES),
      strategies: payload.strategies,
      stats: payload.stats,
      savedAt: Date.now(),
    };
    localStorage.setItem(key, JSON.stringify(body));
  } catch {
    // quota or private mode
  }
}

export function readOptionsBuyCache(): OptionsSnapshotCache {
  return readKey(KEY_BUY);
}

export function writeOptionsBuyCache(payload: {
  positions: unknown[];
  trades: unknown[];
  strategies: unknown[];
  stats: unknown | null;
}) {
  writeKey(KEY_BUY, payload);
}

export function clearOptionsBuyCache() {
  if (typeof window === "undefined") return;
  try {
    localStorage.removeItem(KEY_BUY);
  } catch {
    /* ignore */
  }
}

export function readOptionsSellCache(): OptionsSnapshotCache {
  return readKey(KEY_SELL);
}

export function writeOptionsSellCache(payload: {
  positions: unknown[];
  trades: unknown[];
  strategies: unknown[];
  stats: unknown | null;
}) {
  writeKey(KEY_SELL, payload);
}

export function clearOptionsSellCache() {
  if (typeof window === "undefined") return;
  try {
    localStorage.removeItem(KEY_SELL);
  } catch {
    /* ignore */
  }
}
