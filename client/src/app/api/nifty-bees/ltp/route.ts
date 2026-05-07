import { NextResponse } from "next/server";
import { isEngineProxyConfigured, engineProxyFetch } from "@/lib/engineProxy";
import { resolveNiftyBeesToken } from "@/lib/niftyBeesTokenResolver";

interface TokenCache {
  token: string;
  tradingSymbol: string;
  fetchedAt: number;
}

let cache: TokenCache | null = null;
const TOKEN_CACHE_TTL = 4 * 60 * 60 * 1000;

type AngelQuoteItem = {
  symbolToken?: unknown;
  symboltoken?: unknown;
  ltp?: unknown;
  lastPrice?: unknown;
  open?: unknown;
  high?: unknown;
  low?: unknown;
  close?: unknown;
};

function n(value: unknown): number {
  const num = Number(value);
  return Number.isFinite(num) ? num : 0;
}

export type NiftyBeesLtpPayload = {
  ok: boolean;
  ltp: number;
  open: number;
  high: number;
  low: number;
  close: number;
  change: number;
  changePct: number;
  token: string;
  tradingSymbol: string;
  source?: "angel" | "yahoo";
  error?: string;
};

async function fetchYahooFallbackLtp(): Promise<NiftyBeesLtpPayload> {
  const urls = [
    "https://query1.finance.yahoo.com/v8/finance/chart/NIFTYBEES.NS?interval=1m&range=1d&includePrePost=false",
    "https://query2.finance.yahoo.com/v8/finance/chart/NIFTYBEES.NS?interval=1m&range=1d&includePrePost=false",
  ];

  for (const url of urls) {
    try {
      const res = await fetch(url, {
        cache: "no-store",
        headers: {
          "User-Agent": "Mozilla/5.0",
          Accept: "application/json",
        },
      });
      if (!res.ok) continue;
      const data = await res.json() as {
        chart?: {
          result?: Array<{
            indicators?: { quote?: Array<{ close?: Array<number | null>; open?: Array<number | null>; high?: Array<number | null>; low?: Array<number | null> }> };
            meta?: { previousClose?: number };
          }>;
        };
      };
      const result = data.chart?.result?.[0];
      const quote = result?.indicators?.quote?.[0];
      const closes = (quote?.close ?? []).filter((v): v is number => typeof v === "number" && Number.isFinite(v) && v > 0);
      if (!closes.length) continue;
      const ltp = closes[closes.length - 1];
      const prev = Number(result?.meta?.previousClose ?? 0);
      const change = prev > 0 ? ltp - prev : 0;
      const changePct = prev > 0 ? (change / prev) * 100 : 0;

      return {
        ok: true,
        ltp,
        open: Number(quote?.open?.[quote.open.length - 1] ?? 0),
        high: Number(quote?.high?.[quote.high.length - 1] ?? 0),
        low: Number(quote?.low?.[quote.low.length - 1] ?? 0),
        close: prev > 0 ? prev : ltp,
        change,
        changePct,
        token: "",
        tradingSymbol: "NIFTYBEES",
        source: "yahoo",
      };
    } catch {
      // try next mirror
    }
  }

  return {
    ok: false,
    ltp: 0,
    open: 0,
    high: 0,
    low: 0,
    close: 0,
    change: 0,
    changePct: 0,
    token: "",
    tradingSymbol: "NIFTYBEES",
    source: "yahoo",
    error: "Yahoo fallback failed",
  };
}

export async function GET(): Promise<Response> {
  if (!isEngineProxyConfigured()) {
    const fallback = await fetchYahooFallbackLtp();
    if (fallback.ok) return NextResponse.json(fallback);
    return NextResponse.json({
      ...fallback,
      error:
        "Not configured for Angel and Yahoo fallback failed. Set LIGHTSAIL_ENGINE_URL.",
    } satisfies NiftyBeesLtpPayload);
  }

  try {
    const now = Date.now();
    if (!cache || now - cache.fetchedAt > TOKEN_CACHE_TTL) {
      const resolved = await resolveNiftyBeesToken();
      if (!resolved) {
        const fallback = await fetchYahooFallbackLtp();
        if (fallback.ok) return NextResponse.json(fallback);
        return NextResponse.json({
          ...fallback,
          error: "Could not resolve NIFTYBEES token via Angel One searchScrip (NSE), and Yahoo fallback failed.",
        } satisfies NiftyBeesLtpPayload);
      }
      cache = { ...resolved, fetchedAt: now };
    }

    const res = await engineProxyFetch("/rest/secure/angelbroking/market/v1/quote/", {
      mode: "LTP",
      exchangeTokens: { NSE: [cache.token] },
    });

    if (!res.ok) {
      const fallback = await fetchYahooFallbackLtp();
      if (fallback.ok) return NextResponse.json(fallback);
      return NextResponse.json({
        ...fallback,
        token: cache.token,
        tradingSymbol: cache.tradingSymbol,
        error: `Angel One LTP returned ${res.status}, and Yahoo fallback failed.`,
      } satisfies NiftyBeesLtpPayload);
    }

    const payload = await res.json() as {
      status?: boolean;
      message?: string;
      errorcode?: string;
      data?: { fetched?: AngelQuoteItem[] };
    };

    if (!payload.status || !payload.data?.fetched?.length) {
      const msg = [payload.message, payload.errorcode].filter(Boolean).join(" ") || "LTP response invalid";
      const fallback = await fetchYahooFallbackLtp();
      if (fallback.ok) return NextResponse.json(fallback);
      return NextResponse.json({
        ...fallback,
        token: cache.token,
        tradingSymbol: cache.tradingSymbol,
        error: `${msg}, and Yahoo fallback failed.`,
      } satisfies NiftyBeesLtpPayload);
    }

    const row = payload.data.fetched[0];
    const price = n(row?.ltp ?? row?.lastPrice);
    const prev = n(row?.close);
    const change = prev > 0 ? price - prev : 0;
    const changePct = prev > 0 ? (change / prev) * 100 : 0;

    return NextResponse.json({
      ok: price > 0,
      ltp: price,
      open: n(row?.open),
      high: n(row?.high),
      low: n(row?.low),
      close: prev,
      change,
      changePct,
      token: cache.token,
      tradingSymbol: cache.tradingSymbol,
      source: "angel",
      ...(price <= 0 ? { error: "Angel One returned zero LTP for NIFTYBEES." } : {}),
    } satisfies NiftyBeesLtpPayload);
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    const fallback = await fetchYahooFallbackLtp();
    if (fallback.ok) return NextResponse.json(fallback);
    return NextResponse.json({
      ...fallback,
      token: cache?.token ?? "",
      tradingSymbol: cache?.tradingSymbol ?? "",
      error: `${message}; Yahoo fallback failed.`,
    } satisfies NiftyBeesLtpPayload);
  }
}
