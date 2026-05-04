import { NextResponse } from "next/server";

/** India Delta REST (same default as `/api/probe/delta-btc`). CDN base in `deltaAuth` is for signed routes. */
const DELTA_REST_BASE =
  process.env.DELTA_API_BASE_URL?.replace(/\/$/, "") ?? "https://api.india.delta.exchange";

const DEFAULT_SYMBOL = process.env.DELTA_BTC_CANDLE_SYMBOL?.trim() || "BTCUSD";

const JSON_HEADERS = {
  Accept: "application/json",
  "User-Agent": "RAIG-Trading/1.0",
};

function n(value: unknown): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

type DeltaCandle = {
  time?: unknown;
  open?: unknown;
  high?: unknown;
  low?: unknown;
  close?: unknown;
  volume?: unknown;
};

type DeltaCandlesResponse = {
  success?: boolean;
  result?: DeltaCandle[];
  error?: { code?: string; context?: string };
};

type DeltaTickerResponse = {
  success?: boolean;
  result?: {
    open?: unknown;
    close?: unknown;
    mark_price?: unknown;
    spot_price?: unknown;
    /** Percent string from Delta, e.g. "2.2064" */
    ltp_change_24h?: unknown;
  };
};

/** 1m closes + volumes for BTC (Delta symbol, default BTCUSD perpetual) — BTC spot scalper paper engine. */
export async function GET(): Promise<Response> {
  const symbol = DEFAULT_SYMBOL;
  const endSec = Math.floor(Date.now() / 1000);
  const startSec = endSec - 130 * 60;

  try {
    const candlesPath =
      `/v2/history/candles?resolution=1m&symbol=${encodeURIComponent(symbol)}` +
      `&start=${startSec}&end=${endSec}`;
    const tickerPath = `/v2/tickers/${encodeURIComponent(symbol)}`;

    const [candlesRes, tickerRes] = await Promise.all([
      fetch(`${DELTA_REST_BASE}${candlesPath}`, { headers: JSON_HEADERS, cache: "no-store" }),
      fetch(`${DELTA_REST_BASE}${tickerPath}`, { headers: JSON_HEADERS, cache: "no-store" }),
    ]);

    if (!candlesRes.ok) {
      throw new Error(`Delta history/candles returned ${candlesRes.status}`);
    }

    const candlesJson = (await candlesRes.json()) as DeltaCandlesResponse;
    if (!candlesJson.success || !Array.isArray(candlesJson.result)) {
      const msg =
        candlesJson.error?.context ??
        candlesJson.error?.code ??
        "Delta candles response was not successful";
      throw new Error(msg);
    }

    const rows = [...candlesJson.result].sort((a, b) => n(a.time) - n(b.time));
    const closes: number[] = [];
    const volumes: number[] = [];

    for (const row of rows) {
      const close = n(row.close);
      const vol = n(row.volume);
      if (close > 0) {
        closes.push(close);
        volumes.push(vol);
      }
    }

    let changePct24h = 0;
    if (tickerRes.ok) {
      const tj = (await tickerRes.json()) as DeltaTickerResponse;
      if (tj.success && tj.result) {
        const r = tj.result;
        const ltp24 = n(r.ltp_change_24h);
        if (ltp24 !== 0) {
          changePct24h = ltp24;
        } else {
          const open = n(r.open);
          const close = n(r.close) || n(r.mark_price) || n(r.spot_price);
          if (open > 0 && close > 0) {
            changePct24h = ((close - open) / open) * 100;
          }
        }
      }
    }

    const lastPrice = closes.length > 0 ? closes[closes.length - 1] : 0;

    return NextResponse.json({
      ok: true,
      closes,
      volumes,
      lastPrice,
      changePct24h,
      fetchedAt: new Date().toISOString(),
      source: "delta-exchange-1m",
      symbol,
      deltaBase: DELTA_REST_BASE,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "unknown error";
    return NextResponse.json(
      {
        ok: false,
        error: message,
        closes: [],
        volumes: [],
        lastPrice: 0,
        changePct24h: 0,
        fetchedAt: new Date().toISOString(),
        symbol,
        deltaBase: DELTA_REST_BASE,
      },
      { status: 502 },
    );
  }
}
