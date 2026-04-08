import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, angelMissingEnv, commonHeaders, BASE_URL } from "@/lib/angelAuth";
import { NIFTY50_STOCKS, STOCK_BY_TOKEN, ALL_TOKENS } from "@/lib/nifty50Stocks";

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
};

type AngelBatchItem = {
  symbolToken?: unknown;
  ltp?: unknown;
  open?: unknown;
  high?: unknown;
  low?: unknown;
  close?: unknown;
  lastPrice?: unknown;
};

function n(v: unknown): number {
  const num = Number(v);
  return Number.isFinite(num) ? num : 0;
}

/**
 * Batch-fetch LTP for two chunks of tokens in parallel to stay within limits.
 * Angel One market/v1/quote accepts up to 50 tokens per request.
 */
async function fetchBatch(jwt: string, tokens: string[]): Promise<AngelBatchItem[]> {
  const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/market/v1/quote/`, {
    method: "POST",
    headers: commonHeaders(jwt),
    body: JSON.stringify({
      mode: "LTP",
      exchangeTokens: { NSE: tokens },
    }),
    next: { revalidate: 0 },
  });

  if (!res.ok) throw new Error(`Angel batch LTP status ${res.status}`);

  const payload = await res.json() as {
    status?: boolean;
    message?: string;
    errorcode?: string;
    data?: {
      fetched?: AngelBatchItem[];
      unfetched?: unknown[];
    };
  };

  if (!payload.status || !payload.data?.fetched) {
    const msg = [payload.message, payload.errorcode].filter(Boolean).join(" ") || "Batch LTP failed";
    throw new Error(msg);
  }

  return payload.data.fetched;
}

export async function GET(): Promise<Response> {
  if (!isAngelConfigured()) {
    const empty: StockLTPResponse = {
      ok: false,
      error: `Not configured — set: ${angelMissingEnv()}`,
      stocks: [],
      fetchedAt: new Date().toISOString(),
    };
    return NextResponse.json(empty);
  }

  try {
    const jwt = await getAngelJWT();

    // Split 50 tokens into two batches of 25 to be safe
    const chunk1 = ALL_TOKENS.slice(0, 25);
    const chunk2 = ALL_TOKENS.slice(25);

    const [batch1, batch2] = await Promise.all([
      fetchBatch(jwt, chunk1),
      fetchBatch(jwt, chunk2),
    ]);

    const allItems = [...batch1, ...batch2];

    // Build result map: token → parsed quote
    const resultMap = new Map<string, StockLTPItem>();

    for (const item of allItems) {
      const token = String(item.symbolToken ?? "");
      const stock = STOCK_BY_TOKEN[token];
      if (!stock) continue;

      const ltp = n(item.ltp ?? item.lastPrice);
      const close = n(item.close);
      const changePct = close > 0 ? ((ltp - close) / close) * 100 : 0;

      resultMap.set(token, {
        symbol: stock.symbol,
        token,
        ltp,
        open: n(item.open),
        high: n(item.high),
        low: n(item.low),
        close,
        changePct,
      });
    }

    // Ensure all 50 stocks appear in output (fill missing with zeros)
    const stocks: StockLTPItem[] = NIFTY50_STOCKS.map((s) =>
      resultMap.get(s.token) ?? {
        symbol: s.symbol,
        token: s.token,
        ltp: 0,
        open: 0,
        high: 0,
        low: 0,
        close: 0,
        changePct: 0,
      },
    );

    return NextResponse.json({
      ok: true,
      error: "",
      stocks,
      fetchedAt: new Date().toISOString(),
    } satisfies StockLTPResponse);
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({
      ok: false,
      error: message,
      stocks: [],
      fetchedAt: new Date().toISOString(),
    } satisfies StockLTPResponse);
  }
}
