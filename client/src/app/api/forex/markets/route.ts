import { NextResponse } from "next/server";
import { FOREX_PAIRS } from "@/lib/forexPairs";

export type ForexMarketItem = {
  symbol: string;
  yahooSymbol: string;
  price: number;
  prevClose: number;
  change: number;
  changePct: number;
  dayHigh: number;
  dayLow: number;
  candles?: number[]; // historical 1-min closes, newest last (up to 120)
};

type YahooChartMeta = {
  regularMarketPrice?: number;
  chartPreviousClose?: number;
  regularMarketDayHigh?: number;
  regularMarketDayLow?: number;
};

type YahooChartResponse = {
  chart?: {
    result?: Array<{
      meta?: YahooChartMeta;
      timestamp?: number[];
      indicators?: {
        quote?: Array<{ close?: (number | null)[] }>;
      };
    }>;
    error?: unknown;
  };
};

function n(v: unknown): number {
  const p = Number(v);
  return Number.isFinite(p) ? p : 0;
}

const CHART_HEADERS = {
  "User-Agent":
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
  Accept: "application/json",
};

async function fetchPairChart(
  yahooSymbol: string,
  displaySymbol: string,
): Promise<ForexMarketItem> {
  const url = `https://query1.finance.yahoo.com/v8/finance/chart/${encodeURIComponent(yahooSymbol)}?interval=1m&range=1d`;
  const res = await fetch(url, { headers: CHART_HEADERS, cache: "no-store" });
  if (!res.ok) throw new Error(`Yahoo chart ${yahooSymbol} returned ${res.status}`);

  const body = (await res.json()) as YahooChartResponse;
  const result = body.chart?.result?.[0];
  if (!result) throw new Error(`No chart result for ${yahooSymbol}`);

  const meta = result.meta ?? {};
  const price = n(meta.regularMarketPrice);
  const prevClose = n(meta.chartPreviousClose);
  const dayHigh = n(meta.regularMarketDayHigh);
  const dayLow = n(meta.regularMarketDayLow);
  const change = price - prevClose;
  const changePct = prevClose > 0 ? (change / prevClose) * 100 : 0;

  // Extract up to 120 historical 1-min close prices for engine pre-seeding
  const rawCloses = result.indicators?.quote?.[0]?.close ?? [];
  const candles = rawCloses
    .filter((c): c is number => c !== null && c !== undefined && Number.isFinite(c))
    .slice(-120);

  return {
    symbol: displaySymbol,
    yahooSymbol,
    price,
    prevClose,
    change,
    changePct,
    dayHigh,
    dayLow,
    candles,
  };
}

export async function GET(): Promise<Response> {
  try {
    const settled = await Promise.allSettled(
      FOREX_PAIRS.map((pair) => fetchPairChart(pair.yahooSymbol, pair.symbol)),
    );

    const data: ForexMarketItem[] = FOREX_PAIRS.map((pair, i) => {
      const r = settled[i];
      if (r.status === "fulfilled") return r.value;
      return {
        symbol: pair.symbol,
        yahooSymbol: pair.yahooSymbol,
        price: 0,
        prevClose: 0,
        change: 0,
        changePct: 0,
        dayHigh: 0,
        dayLow: 0,
      };
    });

    const liveCount = data.filter((d) => d.price > 0).length;

    return NextResponse.json({
      ok: liveCount > 0,
      data,
      fetchedAt: new Date().toISOString(),
      source: "yahoo-finance-chart",
      ...(liveCount === 0 && { error: "No live forex quotes retrieved" }),
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "unknown error";
    return NextResponse.json({
      ok: false,
      error: message,
      data: FOREX_PAIRS.map((pair) => ({
        symbol: pair.symbol,
        yahooSymbol: pair.yahooSymbol,
        price: 0,
        prevClose: 0,
        change: 0,
        changePct: 0,
        dayHigh: 0,
        dayLow: 0,
      })),
      fetchedAt: new Date().toISOString(),
      source: "yahoo-finance-chart",
    });
  }
}
