import { NextResponse } from "next/server";

export type GoldMarketItem = {
  symbol: string;
  displayName: string;
  yahooSymbol: string;
  proxySymbol: string;
  price: number;
  prevClose: number;
  change: number;
  changePct: number;
  dayHigh: number;
  dayLow: number;
  candles?: number[];
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
      indicators?: {
        quote?: Array<{ close?: (number | null)[] }>;
      };
    }>;
  };
};

type YahooChartResult = NonNullable<NonNullable<YahooChartResponse["chart"]>["result"]>[number];
type ChartAttempt = { interval: "1m" | "5m"; range: "1d" | "5d" };
type CachedGoldMarket = {
  data: GoldMarketItem;
  fetchedAtMs: number;
  fetchedAt: string;
};

const GOLD_YAHOO_SYMBOL = "GC=F";
const GOLD_SYMBOL = "GOLD";
const GOLD_DISPLAY_NAME = "Gold / USD";
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

let cachedMarket: CachedGoldMarket | null = null;

function n(value: unknown): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function buildChartUrls(yahooSymbol: string, interval: ChartAttempt["interval"], range: ChartAttempt["range"]): string[] {
  const encoded = encodeURIComponent(yahooSymbol);
  return [
    `https://query1.finance.yahoo.com/v8/finance/chart/${encoded}?interval=${interval}&range=${range}&includePrePost=false`,
    `https://query2.finance.yahoo.com/v8/finance/chart/${encoded}?interval=${interval}&range=${range}&includePrePost=false`,
  ];
}

function extractCandles(result: YahooChartResult): number[] {
  const closes = result?.indicators?.quote?.[0]?.close ?? [];
  return closes
    .filter((value): value is number => value !== null && value !== undefined && Number.isFinite(value))
    .slice(-240);
}

async function fetchGoldChart(): Promise<GoldMarketItem> {
  let lastError = `No chart result for ${GOLD_YAHOO_SYMBOL}`;

  for (const attempt of CHART_ATTEMPTS) {
    for (const url of buildChartUrls(GOLD_YAHOO_SYMBOL, attempt.interval, attempt.range)) {
      try {
        const response = await fetch(url, { headers: CHART_HEADERS, cache: "no-store" });
        if (!response.ok) {
          lastError = `Yahoo chart ${GOLD_YAHOO_SYMBOL} returned ${response.status}`;
          continue;
        }

        const body = (await response.json()) as YahooChartResponse;
        const result = body.chart?.result?.[0];
        if (!result) {
          lastError = `No chart result for ${GOLD_YAHOO_SYMBOL}`;
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
          lastError = `No usable gold price for ${GOLD_YAHOO_SYMBOL}`;
          continue;
        }

        return {
          symbol: GOLD_SYMBOL,
          displayName: GOLD_DISPLAY_NAME,
          yahooSymbol: GOLD_YAHOO_SYMBOL,
          proxySymbol: GOLD_YAHOO_SYMBOL,
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
        lastError = error instanceof Error ? error.message : `Unknown Yahoo fetch error for ${GOLD_YAHOO_SYMBOL}`;
      }
    }
  }

  throw new Error(lastError);
}

export async function GET(): Promise<Response> {
  const now = Date.now();
  if (cachedMarket && now - cachedMarket.fetchedAtMs < CACHE_TTL_MS) {
    return NextResponse.json({
      ok: true,
      data: cachedMarket.data,
      fetchedAt: cachedMarket.fetchedAt,
      source: "yahoo-finance-chart",
      cached: true,
      stale: false,
    });
  }

  try {
    const data = await fetchGoldChart();
    const fetchedAt = new Date().toISOString();
    cachedMarket = { data, fetchedAtMs: now, fetchedAt };
    return NextResponse.json({
      ok: true,
      data,
      fetchedAt,
      source: "yahoo-finance-chart",
      cached: false,
      stale: false,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "unknown error";

    if (cachedMarket && now - cachedMarket.fetchedAtMs < STALE_CACHE_TTL_MS) {
      return NextResponse.json({
        ok: true,
        data: cachedMarket.data,
        fetchedAt: cachedMarket.fetchedAt,
        source: "yahoo-finance-chart",
        cached: true,
        stale: true,
        error: `Using cached gold quotes after upstream failure: ${message}`,
      });
    }

    return NextResponse.json({
      ok: false,
      error: message,
      data: {
        symbol: GOLD_SYMBOL,
        displayName: GOLD_DISPLAY_NAME,
        yahooSymbol: GOLD_YAHOO_SYMBOL,
        proxySymbol: GOLD_YAHOO_SYMBOL,
        price: 0,
        prevClose: 0,
        change: 0,
        changePct: 0,
        dayHigh: 0,
        dayLow: 0,
        candles: [],
        source: "yahoo-finance-chart",
      },
      fetchedAt: new Date().toISOString(),
      source: "yahoo-finance-chart",
    });
  }
}
