import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, angelMissingEnv, commonHeaders, BASE_URL } from "@/lib/angelAuth";
import { MCX_COMMODITIES } from "@/lib/mcxCommodities";
import { resolveMCXFutureToken } from "@/lib/mcxTokenResolver";

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

async function getCachedTokens(jwt: string): Promise<Record<string, TokenEntry>> {
  const now = Date.now();
  if (tokenCache.fetchedAt > 0 && now - tokenCache.fetchedAt < TOKEN_CACHE_TTL) {
    return tokenCache.data;
  }
  // Fetch all tokens in parallel
  const entries = await Promise.all(
    MCX_COMMODITIES.map(async (c) => {
      const entry = await resolveMCXFutureToken(jwt, c);
      return [c.id, entry] as [string, TokenEntry | null];
    }),
  );
  const data: Record<string, TokenEntry> = {};
  for (const [id, entry] of entries) {
    if (entry) data[id] = entry;
  }
  if (Object.keys(data).length > 0) {
    tokenCache.data = data;
    tokenCache.fetchedAt = now;
  } else {
    tokenCache.data = {};
    tokenCache.fetchedAt = 0;
  }
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

type AngelMCXQuoteItem = {
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

export async function GET(): Promise<Response> {
  if (!isAngelConfigured()) {
    return NextResponse.json({ ok: false, error: `Not configured — set: ${angelMissingEnv()}`, data: [] });
  }

  try {
    const jwt = await getAngelJWT();
    const tokens = await getCachedTokens(jwt);
    const unresolved = MCX_COMMODITIES.filter((commodity) => !tokens[commodity.id]).map((commodity) => commodity.name);

    const tokenIds = Object.values(tokens).map((t) => t.token).filter(Boolean);
    if (!tokenIds.length) {
      return NextResponse.json({ ok: false, error: `Could not resolve any MCX tokens for: ${unresolved.join(", ")}`, data: [] });
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
      message?: string;
      errorcode?: string;
      data?: {
        fetched?: AngelMCXQuoteItem[];
        unfetched?: unknown[];
      };
    };

    if (!payload.status || !payload.data?.fetched) {
      const msg = [payload.message, payload.errorcode].filter(Boolean).join(" ") || "LTP response invalid";
      return NextResponse.json({ ok: false, error: msg, data: [] });
    }

    const ltpByToken: Record<string, AngelMCXQuoteItem> = {};
    for (const item of payload.data.fetched) {
      const token = String(item.symbolToken ?? item.symboltoken ?? "");
      if (token) ltpByToken[token] = item;
    }

    const results: MCXLTPItem[] = MCX_COMMODITIES.map((c) => {
      const entry = tokens[c.id];
      const ltp = entry ? ltpByToken[entry.token] : undefined;
      const price = n(ltp?.ltp ?? ltp?.lastPrice);
      const prev = n(ltp?.close);
      const change = prev > 0 ? price - prev : 0;
      const changePct = prev > 0 ? (change / prev) * 100 : 0;
      return {
        id: c.id,
        name: c.name,
        token: entry?.token ?? "",
        tradingSymbol: entry?.tradingSymbol ?? "",
        expiry: entry?.expiry ?? "",
        ltp: price,
        open: n(ltp?.open),
        high: n(ltp?.high),
        low: n(ltp?.low),
        close: prev,
        change,
        changePct,
      };
    });

    const positiveQuotes = results.filter((item) => item.ltp > 0);
    if (!positiveQuotes.length) {
      const resolvedSymbols = results
        .filter((item) => item.token)
        .map((item) => item.tradingSymbol || item.id);
      return NextResponse.json({
        ok: false,
        error: `MCX quotes returned no positive prices for resolved symbols: ${resolvedSymbols.join(", ") || "none"}`,
        data: [],
      });
    }

    return NextResponse.json({ ok: true, data: results });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, error: message, data: [] });
  }
}
