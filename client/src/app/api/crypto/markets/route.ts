import { NextResponse } from "next/server";
import { CRYPTO_TOP_20 } from "@/lib/trading/cryptoTop20";

export type CryptoMarketItem = {
  symbol: string;
  pair: string;
  price: number;
  open: number;
  high: number;
  low: number;
  prevClose: number;
  volume: number;
  changePct: number;
};

type BinanceTicker = {
  symbol?: string;
  lastPrice?: string;
  openPrice?: string;
  highPrice?: string;
  lowPrice?: string;
  prevClosePrice?: string;
  volume?: string;
  priceChangePercent?: string;
};

function n(value: unknown): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

const BINANCE_HEADERS = {
  "User-Agent": "Mozilla/5.0",
  Accept: "application/json",
};

async function fetchBatchTickers(): Promise<BinanceTicker[]> {
  const symbols = JSON.stringify(CRYPTO_TOP_20.map((asset) => asset.pair));
  const response = await fetch(
    `https://api.binance.com/api/v3/ticker/24hr?symbols=${encodeURIComponent(symbols)}`,
    { headers: BINANCE_HEADERS, cache: "no-store" },
  );

  if (!response.ok) {
    throw new Error(`Binance ticker returned ${response.status}`);
  }

  return await response.json() as BinanceTicker[];
}

export async function GET(): Promise<Response> {
  try {
    const tickers = await fetchBatchTickers();
    const byPair = new Map(tickers.map((ticker) => [ticker.symbol ?? "", ticker]));

    const data: CryptoMarketItem[] = CRYPTO_TOP_20.map((asset) => {
      const ticker = byPair.get(asset.pair);
      return {
        symbol: asset.symbol,
        pair: asset.pair,
        price: n(ticker?.lastPrice),
        open: n(ticker?.openPrice),
        high: n(ticker?.highPrice),
        low: n(ticker?.lowPrice),
        prevClose: n(ticker?.prevClosePrice),
        volume: n(ticker?.volume),
        changePct: n(ticker?.priceChangePercent),
      };
    });

    return NextResponse.json({
      ok: true,
      data,
      fetchedAt: new Date().toISOString(),
      source: "binance-24hr",
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "unknown error";
    return NextResponse.json({
      ok: false,
      error: message,
      data: CRYPTO_TOP_20.map((asset) => ({
        symbol: asset.symbol,
        pair: asset.pair,
        price: 0,
        open: 0,
        high: 0,
        low: 0,
        prevClose: 0,
        volume: 0,
        changePct: 0,
      })),
      fetchedAt: new Date().toISOString(),
      source: "binance-24hr",
    });
  }
}
