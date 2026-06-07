import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

const SOURCES = [
  // Binance — primary
  {
    name: "binance",
    url: "https://api.binance.com/api/v3/ticker/24hr?symbol=BTCUSDT",
    parse: (d: Record<string, string>) => ({
      price: parseFloat(d.lastPrice),
      change24h: parseFloat(d.priceChangePercent),
      high24h: parseFloat(d.highPrice),
      low24h: parseFloat(d.lowPrice),
      volume24h: parseFloat(d.volume),
    }),
  },
  // Coinbase — fallback
  {
    name: "coinbase",
    url: "https://api.coinbase.com/v2/prices/BTC-USD/spot",
    parse: (d: { data: { amount: string } }) => ({
      price: parseFloat(d.data.amount),
      change24h: 0,
      high24h: 0,
      low24h: 0,
      volume24h: 0,
    }),
  },
];

export async function GET() {
  for (const source of SOURCES) {
    try {
      const res = await fetch(source.url, {
        signal: AbortSignal.timeout(4_000),
        headers: { "Cache-Control": "no-store" },
      });
      if (!res.ok) continue;
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const data = await res.json() as any;
      const parsed = source.parse(data);
      if (!Number.isFinite(parsed.price) || parsed.price <= 0) continue;
      return NextResponse.json({ ok: true, source: source.name, ...parsed }, {
        headers: { "Cache-Control": "no-store, max-age=0" },
      });
    } catch {
      // try next source
    }
  }
  return NextResponse.json({ ok: false, error: "All price sources failed" }, { status: 503 });
}
