import { NextResponse } from "next/server";
import { FOREX_PAIRS } from "@/lib/forexPairs";

export type ForexMarketItem = {
  symbol: string;       // "EUR/USD"
  yahooSymbol: string;  // "EURUSD=X"
  price: number;
  prevClose: number;
  change: number;
  changePct: number;
  dayHigh: number;
  dayLow: number;
};

type YahooQuote = {
  symbol?: string;
  regularMarketPrice?: number;
  regularMarketPreviousClose?: number;
  regularMarketChange?: number;
  regularMarketChangePercent?: number;
  regularMarketDayHigh?: number;
  regularMarketDayLow?: number;
};

function n(v: unknown): number {
  const p = Number(v);
  return Number.isFinite(p) ? p : 0;
}

async function fetchYahooQuotes(): Promise<YahooQuote[]> {
  const symbols = FOREX_PAIRS.map((p) => p.yahooSymbol).join(",");
  const url = `https://query2.finance.yahoo.com/v8/finance/quote?symbols=${encodeURIComponent(symbols)}&fields=regularMarketPrice,regularMarketPreviousClose,regularMarketChange,regularMarketChangePercent,regularMarketDayHigh,regularMarketDayLow`;
  const res = await fetch(url, {
    headers: { "User-Agent": "Mozilla/5.0", Accept: "application/json" },
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Yahoo Finance returned ${res.status}`);
  const body = await res.json() as { quoteResponse?: { result?: YahooQuote[] } };
  return body.quoteResponse?.result ?? [];
}

export async function GET(): Promise<Response> {
  try {
    const quotes = await fetchYahooQuotes();
    const bySymbol = new Map(quotes.map((q) => [q.symbol ?? "", q]));

    const data: ForexMarketItem[] = FOREX_PAIRS.map((pair) => {
      const q = bySymbol.get(pair.yahooSymbol);
      return {
        symbol: pair.symbol,
        yahooSymbol: pair.yahooSymbol,
        price: n(q?.regularMarketPrice),
        prevClose: n(q?.regularMarketPreviousClose),
        change: n(q?.regularMarketChange),
        changePct: n(q?.regularMarketChangePercent),
        dayHigh: n(q?.regularMarketDayHigh),
        dayLow: n(q?.regularMarketDayLow),
      };
    });

    return NextResponse.json({ ok: true, data, fetchedAt: new Date().toISOString(), source: "yahoo-finance" });
  } catch (error) {
    const message = error instanceof Error ? error.message : "unknown error";
    return NextResponse.json({
      ok: false,
      error: message,
      data: FOREX_PAIRS.map((pair) => ({
        symbol: pair.symbol,
        yahooSymbol: pair.yahooSymbol,
        price: 0, prevClose: 0, change: 0, changePct: 0, dayHigh: 0, dayLow: 0,
      })),
      fetchedAt: new Date().toISOString(),
      source: "yahoo-finance",
    });
  }
}
