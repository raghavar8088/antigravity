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

function n(value: unknown): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

/** Delta may return Unix seconds; replay uses epoch ms. */
export function normalizeCandleTimeMs(t: number): number {
  if (!Number.isFinite(t) || t <= 0) return 0;
  return t < 1e12 ? Math.round(t * 1000) : Math.round(t);
}

export type FetchDelta1mResult = {
  symbol: string;
  candles: ReplayCandle[];
  fundingRate: number;
  fetchedAt: string;
  source: string;
};

/**
 * Fetch up to `bars` recent 1m candles for a perpetual symbol.
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
    fetch(
      `${DELTA_REST_BASE}/v2/history/candles?resolution=1m&symbol=${encodeURIComponent(sym)}&start=${startSec}&end=${endSec}`,
      { headers: JSON_HEADERS, cache: "no-store" },
    ),
    fetch(`${DELTA_REST_BASE}/v2/tickers/${encodeURIComponent(sym)}`, {
      headers: JSON_HEADERS,
      cache: "no-store",
    }),
  ]);

  if (!candlesRes.ok) {
    throw new Error(`Delta history/candles returned ${candlesRes.status}`);
  }

  const candlesJson = (await candlesRes.json()) as {
    success?: boolean;
    result?: Array<{
      time?: unknown;
      open?: unknown;
      high?: unknown;
      low?: unknown;
      close?: unknown;
      volume?: unknown;
    }>;
    error?: { code?: string; context?: string };
  };

  if (!candlesJson.success || !Array.isArray(candlesJson.result)) {
    const msg =
      candlesJson.error?.context ?? candlesJson.error?.code ?? "Delta candles response was not successful";
    throw new Error(msg);
  }

  const rows = [...candlesJson.result].sort((a, b) => n(a.time) - n(b.time));
  const candles: ReplayCandle[] = [];
  for (const row of rows) {
    const close = n(row.close);
    if (close <= 0) continue;
    candles.push({
      time: normalizeCandleTimeMs(n(row.time)),
      open: n(row.open) || close,
      high: n(row.high) || close,
      low: n(row.low) || close,
      close,
      volume: n(row.volume),
    });
  }

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
