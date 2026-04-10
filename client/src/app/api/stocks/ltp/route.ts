import { NextResponse } from "next/server";
import { NIFTY50_STOCKS } from "@/lib/nifty50Stocks";

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

type YahooQuoteResult = {
  symbol?: unknown;
  regularMarketPrice?: unknown;
  regularMarketOpen?: unknown;
  regularMarketDayHigh?: unknown;
  regularMarketDayLow?: unknown;
  regularMarketPreviousClose?: unknown;
  regularMarketChangePercent?: unknown;
};

type YahooQuoteResponse = {
  quoteResponse?: {
    result?: YahooQuoteResult[];
    error?: unknown;
  };
};

function n(v: unknown): number {
  const num = Number(v);
  return Number.isFinite(num) ? num : 0;
}

function toYahooSymbol(symbol: string): string {
  return `${symbol}.NS`;
}

function buildYahooQuoteUrls(symbols: string[]): string[] {
  const joined = symbols.map((symbol) => encodeURIComponent(symbol)).join(",");
  const fields = "regularMarketPrice,regularMarketOpen,regularMarketDayHigh,regularMarketDayLow,regularMarketPreviousClose,regularMarketChangePercent";
  return [
    `https://query1.finance.yahoo.com/v7/finance/quote?symbols=${joined}&fields=${fields}`,
    `https://query2.finance.yahoo.com/v7/finance/quote?symbols=${joined}&fields=${fields}`,
  ];
}

async function fetchYahooQuotes(symbols: string[]): Promise<YahooQuoteResult[]> {
  const headers = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
    "Accept": "application/json",
  };

  for (const url of buildYahooQuoteUrls(symbols)) {
    try {
      const res = await fetch(url, { headers, cache: "no-store" });
      if (!res.ok) continue;

      const payload = await res.json() as YahooQuoteResponse;
      const results = payload?.quoteResponse?.result;
      if (Array.isArray(results) && results.length > 0) {
        return results;
      }
    } catch {
      // try next mirror
    }
  }

  throw new Error("Yahoo Finance quote unavailable");
}

export async function GET(): Promise<Response> {
  try {
    const yahooSymbols = NIFTY50_STOCKS.map((stock) => toYahooSymbol(stock.symbol));
    const quotes = await fetchYahooQuotes(yahooSymbols);

    const resultMap = new Map<string, YahooQuoteResult>();
    for (const quote of quotes) {
      const symbol = String(quote.symbol ?? "").toUpperCase();
      if (symbol) {
        resultMap.set(symbol, quote);
      }
    }

    const stocks: StockLTPItem[] = NIFTY50_STOCKS.map((stock) => {
      const yahooSymbol = toYahooSymbol(stock.symbol).toUpperCase();
      const quote = resultMap.get(yahooSymbol);
      return {
        symbol: stock.symbol,
        token: stock.token,
        ltp: n(quote?.regularMarketPrice),
        open: n(quote?.regularMarketOpen),
        high: n(quote?.regularMarketDayHigh),
        low: n(quote?.regularMarketDayLow),
        close: n(quote?.regularMarketPreviousClose),
        changePct: n(quote?.regularMarketChangePercent),
      };
    });

    return NextResponse.json({
      ok: true,
      error: "",
      stocks,
      fetchedAt: new Date().toISOString(),
      source: "yahoo",
    } satisfies StockLTPResponse);
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({
      ok: false,
      error: message,
      stocks: NIFTY50_STOCKS.map((stock) => ({
        symbol: stock.symbol,
        token: stock.token,
        ltp: 0,
        open: 0,
        high: 0,
        low: 0,
        close: 0,
        changePct: 0,
      })),
      fetchedAt: new Date().toISOString(),
      source: "yahoo",
    } satisfies StockLTPResponse);
  }
}
