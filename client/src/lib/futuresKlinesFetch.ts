/**
 * Delta India 1m futures candles for replay fixtures (server/CLI; no Next.js).
 */

import type { ReplayCandle } from "@/lib/futuresReplayFixtures";

const DELTA_REST_BASE =
  process.env.DELTA_API_BASE_URL?.replace(/\/+$/, "") ?? "https://api.india.delta.exchange";

const JSON_HEADERS = {
  Accept: "application/json",
  "User-Agent": "RAIG-Trading/1.0",
};

/**
 * Retry/backoff for Delta REST calls (P2.5.1).
 *
 * - 3 attempts total
 * - Exponential backoff with jitter: 200ms, 400ms, 800ms (±25% jitter)
 * - Retries on 5xx responses and network errors only; 4xx are surfaced immediately
 */
export async function fetchWithRetry(
  url: string,
  init?: RequestInit,
  opts: { attempts?: number; baseDelayMs?: number } = {},
): Promise<Response> {
  const attempts = opts.attempts ?? 3;
  const base = opts.baseDelayMs ?? 200;
  let lastErr: unknown;
  for (let i = 0; i < attempts; i++) {
    try {
      const res = await fetch(url, init);
      if (res.ok || res.status < 500) return res; // success or non-retryable 4xx
      lastErr = new Error(`HTTP ${res.status}`);
    } catch (err) {
      lastErr = err;
    }
    if (i < attempts - 1) {
      const backoff = base * Math.pow(2, i);
      const jitter = backoff * (0.75 + Math.random() * 0.5);
      await new Promise((r) => setTimeout(r, jitter));
    }
  }
  throw lastErr instanceof Error ? lastErr : new Error(String(lastErr));
}

function n(value: unknown): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

/** Delta may return Unix seconds; replay uses epoch ms. */
export function normalizeCandleTimeMs(t: number): number {
  if (!Number.isFinite(t) || t <= 0) return 0;
  return t < 1e12 ? Math.round(t * 1000) : Math.round(t);
}

/**
 * Deduplicate candles by timestamp and sort ascending.
 * Pure function — safe to test and call repeatedly.
 */
export function deduplicateAndSortCandles(candles: ReplayCandle[]): ReplayCandle[] {
  const seen = new Map<number, ReplayCandle>();
  for (const c of candles) {
    seen.set(c.time, c);
  }
  return [...seen.values()].sort((a, b) => a.time - b.time);
}

/**
 * Count timestamp gaps > `gapThresholdMs` between consecutive sorted candles.
 * Useful for reporting missing data (exchange downtime, API holes, etc.).
 */
export function countCandleGaps(
  sortedCandles: ReplayCandle[],
  gapThresholdMs = 120_000,
): number {
  let gaps = 0;
  for (let i = 1; i < sortedCandles.length; i++) {
    if (sortedCandles[i]!.time - sortedCandles[i - 1]!.time > gapThresholdMs) {
      gaps++;
    }
  }
  return gaps;
}

export type FetchDelta1mResult = {
  symbol: string;
  candles: ReplayCandle[];
  fundingRate: number;
  fetchedAt: string;
  source: string;
};

type RawCandleRow = {
  time?: unknown;
  open?: unknown;
  high?: unknown;
  low?: unknown;
  close?: unknown;
  volume?: unknown;
};

function parseRawRow(row: RawCandleRow): ReplayCandle | null {
  const close = n(row.close);
  if (close <= 0) return null;
  const timeMs = normalizeCandleTimeMs(n(row.time));
  if (timeMs <= 0) return null;
  return {
    time: timeMs,
    open: n(row.open) || close,
    high: n(row.high) || close,
    low: n(row.low) || close,
    close,
    volume: n(row.volume),
  };
}

/**
 * Fetch up to `bars` recent 1m candles for a perpetual symbol (single call).
 * Kept for backward compatibility with replay:fetch without --days.
 */
export async function fetchDeltaFutures1mCandles(
  symbol: string,
  bars: number,
): Promise<FetchDelta1mResult> {
  const sym = symbol.trim().toUpperCase() || "BTCUSD";
  const count = Math.min(2000, Math.max(50, Math.floor(bars)));
  const endSec = Math.floor(Date.now() / 1000);
  const startSec = endSec - count * 60 - 120;

  const [candlesRes, tickerRes] = await Promise.all([
    fetchWithRetry(
      `${DELTA_REST_BASE}/v2/history/candles?resolution=1m&symbol=${encodeURIComponent(sym)}&start=${startSec}&end=${endSec}`,
      { headers: JSON_HEADERS, cache: "no-store" },
    ),
    fetchWithRetry(`${DELTA_REST_BASE}/v2/tickers/${encodeURIComponent(sym)}`, {
      headers: JSON_HEADERS,
      cache: "no-store",
    }),
  ]);

  if (!candlesRes.ok) {
    throw new Error(`Delta history/candles returned ${candlesRes.status}`);
  }

  const candlesJson = (await candlesRes.json()) as {
    success?: boolean;
    result?: RawCandleRow[];
    error?: { code?: string; context?: string };
  };

  if (!candlesJson.success || !Array.isArray(candlesJson.result)) {
    const msg =
      candlesJson.error?.context ?? candlesJson.error?.code ?? "Delta candles response was not successful";
    throw new Error(msg);
  }

  const candles: ReplayCandle[] = candlesJson.result
    .sort((a, b) => n(a.time) - n(b.time))
    .map(parseRawRow)
    .filter((c): c is ReplayCandle => c !== null);

  let fundingRate = 0;
  if (tickerRes.ok) {
    const tj = (await tickerRes.json()) as {
      success?: boolean;
      result?: { funding_rate?: unknown };
    };
    if (tj.success && tj.result) {
      fundingRate = n(tj.result.funding_rate);
    }
  }

  const trimmed = candles.length > count ? candles.slice(-count) : candles;

  return {
    symbol: sym,
    candles: trimmed,
    fundingRate,
    fetchedAt: new Date().toISOString(),
    source: "delta-exchange-futures-1m",
  };
}

/**
 * Fetch many 1m candles by paging backwards from now in chunks of PAGE_SIZE bars.
 *
 * Delta Exchange returns at most ~500 candles per call regardless of the requested
 * time window. This function loops backwards until `targetBars` candles are collected
 * or the exchange returns no new data.
 *
 * Progress callback receives `{ fetched, target, chunk }` after each page so callers
 * can print a progress bar.
 */
export async function fetchDeltaFutures1mCandlesPaged(
  symbol: string,
  targetBars: number,
  opts: {
    rateLimitMs?: number;
    onProgress?: (p: { fetched: number; target: number; chunk: number }) => void;
  } = {},
): Promise<FetchDelta1mResult> {
  const sym = symbol.trim().toUpperCase() || "BTCUSD";
  const PAGE_SEC = 500 * 60; // 500 bars per page → 30,000 seconds
  const rateLimitMs = opts.rateLimitMs ?? 150;
  const MAX_ITERATIONS = 400; // hard safety cap (400 × 500 = 200,000 bars max)

  const accumulated = new Map<number, ReplayCandle>(); // keyed by timeMs
  let endSec = Math.floor(Date.now() / 1000);
  const targetStartSec = endSec - targetBars * 60 - 300; // slightly wider

  let iterations = 0;

  while (accumulated.size < targetBars && endSec > targetStartSec && iterations < MAX_ITERATIONS) {
    const chunkStartSec = Math.max(targetStartSec, endSec - PAGE_SEC);

    let chunkAdded = 0;
    try {
      const res = await fetchWithRetry(
        `${DELTA_REST_BASE}/v2/history/candles?resolution=1m&symbol=${encodeURIComponent(sym)}&start=${chunkStartSec}&end=${endSec}`,
        { headers: JSON_HEADERS, cache: "no-store" },
        { attempts: 2 }, // fewer retries during paged loop to keep total time reasonable
      );

      if (res.ok) {
        const j = (await res.json()) as {
          success?: boolean;
          result?: RawCandleRow[];
        };

        if (j.success && Array.isArray(j.result)) {
          for (const row of j.result) {
            const c = parseRawRow(row);
            if (c && !accumulated.has(c.time)) {
              accumulated.set(c.time, c);
              chunkAdded++;
            }
          }
        }
      }
    } catch {
      // Network error on this chunk — stop paging, use what we have
      break;
    }

    opts.onProgress?.({
      fetched: accumulated.size,
      target: targetBars,
      chunk: chunkAdded,
    });

    if (chunkAdded === 0) {
      // No new candles from this range → exchange has no more history here
      break;
    }

    endSec = chunkStartSec - 1; // step back past this window
    iterations++;

    if (accumulated.size < targetBars && endSec > targetStartSec) {
      await new Promise((r) => setTimeout(r, rateLimitMs));
    }
  }

  // Sort ascending, trim to targetBars (keep most recent)
  const sorted = deduplicateAndSortCandles([...accumulated.values()]);
  const candles = sorted.length > targetBars ? sorted.slice(-targetBars) : sorted;

  // Fetch current funding rate (best-effort)
  let fundingRate = 0;
  try {
    const tickerRes = await fetchWithRetry(
      `${DELTA_REST_BASE}/v2/tickers/${encodeURIComponent(sym)}`,
      { headers: JSON_HEADERS, cache: "no-store" },
    );
    if (tickerRes.ok) {
      const tj = (await tickerRes.json()) as {
        success?: boolean;
        result?: { funding_rate?: unknown };
      };
      if (tj.success && tj.result) {
        fundingRate = n(tj.result.funding_rate);
      }
    }
  } catch {
    // non-fatal
  }

  return {
    symbol: sym,
    candles,
    fundingRate,
    fetchedAt: new Date().toISOString(),
    source: `delta-exchange-futures-1m-paged-${iterations}chunks`,
  };
}
