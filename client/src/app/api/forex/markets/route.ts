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
  interval?: "1m" | "5m";
  source?: string;
};

type YahooChartMeta = {
  regularMarketPrice?: number;
  chartPreviousClose?: number;
  previousClose?: number;
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

type YahooChartResult = NonNullable<NonNullable<YahooChartResponse["chart"]>["result"]>[number];
type ChartAttempt = { interval: "1m" | "5m"; range: "1d" | "5d" };
type CachedForexMarkets = {
  data: ForexMarketItem[];
  fetchedAtMs: number;
  fetchedAt: string;
  liveCount: number;
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

const CHART_ATTEMPTS: ChartAttempt[] = [
  { interval: "1m", range: "1d" },
  { interval: "5m", range: "5d" },
];
const CACHE_TTL_MS = 20_000;
const STALE_CACHE_TTL_MS = 5 * 60 * 1000;

let cachedMarkets: CachedForexMarkets | null = null;

function buildChartUrls(yahooSymbol: string, interval: ChartAttempt["interval"], range: ChartAttempt["range"]): string[] {
  const encoded = encodeURIComponent(yahooSymbol);
  return [
    `https://query1.finance.yahoo.com/v8/finance/chart/${encoded}?interval=${interval}&range=${range}&includePrePost=false`,
    `https://query2.finance.yahoo.com/v8/finance/chart/${encoded}?interval=${interval}&range=${range}&includePrePost=false`,
  ];
}

function extractCandles(result: YahooChartResult): number[] {
  const rawCloses = result?.indicators?.quote?.[0]?.close ?? [];
  return rawCloses
    .filter((c): c is number => c !== null && c !== undefined && Number.isFinite(c))
    .slice(-120);
}

async function fetchPairChart(
  yahooSymbol: string,
  displaySymbol: string,
): Promise<ForexMarketItem> {
  let lastError = `No chart result for ${yahooSymbol}`;

  for (const attempt of CHART_ATTEMPTS) {
    for (const url of buildChartUrls(yahooSymbol, attempt.interval, attempt.range)) {
      try {
        const res = await fetch(url, { headers: CHART_HEADERS, cache: "no-store" });
        if (!res.ok) {
          lastError = `Yahoo chart ${yahooSymbol} returned ${res.status}`;
          continue;
        }

        const body = (await res.json()) as YahooChartResponse;
        const result = body.chart?.result?.[0];
        if (!result) {
          lastError = `No chart result for ${yahooSymbol}`;
          continue;
        }

        const candles = extractCandles(result);
        const lastCandle = candles.length > 0 ? candles[candles.length - 1] : 0;
        const meta = result.meta ?? {};
        const price = n(meta.regularMarketPrice) || lastCandle;
        const prevClose = n(meta.chartPreviousClose ?? meta.previousClose) || (candles.length > 0 ? candles[0] : 0);
        const dayHigh = n(meta.regularMarketDayHigh) || (candles.length > 0 ? Math.max(...candles) : 0);
        const dayLow = n(meta.regularMarketDayLow) || (candles.length > 0 ? Math.min(...candles) : 0);
        const change = price - prevClose;
        const changePct = prevClose > 0 ? (change / prevClose) * 100 : 0;

        if (price <= 0 && candles.length === 0) {
          lastError = `No usable forex price for ${yahooSymbol}`;
          continue;
        }

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
          interval: attempt.interval,
          source: "yahoo-finance-chart",
        };
      } catch (error) {
        lastError = error instanceof Error ? error.message : `Unknown Yahoo fetch error for ${yahooSymbol}`;
      }
    }
  }

  throw new Error(lastError);
}

export async function GET(): Promise<Response> {
  const now = Date.now();
  if (cachedMarkets && now - cachedMarkets.fetchedAtMs < CACHE_TTL_MS) {
    return NextResponse.json({
      ok: true,
      data: cachedMarkets.data,
      liveCount: cachedMarkets.liveCount,
      fetchedAt: cachedMarkets.fetchedAt,
      source: "yahoo-finance-chart",
      cached: true,
      stale: false,
    });
  }

  try {
    const cachedBySymbol = new Map((cachedMarkets?.data ?? []).map((item) => [item.symbol, item]));
    const settled = await Promise.allSettled(
      FOREX_PAIRS.map((pair) => fetchPairChart(pair.yahooSymbol, pair.symbol)),
    );

    let usedCachedPairs = 0;
    const data: ForexMarketItem[] = FOREX_PAIRS.map((pair, i) => {
      const r = settled[i];
      if (r.status === "fulfilled") return r.value;
      const cached = cachedBySymbol.get(pair.symbol);
      if (cached) {
        usedCachedPairs += 1;
        return cached;
      }
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
    const fetchedAt = new Date().toISOString();

    if (liveCount > 0) {
      cachedMarkets = { data, liveCount, fetchedAtMs: now, fetchedAt };
      return NextResponse.json({
        ok: true,
        data,
        liveCount,
        fetchedAt,
        source: "yahoo-finance-chart",
        cached: false,
        stale: false,
        ...(usedCachedPairs > 0 && { error: `Reused ${usedCachedPairs} cached forex pair(s) after partial Yahoo refresh failures.` }),
      });
    }

    if (cachedMarkets && now - cachedMarkets.fetchedAtMs < STALE_CACHE_TTL_MS) {
      return NextResponse.json({
        ok: true,
        data: cachedMarkets.data,
        liveCount: cachedMarkets.liveCount,
        fetchedAt: cachedMarkets.fetchedAt,
        source: "yahoo-finance-chart",
        cached: true,
        stale: true,
        error: "Using cached forex quotes because Yahoo did not return fresh prices.",
      });
    }

    return NextResponse.json({
      ok: false,
      data,
      liveCount,
      fetchedAt,
      source: "yahoo-finance-chart",
      error: "No live forex quotes retrieved",
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "unknown error";

    if (cachedMarkets && now - cachedMarkets.fetchedAtMs < STALE_CACHE_TTL_MS) {
      return NextResponse.json({
        ok: true,
        data: cachedMarkets.data,
        liveCount: cachedMarkets.liveCount,
        fetchedAt: cachedMarkets.fetchedAt,
        source: "yahoo-finance-chart",
        cached: true,
        stale: true,
        error: `Using cached forex quotes after upstream failure: ${message}`,
      });
    }

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
      liveCount: 0,
      fetchedAt: new Date().toISOString(),
      source: "yahoo-finance-chart",
    });
  }
}
