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
  error?: string;
};

export async function GET(): Promise<Response> {
  if (!isEngineProxyConfigured()) {
    return NextResponse.json({
      ok: false,
      ltp: 0,
      open: 0,
      high: 0,
      low: 0,
      close: 0,
      change: 0,
      changePct: 0,
      token: "",
      tradingSymbol: "",
      error:
        "Not configured — set LIGHTSAIL_ENGINE_URL to your Lightsail engine base so Angel One NSE quotes use the whitelisted IP.",
    } satisfies NiftyBeesLtpPayload);
  }

  try {
    const now = Date.now();
    if (!cache || now - cache.fetchedAt > TOKEN_CACHE_TTL) {
      const resolved = await resolveNiftyBeesToken();
      if (!resolved) {
        return NextResponse.json({
          ok: false,
          ltp: 0,
          open: 0,
          high: 0,
          low: 0,
          close: 0,
          change: 0,
          changePct: 0,
          token: "",
          tradingSymbol: "",
          error: "Could not resolve NIFTYBEES token via Angel One searchScrip (NSE).",
        } satisfies NiftyBeesLtpPayload);
      }
      cache = { ...resolved, fetchedAt: now };
    }

    const res = await engineProxyFetch("/rest/secure/angelbroking/market/v1/quote/", {
      mode: "LTP",
      exchangeTokens: { NSE: [cache.token] },
    });

    if (!res.ok) {
      return NextResponse.json({
        ok: false,
        ltp: 0,
        open: 0,
        high: 0,
        low: 0,
        close: 0,
        change: 0,
        changePct: 0,
        token: cache.token,
        tradingSymbol: cache.tradingSymbol,
        error: `Angel One LTP returned ${res.status}`,
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
      return NextResponse.json({
        ok: false,
        ltp: 0,
        open: 0,
        high: 0,
        low: 0,
        close: 0,
        change: 0,
        changePct: 0,
        token: cache.token,
        tradingSymbol: cache.tradingSymbol,
        error: msg,
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
      ...(price <= 0 ? { error: "Angel One returned zero LTP for NIFTYBEES." } : {}),
    } satisfies NiftyBeesLtpPayload);
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({
      ok: false,
      ltp: 0,
      open: 0,
      high: 0,
      low: 0,
      close: 0,
      change: 0,
      changePct: 0,
      token: cache?.token ?? "",
      tradingSymbol: cache?.tradingSymbol ?? "",
      error: message,
    } satisfies NiftyBeesLtpPayload);
  }
}
