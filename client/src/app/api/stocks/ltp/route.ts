import { NextResponse } from "next/server";
import { NIFTY50_STOCKS } from "@/lib/trading/nifty50Stocks";

export type StockLTPItem = {
  symbol: string;
  token: string;
  ltp: number;
  open: number;
  high: number;
  low: number;
  close: number; // prev day close
  changePct: number;
};

type StockLTPResponse = {
  ok: boolean;
  error: string;
  stocks: StockLTPItem[];
  fetchedAt: string;
  source?: string;
};

type ChartMeta = {
  regularMarketPrice?: number;
  regularMarketOpen?: number;
  regularMarketDayHigh?: number;
  regularMarketDayLow?: number;
  chartPreviousClose?: number;
  previousClose?: number;
};

type ChartResponse = {
  chart?: { result?: Array<{ meta?: ChartMeta }>; error?: unknown };
};

function n(v: unknown): number {
  const num = Number(v);
  return Number.isFinite(num) ? num : 0;
}

const HEADERS = {
  "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
  "Accept": "application/json",
};

// Fetch a single stock's price using v8/chart (v7/quote returns 401)
async function fetchStockPrice(yahooSymbol: string): Promise<ChartMeta | null> {
  const mirrors = [
    `https://query1.finance.yahoo.com/v8/finance/chart/${encodeURIComponent(yahooSymbol)}?interval=1d&range=2d&includePrePost=false`,
    `https://query2.finance.yahoo.com/v8/finance/chart/${encodeURIComponent(yahooSymbol)}?interval=1d&range=2d&includePrePost=false`,
  ];
  for (const url of mirrors) {
    try {
      const res = await fetch(url, { headers: HEADERS, cache: "no-store" });
      if (!res.ok) continue;
      const data = await res.json() as ChartResponse;
      const meta = data?.chart?.result?.[0]?.meta;
      if (meta?.regularMarketPrice) return meta;
    } catch {
      // try next mirror
    }
  }
  return null;
}

export async function GET(): Promise<Response> {
  try {
    // Fetch all 50 stocks in parallel (v8/chart, no auth required)
    const results = await Promise.allSettled(
      NIFTY50_STOCKS.map((stock) => fetchStockPrice(`${stock.symbol}.NS`)),
    );

    const stocks: StockLTPItem[] = NIFTY50_STOCKS.map((stock, i) => {
      const result = results[i];
      const meta = result.status === "fulfilled" ? result.value : null;
      const prevClose = n(meta?.chartPreviousClose ?? meta?.previousClose);
      const ltp = n(meta?.regularMarketPrice);
      const changePct = prevClose > 0 ? ((ltp - prevClose) / prevClose) * 100 : 0;
      return {
        symbol: stock.symbol,
        token: stock.token,
        ltp,
        open: n(meta?.regularMarketOpen),
        high: n(meta?.regularMarketDayHigh),
        low: n(meta?.regularMarketDayLow),
        close: prevClose,
        changePct,
      };
    });

    return NextResponse.json({
      ok: true,
      error: "",
      stocks,
      fetchedAt: new Date().toISOString(),
      source: "yahoo-v8",
    } satisfies StockLTPResponse);
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({
      ok: false,
      error: message,
      stocks: NIFTY50_STOCKS.map((stock) => ({
        symbol: stock.symbol,
        token: stock.token,
        ltp: 0, open: 0, high: 0, low: 0, close: 0, changePct: 0,
      })),
      fetchedAt: new Date().toISOString(),
      source: "yahoo-v8",
    } satisfies StockLTPResponse);
  }
}
