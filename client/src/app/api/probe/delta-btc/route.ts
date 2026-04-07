import { NextResponse } from "next/server";

const DELTA_BASE = process.env.DELTA_API_BASE_URL?.replace(/\/$/, "") ?? "https://api.india.delta.exchange";
const SYMBOL = "BTCUSD";

function parseNum(v: unknown): number {
  if (v === null || v === undefined) return 0;
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
}

export async function GET() {
  try {
    const res = await fetch(`${DELTA_BASE}/v2/tickers/${SYMBOL}`, {
      headers: { Accept: "application/json" },
      next: { revalidate: 0 },
    });

    if (!res.ok) {
      return NextResponse.json({ ok: false, source: "delta_exchange", symbol: SYMBOL, error: `Delta returned status ${res.status}` });
    }

    const payload = await res.json() as {
      success?: boolean;
      result?: {
        symbol?: string;
        close?: unknown;
        open?: unknown;
        high?: unknown;
        low?: unknown;
        volume?: unknown;
        turnover?: unknown;
        mark_price?: unknown;
        spot_price?: unknown;
        contract_type?: string;
        timestamp?: unknown;
      };
    };

    if (!payload.success || !payload.result) {
      return NextResponse.json({ ok: false, source: "delta_exchange", symbol: SYMBOL, error: "Delta response was not successful" });
    }

    const r = payload.result;
    const fetchedAt = new Date().toISOString();
    const tsRaw = Number(r.timestamp);
    const exchangeTime = tsRaw > 0 ? new Date(tsRaw * 1000).toISOString() : fetchedAt;

    return NextResponse.json({
      ok: true,
      error: "",
      source: "delta_exchange",
      symbol: r.symbol ?? SYMBOL,
      price: parseNum(r.close),
      open: parseNum(r.open),
      high: parseNum(r.high),
      low: parseNum(r.low),
      volume: parseNum(r.volume),
      turnover: parseNum(r.turnover),
      mark_price: parseNum(r.mark_price),
      spot_price: parseNum(r.spot_price),
      contract_type: r.contract_type ?? "",
      exchange_time: exchangeTime,
      fetched_at: fetchedAt,
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, source: "delta_exchange", symbol: SYMBOL, error: message });
  }
}
