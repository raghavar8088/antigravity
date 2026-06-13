import { NextResponse } from "next/server";
import { DELTA_BASE, isDeltaConfigured, deltaHeaders } from "@/lib/broker/deltaAuth";
import type { ChainData, ChainRow, ChainLeg, ExpiryMeta } from "@/hooks/useOptionChain";

// ── Date helpers ──────────────────────────────────────────────────────────────

/** Returns the date string "YYYY-MM-DD" for a Date object */
function toDateStr(d: Date): string {
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, "0");
  const dd = String(d.getUTCDate()).padStart(2, "0");
  return `${y}-${m}-${dd}`;
}

/** Returns the next N Fridays (inclusive of today if it's Friday) from today */
function nextNFridays(n: number): Date[] {
  const result: Date[] = [];
  const now = new Date();
  // Start from today UTC midnight
  const d = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
  // Move forward until we hit a Friday (day 5 = Friday in UTC)
  while (d.getUTCDay() !== 5) {
    d.setUTCDate(d.getUTCDate() + 1);
  }
  for (let i = 0; i < n; i++) {
    result.push(new Date(d));
    d.setUTCDate(d.getUTCDate() + 7);
  }
  return result;
}

/** Days until a given date from today */
function dte(target: Date): number {
  const now = new Date();
  const todayUTC = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
  return Math.round((target.getTime() - todayUTC) / 86400000);
}

/** Format expiry label e.g. "25 Apr 25" */
function expiryLabel(d: Date): string {
  const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
  return `${d.getUTCDate()} ${months[d.getUTCMonth()]} ${String(d.getUTCFullYear()).slice(2)}`;
}

// ── Delta response types ──────────────────────────────────────────────────────

type DeltaGreeks = {
  delta?: string;
  gamma?: string;
  theta?: string;
  vega?: string;
  iv?: string;
};

type DeltaOptionSide = {
  symbol?: string;
  mark_price?: string;
  best_bid_price?: string;
  best_ask_price?: string;
  oi?: string;
  volume?: string;
  greeks?: DeltaGreeks;
};

type DeltaChainRow = {
  strike_price: number;
  call_options?: DeltaOptionSide;
  put_options?: DeltaOptionSide;
};

type DeltaChainResponse = {
  success: boolean;
  result?: DeltaChainRow[];
  error?: { code?: string; context?: string };
};

type DeltaTickerResponse = {
  success: boolean;
  result?: { mark_price?: string; spot_price?: string; close?: string };
};

// ── Mapper helpers ────────────────────────────────────────────────────────────

function mapLeg(side: DeltaOptionSide | undefined, spot: number, strike: number, isCall: boolean): ChainLeg {
  if (!side) {
    return { iv: 0, delta: 0, gamma: 0, theta: 0, vega: 0, mark: 0, bid: 0, ask: 0, oi: 0, volume: 0, isItm: false };
  }
  const g = side.greeks ?? {};
  const iv = parseFloat(g.iv ?? "0") || 0;
  const delta = parseFloat(g.delta ?? "0") || 0;
  const gamma = parseFloat(g.gamma ?? "0") || 0;
  const theta = parseFloat(g.theta ?? "0") || 0;
  const vega = parseFloat(g.vega ?? "0") || 0;
  const mark = parseFloat(side.mark_price ?? "0") || 0;
  const bid = parseFloat(side.best_bid_price ?? "0") || 0;
  const ask = parseFloat(side.best_ask_price ?? "0") || 0;
  const oi = parseFloat(side.oi ?? "0") || 0;
  const volume = parseFloat(side.volume ?? "0") || 0;
  const isItm = isCall ? strike < spot : strike > spot;
  return { iv, delta, gamma, theta, vega, mark, bid, ask, oi, volume, isItm };
}

// ── Handler ───────────────────────────────────────────────────────────────────

export async function GET(request: Request) {
  if (!isDeltaConfigured()) {
    return NextResponse.json({
      ok: false,
      error: "Set DELTA_API_KEY and DELTA_API_SECRET in Vercel env vars",
    });
  }

  const { searchParams } = new URL(request.url);
  const expiryParam = searchParams.get("expiry");

  // Compute next 4 Fridays for expiry list
  const fridays = nextNFridays(4);

  // Determine selected expiry
  let selectedExpiry: string;
  if (expiryParam) {
    selectedExpiry = expiryParam;
  } else {
    selectedExpiry = toDateStr(fridays[0]);
  }

  // Build expiry metadata list
  const expiries: ExpiryMeta[] = fridays.map((f) => ({
    label: expiryLabel(f),
    value: toDateStr(f),
    dte: dte(f),
  }));

  // If selectedExpiry isn't one of the 4 Fridays, find the nearest match or use as-is
  const selectedDte = expiries.find((e) => e.value === selectedExpiry)?.dte ?? 0;
  const selectedLabel = expiries.find((e) => e.value === selectedExpiry)?.label ?? selectedExpiry;

  try {
    // 1. Fetch spot price from BTCUSD ticker
    const tickerPath = "/v2/tickers/BTCUSD";
    const tickerRes = await fetch(`${DELTA_BASE}${tickerPath}`, {
      headers: deltaHeaders("GET", tickerPath),
      next: { revalidate: 0 },
    });

    let spot = 0;
    if (tickerRes.ok) {
      const tickerJson = await tickerRes.json() as DeltaTickerResponse;
      if (tickerJson.success && tickerJson.result) {
        spot = parseFloat(tickerJson.result.mark_price ?? tickerJson.result.spot_price ?? tickerJson.result.close ?? "0") || 0;
      }
    }

    // 2. Fetch option chain for selected expiry
    const chainPath = `/v2/options/chain?underlying_asset_symbol=BTC&expiry_date=${encodeURIComponent(selectedExpiry)}`;
    const chainRes = await fetch(`${DELTA_BASE}${chainPath}`, {
      headers: deltaHeaders("GET", chainPath),
      next: { revalidate: 0 },
    });

    if (!chainRes.ok) {
      return NextResponse.json({ ok: false, error: `Delta Exchange returned ${chainRes.status}` });
    }

    const chainJson = await chainRes.json() as DeltaChainResponse;

    if (!chainJson.success || !Array.isArray(chainJson.result)) {
      const errMsg = chainJson.error?.context ?? chainJson.error?.code ?? "Option chain fetch failed";
      return NextResponse.json({ ok: false, error: errMsg });
    }

    // 3. Find ATM strike
    let atmStrike = 0;
    if (spot > 0 && chainJson.result.length > 0) {
      let minDist = Infinity;
      for (const row of chainJson.result) {
        const dist = Math.abs(row.strike_price - spot);
        if (dist < minDist) {
          minDist = dist;
          atmStrike = row.strike_price;
        }
      }
    }

    // 4. Compute baseIv from ATM call/put average
    let baseIv = 0;
    if (atmStrike > 0) {
      const atmRow = chainJson.result.find((r) => r.strike_price === atmStrike);
      if (atmRow) {
        const callIv = parseFloat(atmRow.call_options?.greeks?.iv ?? "0") || 0;
        const putIv = parseFloat(atmRow.put_options?.greeks?.iv ?? "0") || 0;
        const count = (callIv > 0 ? 1 : 0) + (putIv > 0 ? 1 : 0);
        if (count > 0) baseIv = (callIv + putIv) / count;
      }
    }

    // 5. Map rows
    const chain: ChainRow[] = chainJson.result.map((row) => {
      const strike = row.strike_price;
      const isAtm = strike === atmStrike;
      const moneynessPC = spot > 0 ? (strike - spot) / spot * 100 : 0;
      const call = mapLeg(row.call_options, spot, strike, true);
      const put = mapLeg(row.put_options, spot, strike, false);
      return { strike, isAtm, moneynessPC, call, put };
    });

    // Sort by strike ascending
    chain.sort((a, b) => a.strike - b.strike);

    const responseData: ChainData & { source: string; ok: boolean } = {
      underlyingPrice: spot,
      baseIv,
      expiries,
      selectedExpiry,
      expiryLabel: selectedLabel,
      dte: selectedDte,
      chain,
      source: "delta_exchange",
      ok: true,
    };

    return NextResponse.json(responseData);
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, error: message });
  }
}
