import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";

/**
 * BTC Pre-Live Engine — paged candle history + live ticker (Phase 1).
 *
 * Same public Delta Exchange endpoints as /api/btc/futures-klines, but with
 * explicit start/end paging so the chart can lazy-load a full year of history
 * as the user pans left, instead of a fixed recent window. Additive route:
 * nothing else in the app depends on it.
 *
 * Query params:
 *   resolution — 5m | 15m | 1h | 4h | 1d   (default 1h)
 *   start      — unix seconds (inclusive). Optional.
 *   end        — unix seconds (exclusive). Optional, default now.
 *   ticker     — "1" to include live ticker fields (for the tail poll).
 *
 * With no start/end it returns the most recent PAGE_BARS candles.
 */

const DELTA_REST_BASE =
  process.env.DELTA_API_BASE_URL?.replace(/\/+$/, "") ?? "https://api.india.delta.exchange";

const DEFAULT_SYMBOL = process.env.DELTA_BTC_FUTURES_SYMBOL?.trim() || "BTCUSD";

/** Max candles served per call — one lazy-load page. */
const PAGE_BARS = 1500;

/** History is capped at ~1 year back (the Phase 1 scope). */
const MAX_LOOKBACK_SEC = 366 * 24 * 60 * 60;

const RESOLUTION_TABLE = {
  "5m": { minutes: 5 },
  "15m": { minutes: 15 },
  "1h": { minutes: 60 },
  "4h": { minutes: 240 },
  "1d": { minutes: 1440 },
} as const;

type Resolution = keyof typeof RESOLUTION_TABLE;

function sanitizeResolution(raw: string | null): Resolution {
  const v = raw?.trim().toLowerCase();
  if (v && v in RESOLUTION_TABLE) return v as Resolution;
  return "1h";
}

function sanitizeSymbol(raw: string | null): string {
  const s = raw?.trim().toUpperCase() ?? "";
  if (!s || !/^[A-Z0-9]{2,20}$/.test(s)) return DEFAULT_SYMBOL;
  return s;
}

function n(value: unknown): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

const JSON_HEADERS = { Accept: "application/json", "User-Agent": "RAIG-Trading/1.0" };
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

type DeltaCandlesResponse = {
  success?: boolean;
  result?: { time?: unknown; open?: unknown; high?: unknown; low?: unknown; close?: unknown; volume?: unknown }[];
  error?: { code?: string; context?: string };
};

type DeltaTickerResponse = {
  success?: boolean;
  result?: {
    close?: unknown;
    mark_price?: unknown;
    ltp_change_24h?: unknown;
    funding_rate?: unknown;
  };
};

export async function GET(request: NextRequest): Promise<Response> {
  const params = request.nextUrl.searchParams;
  const symbol = sanitizeSymbol(params.get("symbol"));
  const resolution = sanitizeResolution(params.get("resolution"));
  const wantTicker = params.get("ticker") === "1";
  const { minutes } = RESOLUTION_TABLE[resolution];

  const nowSec = Math.floor(Date.now() / 1000);
  const oldestAllowed = nowSec - MAX_LOOKBACK_SEC;

  let endSec = n(params.get("end")) || nowSec;
  let startSec = n(params.get("start"));
  if (!startSec) startSec = endSec - PAGE_BARS * minutes * 60;

  // Clamp to the 1-year window and to a single page of bars.
  startSec = Math.max(startSec, oldestAllowed);
  endSec = Math.min(endSec, nowSec);
  if (endSec - startSec > PAGE_BARS * minutes * 60) {
    startSec = endSec - PAGE_BARS * minutes * 60;
  }

  try {
    const candlesURL =
      `${DELTA_REST_BASE}/v2/history/candles?resolution=${resolution}` +
      `&symbol=${encodeURIComponent(symbol)}&start=${startSec}&end=${endSec}`;

    const [candlesRes, tickerRes] = await Promise.all([
      fetchWithRetry(candlesURL),
      wantTicker
        ? fetchWithRetry(`${DELTA_REST_BASE}/v2/tickers/${encodeURIComponent(symbol)}`)
        : Promise.resolve(null),
    ]);

    if (!candlesRes.ok) throw new Error(`Delta history/candles returned ${candlesRes.status}`);
    const candlesJson = (await candlesRes.json()) as DeltaCandlesResponse;
    if (!candlesJson.success || !Array.isArray(candlesJson.result)) {
      throw new Error(
        candlesJson.error?.context ?? candlesJson.error?.code ?? "Delta candles response not successful",
      );
    }

    const rows = [...candlesJson.result].sort((a, b) => n(a.time) - n(b.time));
    const candles = rows
      .filter((r) => n(r.close) > 0)
      .map((r) => ({
        time: n(r.time),
        open: n(r.open) || n(r.close),
        high: n(r.high) || n(r.close),
        low: n(r.low) || n(r.close),
        close: n(r.close),
        volume: n(r.volume),
      }));

    let lastPrice = 0;
    let markPrice = 0;
    let changePct24h = 0;
    let fundingRate = 0;
    if (tickerRes?.ok) {
      const tj = (await tickerRes.json()) as DeltaTickerResponse;
      if (tj.success && tj.result) {
        lastPrice = n(tj.result.close);
        markPrice = n(tj.result.mark_price) || lastPrice;
        changePct24h = n(tj.result.ltp_change_24h);
        fundingRate = n(tj.result.funding_rate);
      }
    }

    return NextResponse.json({
      ok: true,
      symbol,
      resolution,
      candles,
      // True when the caller has reached the 1-year cap — the chart stops
      // requesting older pages once this comes back.
      reachedHistoryCap: startSec <= oldestAllowed,
      windowStart: startSec,
      windowEnd: endSec,
      ...(wantTicker ? { lastPrice, markPrice, changePct24h, fundingRate } : {}),
      fetchedAt: new Date().toISOString(),
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "unknown error";
    return NextResponse.json(
      { ok: false, error: message, symbol, resolution, candles: [] },
      { status: 502 },
    );
  }
}
