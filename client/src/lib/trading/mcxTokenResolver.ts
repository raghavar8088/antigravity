import { engineProxyFetch } from "@/lib/broker/engineProxy";
import type { MCXCommodity } from "@/lib/trading/mcxCommodities";

type AngelSearchItem = {
  tradingsymbol?: string;
  symboltoken?: string;
  expiry?: string;
  instrumenttype?: string;
  exch_seg?: string;
  exchange?: string;
  name?: string;
};

export type MCXResolvedToken = {
  token: string;
  tradingSymbol: string;
  expiry: string;
};

function normalize(value: string | undefined): string {
  return (value ?? "").toUpperCase().replace(/[^A-Z0-9]/g, "");
}

function uniqueStrings(values: Array<string | undefined>): string[] {
  return Array.from(
    new Set(
      values
        .map((value) => value?.trim())
        .filter((value): value is string => !!value),
    ),
  );
}

function parseExpiry(expiry?: string): number {
  if (!expiry) return Number.MAX_SAFE_INTEGER;

  // Standard JS-parseable formats (ISO, "30 Apr 2025", etc.)
  const direct = new Date(expiry).getTime();
  if (Number.isFinite(direct)) return direct;

  // Angel One format: "30APR2025" → "30 Apr 2025"
  const m = expiry.match(/^(\d{1,2})([A-Za-z]{3})(\d{4})$/);
  if (m) {
    const spaced = new Date(`${m[1]} ${m[2]} ${m[3]}`).getTime();
    if (Number.isFinite(spaced)) return spaced;
  }

  return Number.MAX_SAFE_INTEGER;
}

function isMCXFuture(item: AngelSearchItem): boolean {
  // Angel One uses `exchange` in searchScrip responses; some endpoints use `exch_seg`.
  const exch = normalize(item.exchange) || normalize(item.exch_seg);
  if (exch !== "MCX") return false;

  const instrType = normalize(item.instrumenttype);
  const symbol = normalize(item.tradingsymbol);

  // Reject options outright (anything that ends in CE/PE is an option, not a future).
  if (symbol.endsWith("CE") || symbol.endsWith("PE")) return false;

  // Accept if instrumenttype starts with FUT (e.g. FUTCOM, FUTCUR)
  // OR if tradingsymbol itself contains FUT (Angel One sometimes omits instrumenttype)
  return instrType.startsWith("FUT") || symbol.includes("FUT");
}

function getSymbolMatchScore(symbol: string, hints: string[]): number {
  let score = 0;
  for (const hint of hints) {
    if (!hint) continue;
    if (symbol === hint) score = Math.max(score, 500 + hint.length);
    else if (symbol.startsWith(hint)) score = Math.max(score, 400 + hint.length);
    else if (symbol.includes(hint)) score = Math.max(score, 250 + hint.length);
  }
  return score;
}

function buildSearchQueries(commodity: MCXCommodity): string[] {
  const compactName = commodity.name.replace(/\s+/g, "");
  return uniqueStrings([
    commodity.searchQuery,
    ...(commodity.searchQueries ?? []),
    commodity.id,
    compactName,
    commodity.name,
  ]);
}

function buildSymbolHints(commodity: MCXCommodity): string[] {
  const compactName = commodity.name.replace(/\s+/g, "");
  return uniqueStrings([
    commodity.searchQuery,
    ...(commodity.symbolHints ?? []),
    commodity.id,
    compactName,
  ]).map((hint) => normalize(hint));
}

async function searchScrip(query: string): Promise<AngelSearchItem[]> {
  const res = await engineProxyFetch(
    "/rest/secure/angelbroking/order/v1/searchScrip",
    { exchange: "MCX", searchscrip: query },
  );
  if (!res.ok) return [];

  const payload = await res.json() as { status?: boolean; data?: AngelSearchItem[] };
  return payload.status && Array.isArray(payload.data) ? payload.data : [];
}

export async function resolveMCXFutureToken(commodity: MCXCommodity): Promise<MCXResolvedToken | null> {
  const queries = buildSearchQueries(commodity);
  const hints = buildSymbolHints(commodity);
  const candidatesByToken = new Map<string, AngelSearchItem>();

  for (const query of queries) {
    try {
      const results = await searchScrip(query);
      for (const item of results) {
        if (!item.symboltoken || candidatesByToken.has(item.symboltoken)) continue;
        candidatesByToken.set(item.symboltoken, item);
      }
    } catch {
      // Ignore individual query failures and keep trying the remaining aliases.
    }
  }

  const futures = Array.from(candidatesByToken.values()).filter((item) => item.symboltoken && isMCXFuture(item));
  if (!futures.length) return null;

  const preferred = futures.filter((item) => getSymbolMatchScore(normalize(item.tradingsymbol), hints) > 0);
  const ranked = (preferred.length ? preferred : futures).sort((a, b) => {
    const scoreA = getSymbolMatchScore(normalize(a.tradingsymbol), hints);
    const scoreB = getSymbolMatchScore(normalize(b.tradingsymbol), hints);
    if (scoreA !== scoreB) return scoreB - scoreA;
    return parseExpiry(a.expiry) - parseExpiry(b.expiry);
  });

  const winner = ranked[0];
  if (!winner?.symboltoken) return null;

  return {
    token: winner.symboltoken,
    tradingSymbol: winner.tradingsymbol ?? commodity.searchQuery,
    expiry: winner.expiry ?? "",
  };
}
