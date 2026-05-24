import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";

/** Delta Exchange India REST API */
const DELTA_REST_BASE =
  process.env.DELTA_API_BASE_URL?.replace(/\/+$/, "") ?? "https://api.india.delta.exchange";

const DEFAULT_SYMBOL = process.env.DELTA_BTC_FUTURES_SYMBOL?.trim() || "BTCUSD";

function sanitizeSymbol(raw: string | null): string {
  const s = raw?.trim().toUpperCase() ?? "";
  if (!s || !/^[A-Z0-9]{2,20}$/.test(s)) return DEFAULT_SYMBOL;
  return s;
}

/**
 * Supported bar intervals + corresponding Delta Exchange resolution strings
 * and minute-counts (used to size the start/end fetch window for ~400 bars).
 *
 * PR 10 (multi-bar-interval engine routing) — research strategies declare a
 * `primaryBarInterval` from this set; the engine fetches each TF separately
 * and routes signal eval per-strategy to the correct cache.
 */
const INTERVAL_TABLE = {
  "1m":  { resolution: "1m",  minutes: 1 },
  "5m":  { resolution: "5m",  minutes: 5 },
  "15m": { resolution: "15m", minutes: 15 },
  "4h":  { resolution: "4h",  minutes: 240 },
  "1d":  { resolution: "1d",  minutes: 1440 },
} as const;

type BarInterval = keyof typeof INTERVAL_TABLE;

function sanitizeInterval(raw: string | null): BarInterval {
  const v = raw?.trim().toLowerCase();
  if (v && v in INTERVAL_TABLE) return v as BarInterval;
  return "1m";
}

const JSON_HEADERS = {
  Accept: "application/json",
  "User-Agent": "RAIG-Trading/1.0",
};

/** Per-attempt timeout. Generous enough for Vercel cold start + Delta Exchange India latency. */
const FETCH_TIMEOUT_MS = 9_000;

async function fetchWithRetry(url: string, retries = 2, delayMs = 400): Promise<Response> {
  let lastErr: unknown;
  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      const res = await fetch(url, {
        headers: JSON_HEADERS,
        cache: "no-store",
        signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
      });
      if (res.ok || attempt === retries) return res;
    } catch (err) {
      lastErr = err;
      if (attempt === retries) break;
    }
    await new Promise((r) => setTimeout(r, delayMs * (attempt + 1)));
  }
  throw lastErr ?? new Error("fetchWithRetry: all attempts exhausted");
}

function n(value: unknown): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

type DeltaCandle = {
  time?: unknown;
  open?: unknown;
  high?: unknown;
  low?: unknown;
  close?: unknown;
  volume?: unknown;
};

type DeltaCandlesResponse = {
  success?: boolean;
  result?: DeltaCandle[];
  error?: { code?: string; context?: string };
};

type DeltaTickerResponse = {
  success?: boolean;
  result?: {
    open?: unknown;
    close?: unknown;
    mark_price?: unknown;
    index_price?: unknown;
    spot_price?: unknown;
    ltp_change_24h?: unknown;
    funding_rate?: unknown;
    next_funding_time?: unknown;
  };
};

/**
 * OHLCV candles for perpetual futures with mark price and funding rate.
 *
 * Query params:
 *   symbol    — Delta Exchange symbol (default BTCUSD)
 *   interval  — bar interval: 1m (default) | 5m | 15m | 4h | 1d. Used by PR 10
 *               multi-TF engine routing. Backwards-compatible: omitting it
 *               returns the same 1m payload as before.
 */
export async function GET(request: NextRequest): Promise<Response> {
  const symbol = sanitizeSymbol(request.nextUrl.searchParams.get("symbol"));
  const interval = sanitizeInterval(request.nextUrl.searchParams.get("interval"));
  const { resolution, minutes } = INTERVAL_TABLE[interval];

  if (
    process.env.NODE_ENV === "development" &&
    request.nextUrl.searchParams.get("debugFutures502") === "1"
  ) {
    return NextResponse.json(
      {
        ok: false,
        error: "debugFutures502 (dev only)",
        candles: [],
        lastPrice: 0,
        markPrice: 0,
        indexPrice: 0,
        changePct24h: 0,
        fundingRate: 0,
        nextFunding: 0,
        symbol,
        fetchedAt: new Date().toISOString(),
        deltaBase: DELTA_REST_BASE,
      },
      { status: 502 },
    );
  }

  const endSec = Math.floor(Date.now() / 1000);
  /** Fetch ~400 bars worth of history at the requested resolution. */
  const startSec = endSec - 400 * minutes * 60;

  try {
    // Fetch candles, ticker (for mark price), and funding rate
    const [candlesRes, tickerRes] = await Promise.all([
      fetchWithRetry(
        `${DELTA_REST_BASE}/v2/history/candles?resolution=${resolution}&symbol=${encodeURIComponent(symbol)}&start=${startSec}&end=${endSec}`,
      ),
      fetchWithRetry(`${DELTA_REST_BASE}/v2/tickers/${encodeURIComponent(symbol)}`),
    ]);

    if (!candlesRes.ok) {
      throw new Error(`Delta history/candles returned ${candlesRes.status}`);
    }

    const candlesJson = (await candlesRes.json()) as DeltaCandlesResponse;
    if (!candlesJson.success || !Array.isArray(candlesJson.result)) {
      const msg =
        candlesJson.error?.context ??
        candlesJson.error?.code ??
        "Delta candles response was not successful";
      throw new Error(msg);
    }

    const rows = [...candlesJson.result].sort((a, b) => n(a.time) - n(b.time));
    const candles: {
      time: number;
      open: number;
      high: number;
      low: number;
      close: number;
      volume: number;
    }[] = [];

    for (const row of rows) {
      const close = n(row.close);
      const vol = n(row.volume);
      if (close > 0) {
        candles.push({
          time: n(row.time),
          open: n(row.open) || close,
          high: n(row.high) || close,
          low: n(row.low) || close,
          close,
          volume: vol,
        });
      }
    }

    // Parse ticker data
    let lastPrice = 0;
    let markPrice = 0;
    let indexPrice = 0;
    let changePct24h = 0;
    let fundingRate = 0;
    let nextFunding = 0;

    if (tickerRes.ok) {
      const tj = (await tickerRes.json()) as DeltaTickerResponse;
      if (tj.success && tj.result) {
        const r = tj.result;
        lastPrice = n(r.close);
        markPrice = n(r.mark_price) || lastPrice;
        indexPrice = n(r.index_price) || markPrice;
        changePct24h = n(r.ltp_change_24h);
        fundingRate = n(r.funding_rate);
        nextFunding = n(r.next_funding_time);
      }
    }

    const candleLastPrice = candles.length > 0 ? candles[candles.length - 1].close : 0;
    const effectiveLastPrice = lastPrice > 0 ? lastPrice : candleLastPrice;
    const effectiveMarkPrice = markPrice > 0 ? markPrice : effectiveLastPrice;

    /** `fundingRate`: fraction for one funding period (not per HTTP poll). `nextFunding`: Unix seconds, next funding timestamp (Delta `next_funding_time`). */
    return NextResponse.json({
      ok: true,
      candles,
      lastPrice: effectiveLastPrice,
      markPrice: effectiveMarkPrice,
      indexPrice,
      changePct24h,
      fundingRate,
      nextFunding,
      symbol,
      interval,
      fetchedAt: new Date().toISOString(),
      source: `delta-exchange-futures-${interval}`,
      deltaBase: DELTA_REST_BASE,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "unknown error";
    return NextResponse.json(
      {
        ok: false,
        error: message,
        candles: [],
        lastPrice: 0,
        markPrice: 0,
        indexPrice: 0,
        changePct24h: 0,
        fundingRate: 0,
        nextFunding: 0,
        symbol: DEFAULT_SYMBOL,
        interval,
        fetchedAt: new Date().toISOString(),
        deltaBase: DELTA_REST_BASE,
      },
      { status: 502 }
    );
  }
}
