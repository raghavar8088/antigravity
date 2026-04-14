/**
 * /api/nifty/vix — India VIX live quote
 *
 * Uses Yahoo Finance v8/chart (^INDIAVIX) — no credentials needed.
 * Falls back between query1 and query2 mirrors.
 */

import { NextResponse } from "next/server";

type YahooChartMeta = {
  regularMarketPrice?: number;
  chartPreviousClose?: number;
  previousClose?: number;
};

type YahooChartResponse = {
  chart?: {
    result?: Array<{ meta?: YahooChartMeta }>;
    error?: unknown;
  };
};

function n(v: unknown): number {
  const num = Number(v);
  return Number.isFinite(num) ? num : 0;
}

const HEADERS = {
  "User-Agent":
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
  Accept: "application/json",
};

async function fetchVIX(): Promise<YahooChartMeta | null> {
  for (const mirror of [
    "https://query1.finance.yahoo.com",
    "https://query2.finance.yahoo.com",
  ]) {
    try {
      const url = `${mirror}/v8/finance/chart/%5EINDIAVIX?interval=1m&range=1d&includePrePost=false`;
      const res = await fetch(url, { headers: HEADERS, cache: "no-store" });
      if (!res.ok) continue;
      const data = (await res.json()) as YahooChartResponse;
      const meta = data?.chart?.result?.[0]?.meta;
      if (meta?.regularMarketPrice) return meta;
    } catch {
      // try next mirror
    }
  }
  return null;
}

export async function GET() {
  try {
    const meta = await fetchVIX();

    if (!meta) {
      return NextResponse.json({
        ok: false,
        vix: 0,
        error: "Yahoo Finance unavailable for India VIX",
      });
    }

    const vix = n(meta.regularMarketPrice);
    const prevClose = n(meta.chartPreviousClose ?? meta.previousClose);
    const change = prevClose > 0 ? vix - prevClose : 0;
    const percentChange = prevClose > 0 ? (change / prevClose) * 100 : 0;

    return NextResponse.json({
      ok: true,
      vix,
      change,
      percentChange,
      fetched_at: new Date().toISOString(),
      source: "yahoo-finance",
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, vix: 0, error: message });
  }
}
