import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, angelMissingEnv, commonHeaders, BASE_URL } from "@/lib/angelAuth";
import type { ChainData, ChainRow, ChainLeg, ExpiryMeta } from "@/hooks/useOptionChain";

// Angel One option chain item shape
type AngelLeg = {
  strikePrice?: unknown;
  impliedVolatility?: unknown;
  openInterest?: unknown;
  changeinOpenInterest?: unknown;
  totalTradedVolume?: unknown;
  lastPrice?: unknown;
  bidPrice?: unknown;
  askPrice?: unknown;
  delta?: unknown;
  gamma?: unknown;
  theta?: unknown;
  vega?: unknown;
};

type AngelChainItem = {
  strikePrice?: unknown;
  PE?: AngelLeg | null;
  CE?: AngelLeg | null;
};

function n(v: unknown): number {
  const num = Number(v);
  return Number.isFinite(num) ? num : 0;
}

/** Format a Date as "DDMMMYYYY" e.g. "10APR2025" */
function formatExpiryDate(date: Date): string {
  const months = ["JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"];
  const dd = String(date.getUTCDate()).padStart(2, "0");
  const mmm = months[date.getUTCMonth()];
  const yyyy = date.getUTCFullYear();
  return `${dd}${mmm}${yyyy}`;
}

/** Return the next Thursday on or after 'from' (UTC) */
function nextThursday(from: Date): Date {
  const d = new Date(from);
  d.setUTCHours(0, 0, 0, 0);
  // Thursday = day 4
  const dow = d.getUTCDay();
  const daysAhead = dow <= 4 ? 4 - dow : 7 - dow + 4;
  d.setUTCDate(d.getUTCDate() + daysAhead);
  return d;
}

/** Compute next N NIFTY weekly expiries from today */
function computeExpiries(n: number): ExpiryMeta[] {
  const now = new Date();
  const expiries: ExpiryMeta[] = [];
  let base = new Date(now);
  for (let i = 0; i < n; i++) {
    const thursday = nextThursday(base);
    const label = formatExpiryDate(thursday);
    const dte = Math.ceil((thursday.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
    expiries.push({ label, value: label, dte: Math.max(0, dte) });
    // Advance base to the day after this Thursday
    base = new Date(thursday);
    base.setUTCDate(base.getUTCDate() + 1);
  }
  return expiries;
}

function buildLeg(leg: AngelLeg | null | undefined, spotPrice: number, isItm: boolean): ChainLeg {
  if (!leg) {
    return { iv: 0, delta: 0, gamma: 0, theta: 0, vega: 0, mark: 0, bid: 0, ask: 0, oi: 0, volume: 0, isItm };
  }
  return {
    iv: n(leg.impliedVolatility),
    delta: n(leg.delta),
    gamma: n(leg.gamma),
    theta: n(leg.theta),
    vega: n(leg.vega),
    mark: n(leg.lastPrice),
    bid: n(leg.bidPrice),
    ask: n(leg.askPrice),
    oi: n(leg.openInterest),
    volume: n(leg.totalTradedVolume),
    isItm,
  };
  void spotPrice; // suppress unused
}

export async function GET(request: Request) {
  if (!isAngelConfigured()) {
    return NextResponse.json({
      ok: false,
      error: `Not configured — set these env vars: ${angelMissingEnv()}`,
      source: "error",
    });
  }

  const { searchParams } = new URL(request.url);
  const expiries = computeExpiries(4);
  const expiry = searchParams.get("expiry") || expiries[0]?.value || "";

  try {
    const jwt = await getAngelJWT();

    // Fetch option chain
    const chainRes = await fetch(`${BASE_URL}/rest/secure/angelbroking/market/v1/optionChain`, {
      method: "POST",
      headers: commonHeaders(jwt),
      body: JSON.stringify({ name: "NIFTY", expirydate: expiry }),
      next: { revalidate: 0 },
    });

    if (!chainRes.ok) {
      return NextResponse.json({ ok: false, error: `Angel One option chain returned ${chainRes.status}`, source: "error" });
    }

    const chainPayload = await chainRes.json() as {
      status?: boolean;
      message?: string;
      errorcode?: string;
      data?: AngelChainItem[];
    };

    if (!chainPayload.status || !Array.isArray(chainPayload.data)) {
      const msg = [chainPayload.message, chainPayload.errorcode].filter(Boolean).join(" ") || "Option chain fetch failed";
      return NextResponse.json({ ok: false, error: msg, source: "error" });
    }

    // Fetch spot price
    const spotRes = await fetch(`${BASE_URL}/rest/secure/angelbroking/order/v1/getLtpData`, {
      method: "POST",
      headers: commonHeaders(jwt),
      body: JSON.stringify({ exchange: "NSE", tradingsymbol: "NIFTY 50", symboltoken: "99926000" }),
      next: { revalidate: 0 },
    });

    let spotPrice = 0;
    if (spotRes.ok) {
      const spotPayload = await spotRes.json() as { status?: boolean; data?: { ltp?: unknown } };
      if (spotPayload.status) {
        spotPrice = n(spotPayload.data?.ltp);
      }
    }

    // Build chain rows
    const rawItems = chainPayload.data as AngelChainItem[];
    const sorted = [...rawItems].sort((a, b) => n(a.strikePrice) - n(b.strikePrice));

    // Find ATM: closest strike to spot
    const atmStrike = sorted.reduce((best, item) => {
      const s = n(item.strikePrice);
      const bestS = n(best.strikePrice);
      return Math.abs(s - spotPrice) < Math.abs(bestS - spotPrice) ? item : best;
    }, sorted[0]);
    const atmStrikePrice = n(atmStrike?.strikePrice ?? 0);

    const chain: ChainRow[] = sorted.map((item) => {
      const strike = n(item.strikePrice);
      const isAtm = strike === atmStrikePrice;
      const moneynessPC = spotPrice > 0 ? (strike - spotPrice) / spotPrice * 100 : 0;
      const callItm = strike < spotPrice;
      const putItm = strike > spotPrice;
      return {
        strike,
        isAtm,
        moneynessPC,
        call: buildLeg(item.CE, spotPrice, callItm),
        put: buildLeg(item.PE, spotPrice, putItm),
      };
    });

    // ATM IV average
    const atmRow = chain.find((r) => r.isAtm);
    const baseIv = atmRow ? (atmRow.call.iv + atmRow.put.iv) / 2 : 0;

    // Find DTE for selected expiry
    const selectedMeta = expiries.find((e) => e.value === expiry) ?? expiries[0];
    const dte = selectedMeta?.dte ?? 0;
    const expiryLabel = selectedMeta?.label ?? expiry;

    const responseData: ChainData & { source: string } = {
      underlyingPrice: spotPrice,
      baseIv,
      expiries,
      selectedExpiry: expiry,
      expiryLabel,
      dte,
      chain,
      source: "angel_one",
    };

    return NextResponse.json(responseData);
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, error: message, source: "error" });
  }
}
