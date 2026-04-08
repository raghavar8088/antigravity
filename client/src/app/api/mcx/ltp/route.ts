import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, angelMissingEnv, commonHeaders, BASE_URL } from "@/lib/angelAuth";
import { MCX_COMMODITIES } from "@/lib/mcxCommodities";

// ── In-memory token cache (survives warm Vercel invocations) ──────────────────

interface TokenEntry {
  token: string;
  tradingSymbol: string;
  expiry: string;
}

const tokenCache: {
  data: Record<string, TokenEntry>;
  fetchedAt: number;
} = { data: {}, fetchedAt: 0 };

const TOKEN_CACHE_TTL = 4 * 60 * 60 * 1000; // 4 hours

async function fetchToken(jwt: string, searchQuery: string): Promise<TokenEntry | null> {
  try {
    const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/order/v1/searchScrip`, {
      method: "POST",
      headers: commonHeaders(jwt),
      body: JSON.stringify({ exchange: "MCX", searchscrip: searchQuery }),
      next: { revalidate: 0 },
    });
    if (!res.ok) return null;
    const payload = await res.json() as {
      status?: boolean;
      data?: Array<{
        tradingsymbol?: string;
        symboltoken?: string;
        expiry?: string;
        instrumenttype?: string;
        exch_seg?: string;
      }>;
    };
    if (!payload.status || !Array.isArray(payload.data)) return null;

    // Filter FUTCOM on MCX where tradingsymbol starts with searchQuery (exact prefix match)
    const futures = payload.data.filter(
      (d) =>
        d.instrumenttype === "FUTCOM" &&
        d.exch_seg === "MCX" &&
        d.tradingsymbol?.toUpperCase().startsWith(searchQuery.toUpperCase()),
    );
    if (!futures.length) return null;

    // Sort by expiry ascending (nearest first)
    futures.sort((a, b) => {
      const parseExp = (s: string | undefined) => {
        if (!s) return 0;
        try { return new Date(s).getTime(); } catch { return 0; }
      };
      return parseExp(a.expiry) - parseExp(b.expiry);
    });

    const first = futures[0];
    if (!first.symboltoken) return null;
    return {
      token: first.symboltoken,
      tradingSymbol: first.tradingsymbol ?? searchQuery,
      expiry: first.expiry ?? "",
    };
  } catch {
    return null;
  }
}

async function getCachedTokens(jwt: string): Promise<Record<string, TokenEntry>> {
  const now = Date.now();
  if (tokenCache.fetchedAt > 0 && now - tokenCache.fetchedAt < TOKEN_CACHE_TTL) {
    return tokenCache.data;
  }
  // Fetch all tokens in parallel
  const entries = await Promise.all(
    MCX_COMMODITIES.map(async (c) => {
      const entry = await fetchToken(jwt, c.searchQuery);
      return [c.id, entry] as [string, TokenEntry | null];
    }),
  );
  const data: Record<string, TokenEntry> = {};
  for (const [id, entry] of entries) {
    if (entry) data[id] = entry;
  }
  tokenCache.data = data;
  tokenCache.fetchedAt = now;
  return data;
}

// ── LTP batch fetch ────────────────────────────────────────────────────────────

export type MCXLTPItem = {
  id: string;
  name: string;
  token: string;
  tradingSymbol: string;
  expiry: string;
  ltp: number;
  open: number;
  high: number;
  low: number;
  close: number;
  change: number;
  changePct: number;
};

export async function GET(): Promise<Response> {
  if (!isAngelConfigured()) {
    return NextResponse.json({ ok: false, error: `Not configured — set: ${angelMissingEnv()}`, data: [] });
  }

  try {
    const jwt = await getAngelJWT();
    const tokens = await getCachedTokens(jwt);

    const tokenIds = Object.values(tokens).map((t) => t.token).filter(Boolean);
    if (!tokenIds.length) {
      return NextResponse.json({ ok: false, error: "Could not resolve any MCX tokens", data: [] });
    }

    const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/market/v1/quote/`, {
      method: "POST",
      headers: commonHeaders(jwt),
      body: JSON.stringify({ mode: "LTP", exchangeTokens: { MCX: tokenIds } }),
      next: { revalidate: 0 },
    });

    if (!res.ok) {
      return NextResponse.json({ ok: false, error: `Angel One LTP returned ${res.status}`, data: [] });
    }

    const payload = await res.json() as {
      status?: boolean;
      data?: {
        fetched?: Array<{
          symbolToken?: string;
          ltp?: number;
          open?: number;
          high?: number;
          low?: number;
          close?: number;
        }>;
      };
    };

    if (!payload.status || !payload.data?.fetched) {
      return NextResponse.json({ ok: false, error: "LTP response invalid", data: [] });
    }

    const ltpByToken: Record<string, (typeof payload.data.fetched)[0]> = {};
    for (const item of payload.data.fetched) {
      if (item.symbolToken) ltpByToken[item.symbolToken] = item;
    }

    const results: MCXLTPItem[] = MCX_COMMODITIES.map((c) => {
      const entry = tokens[c.id];
      const ltp = entry ? ltpByToken[entry.token] : undefined;
      const price = ltp?.ltp ?? 0;
      const prev = ltp?.close ?? 0;
      const change = prev > 0 ? price - prev : 0;
      const changePct = prev > 0 ? (change / prev) * 100 : 0;
      return {
        id: c.id,
        name: c.name,
        token: entry?.token ?? "",
        tradingSymbol: entry?.tradingSymbol ?? "",
        expiry: entry?.expiry ?? "",
        ltp: price,
        open: ltp?.open ?? 0,
        high: ltp?.high ?? 0,
        low: ltp?.low ?? 0,
        close: prev,
        change,
        changePct,
      };
    });

    return NextResponse.json({ ok: true, data: results });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, error: message, data: [] });
  }
}
