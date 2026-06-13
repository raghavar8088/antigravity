import { engineProxyFetch } from "@/lib/broker/engineProxy";

type AngelSearchItem = {
  tradingsymbol?: string;
  symboltoken?: string;
  instrumenttype?: string;
  exch_seg?: string;
  exchange?: string;
  name?: string;
};

export type NiftyBeesResolvedToken = {
  token: string;
  tradingSymbol: string;
};

function normalize(value: string | undefined): string {
  return (value ?? "").toUpperCase().replace(/[^A-Z0-9]/g, "");
}

async function searchScripNSE(query: string): Promise<AngelSearchItem[]> {
  const res = await engineProxyFetch("/rest/secure/angelbroking/order/v1/searchScrip", {
    exchange: "NSE",
    searchscrip: query,
  });
  if (!res.ok) return [];

  const payload = await res.json() as { status?: boolean; data?: AngelSearchItem[] };
  return payload.status && Array.isArray(payload.data) ? payload.data : [];
}

function isNSEListedCashOrEtf(item: AngelSearchItem): boolean {
  const exch = normalize(item.exchange) || normalize(item.exch_seg);
  if (exch !== "NSE") return false;

  const instr = normalize(item.instrumenttype);
  const sym = normalize(item.tradingsymbol);
  if (sym.endsWith("CE") || sym.endsWith("PE")) return false;

  return instr === "EQ" || instr.includes("ETF") || sym.includes("NIFTYBEES");
}

function scoreCandidate(item: AngelSearchItem): number {
  const sym = normalize(item.tradingsymbol);
  let score = 0;
  if (sym === "NIFTYBEES" || sym.startsWith("NIFTYBEESEQ")) score += 500;
  else if (sym.includes("NIFTYBEES")) score += 400;
  const name = normalize(item.name);
  if (name.includes("NIFTY") && name.includes("BEES")) score += 120;
  if (name.includes("ETF") && name.includes("NIFTY")) score += 80;
  return score;
}

/**
 * Resolve Angel One symbol token for Nippon India ETF Nifty BeES (NIFTYBEES) on NSE.
 */
export async function resolveNiftyBeesToken(): Promise<NiftyBeesResolvedToken | null> {
  const queries = ["NIFTYBEES", "NIP IND ETF NIFTY BEES", "NIPPON INDIA ETF NIFTY", "NIFTY BEES"];
  const candidatesByToken = new Map<string, AngelSearchItem>();

  for (const query of queries) {
    try {
      const results = await searchScripNSE(query);
      for (const item of results) {
        if (!item.symboltoken || candidatesByToken.has(item.symboltoken)) continue;
        if (!isNSEListedCashOrEtf(item)) continue;
        candidatesByToken.set(item.symboltoken, item);
      }
    } catch {
      // keep trying other queries
    }
  }

  const list = Array.from(candidatesByToken.values()).filter((i) => i.symboltoken);
  if (!list.length) return null;

  list.sort((a, b) => scoreCandidate(b) - scoreCandidate(a));
  const winner = list[0];
  if (!winner?.symboltoken) return null;

  return {
    token: winner.symboltoken,
    tradingSymbol: winner.tradingsymbol ?? "NIFTYBEES",
  };
}
