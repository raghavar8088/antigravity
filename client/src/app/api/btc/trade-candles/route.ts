/**
 * FEAT 3 — BTC Trade Candles from Binance
 * GET ?from=ms&to=ms&interval=1m
 */
import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

interface BinanceKline {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

export async function GET(req: Request) {
  const url = new URL(req.url);
  const from = url.searchParams.get("from");
  const to = url.searchParams.get("to");
  const interval = url.searchParams.get("interval") ?? "1m";

  if (!from || !to) {
    return NextResponse.json({ error: "from and to params are required" }, { status: 400 });
  }

  const fromMs = parseInt(from, 10);
  const toMs = parseInt(to, 10);

  if (isNaN(fromMs) || isNaN(toMs) || fromMs >= toMs) {
    return NextResponse.json({ error: "invalid from/to range" }, { status: 400 });
  }

  try {
    const binanceUrl = new URL("https://api.binance.com/api/v3/klines");
    binanceUrl.searchParams.set("symbol", "BTCUSDT");
    binanceUrl.searchParams.set("interval", interval);
    binanceUrl.searchParams.set("startTime", String(fromMs));
    binanceUrl.searchParams.set("endTime", String(toMs));
    binanceUrl.searchParams.set("limit", "1000");

    const res = await fetch(binanceUrl.toString(), { next: { revalidate: 0 } });
    if (!res.ok) {
      return NextResponse.json({ error: `Binance error ${res.status}` }, { status: 502 });
    }

    // Binance returns arrays: [openTime, open, high, low, close, volume, ...]
    const raw = (await res.json()) as Array<[number, string, string, string, string, string, ...unknown[]]>;

    const candles: BinanceKline[] = raw.map((k) => ({
      time: k[0],
      open: parseFloat(k[1]),
      high: parseFloat(k[2]),
      low: parseFloat(k[3]),
      close: parseFloat(k[4]),
      volume: parseFloat(k[5]),
    }));

    return NextResponse.json({ candles });
  } catch (err) {
    return NextResponse.json(
      { error: err instanceof Error ? err.message : "unknown" },
      { status: 500 },
    );
  }
}
