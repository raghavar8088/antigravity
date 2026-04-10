/**
 * /api/nifty/candles — Historical 1-minute candles for NIFTY 50
 *
 * Data source: Yahoo Finance v8 chart API (^NSEI)
 * - No authentication required
 * - No API key required
 * - Returns today's 1-minute bars from market open (09:15 IST) to now
 * - Falls back to 5-minute bars if 1-minute is unavailable
 */

import { NextResponse } from "next/server";

export type Candle = {
  time: number;   // Unix ms timestamp
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

type YahooChartMeta = {
  regularMarketPrice?: number;
  previousClose?: number;
};

type YahooChartResponse = {
  chart?: {
    result?: Array<{
      meta?: YahooChartMeta;
      timestamp?: number[];
      indicators?: {
        quote?: Array<{
          open?: (number | null)[];
          high?: (number | null)[];
          low?: (number | null)[];
          close?: (number | null)[];
          volume?: (number | null)[];
        }>;
      };
    }>;
    error?: { code?: string; description?: string };
  };
};

function safeNum(v: number | null | undefined): number {
  return typeof v === "number" && Number.isFinite(v) ? v : 0;
}

async function fetchYahooCandles(interval: string): Promise<Candle[] | null> {
  // Yahoo Finance chart — 1d range gives today's intraday bars
  const url = `https://query1.finance.yahoo.com/v8/finance/chart/%5ENSEI?interval=${interval}&range=1d&includePrePost=false`;
  const url2 = `https://query2.finance.yahoo.com/v8/finance/chart/%5ENSEI?interval=${interval}&range=1d&includePrePost=false`;

  const headers = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
    "Accept": "application/json",
  };

  for (const endpoint of [url, url2]) {
    try {
      const res = await fetch(endpoint, { headers, cache: "no-store" });
      if (!res.ok) continue;

      const data = await res.json() as YahooChartResponse;
      const result = data?.chart?.result?.[0];
      if (!result) continue;

      const timestamps = result.timestamp ?? [];
      const quote = result.indicators?.quote?.[0];
      if (!timestamps.length || !quote) continue;

      const candles: Candle[] = [];
      for (let i = 0; i < timestamps.length; i++) {
        const close = safeNum(quote.close?.[i]);
        if (close <= 0) continue; // skip gaps/nulls
        candles.push({
          time: timestamps[i] * 1000, // seconds → ms
          open:   safeNum(quote.open?.[i]),
          high:   safeNum(quote.high?.[i]),
          low:    safeNum(quote.low?.[i]),
          close,
          volume: safeNum(quote.volume?.[i]),
        });
      }

      if (candles.length > 0) return candles;
    } catch {
      // try next mirror
    }
  }
  return null;
}

export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const requestedInterval = searchParams.get("interval") ?? "ONE_MINUTE";

  // Map Angel One interval names → Yahoo Finance interval codes
  const intervalMap: Record<string, string> = {
    ONE_MINUTE:   "1m",
    THREE_MINUTE: "2m",  // Yahoo has 2m, not 3m
    FIVE_MINUTE:  "5m",
    TEN_MINUTE:   "5m",
    FIFTEEN_MINUTE: "15m",
    THIRTY_MINUTE: "30m",
    ONE_HOUR:     "60m",
  };
  const yahooInterval = intervalMap[requestedInterval] ?? "1m";

  // Try requested interval, fallback to 5m if 1m fails (Yahoo sometimes delays 1m)
  let candles = await fetchYahooCandles(yahooInterval);
  if (!candles && yahooInterval === "1m") {
    candles = await fetchYahooCandles("5m");
  }

  if (!candles) {
    return NextResponse.json({
      ok: false,
      candles: [],
      count: 0,
      error: "Yahoo Finance unavailable — no candle data returned",
    });
  }

  return NextResponse.json({
    ok: true,
    candles,
    count: candles.length,
    source: "yahoo",
    interval: yahooInterval,
  });
}
