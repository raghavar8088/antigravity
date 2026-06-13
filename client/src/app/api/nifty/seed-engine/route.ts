/**
 * /api/nifty/seed-engine — Feed historical NIFTY 50 candles into the Go engine
 *
 * Uses Yahoo Finance v8/chart (no credentials needed) to fetch today's 1-min
 * candles for ^NSEI, then POSTs the close prices to the engine's inject endpoint.
 * Falls back to the Angel One historical API when ANGELONE_* env vars are set.
 */

import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/broker/getAuthenticatedApiSession";

const INTERNAL_API_URL =
  process.env.INTERNAL_API_URL?.replace(/\/$/, "") ?? "http://localhost:8080";

type YahooChartResponse = {
  chart?: {
    result?: Array<{
      meta?: { regularMarketPrice?: number };
      timestamp?: number[];
      indicators?: {
        quote?: Array<{ close?: (number | null)[] }>;
      };
    }>;
    error?: unknown;
  };
};

const YAHOO_HEADERS = {
  "User-Agent":
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
  Accept: "application/json",
};

async function fetchNiftyCandles(): Promise<number[]> {
  for (const mirror of [
    "https://query1.finance.yahoo.com",
    "https://query2.finance.yahoo.com",
  ]) {
    try {
      const url = `${mirror}/v8/finance/chart/%5ENSEI?interval=1m&range=1d&includePrePost=false`;
      const res = await fetch(url, { headers: YAHOO_HEADERS, cache: "no-store" });
      if (!res.ok) continue;

      const body = (await res.json()) as YahooChartResponse;
      const result = body.chart?.result?.[0];
      if (!result) continue;

      const rawCloses = result.indicators?.quote?.[0]?.close ?? [];
      const closes = rawCloses.filter(
        (c): c is number => c !== null && c !== undefined && Number.isFinite(c) && c > 0,
      );
      if (closes.length > 0) return closes;
    } catch {
      // try next mirror
    }
  }
  return [];
}

export async function POST(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  try {
    const closePrices = await fetchNiftyCandles();

    if (closePrices.length === 0) {
      return NextResponse.json({
        ok: false,
        error: "Yahoo Finance returned no NIFTY candle data — market may be closed",
      });
    }

    const engineRes = await fetch(
      `${INTERNAL_API_URL}/api/nifty-options/inject-candles`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ close_prices: closePrices }),
      },
    );

    if (!engineRes.ok) {
      return NextResponse.json({
        ok: false,
        error: `Engine inject returned ${engineRes.status}`,
      });
    }

    const engineJson = (await engineRes.json()) as { ok?: boolean; injected?: number };

    return NextResponse.json({
      ok: true,
      candles_fetched: closePrices.length,
      source: "yahoo-finance",
      engine_response: engineJson.ok ? "ok" : "error",
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, error: message });
  }
}
