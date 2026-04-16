import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, commonHeaders, BASE_URL } from "@/lib/angelAuth";
import type { ChainData, ChainRow, ChainLeg, ExpiryMeta } from "@/hooks/useOptionChain";

// ─── Angel One types ──────────────────────────────────────────────────────────

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

// ─── Date helpers ─────────────────────────────────────────────────────────────

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
  const dow = d.getUTCDay();
  const daysAhead = dow <= 4 ? 4 - dow : 7 - dow + 4;
  d.setUTCDate(d.getUTCDate() + daysAhead);
  return d;
}

/** Compute next N NIFTY weekly expiries from today */
function computeExpiries(count: number): ExpiryMeta[] {
  const now = new Date();
  const expiries: ExpiryMeta[] = [];
  let base = new Date(now);
  for (let i = 0; i < count; i++) {
    const thursday = nextThursday(base);
    const label = formatExpiryDate(thursday);
    const dte = Math.max(0, Math.ceil((thursday.getTime() - now.getTime()) / (1000 * 60 * 60 * 24)));
    expiries.push({ label, value: label, dte });
    base = new Date(thursday);
    base.setUTCDate(base.getUTCDate() + 1);
  }
  return expiries;
}

// ─── Angel One helpers ────────────────────────────────────────────────────────

function buildLeg(leg: AngelLeg | null | undefined, spotPrice: number, isItm: boolean): ChainLeg {
  void spotPrice;
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
}

/** Check that the response has a JSON content-type before calling .json() */
async function safeJson<T>(res: Response, label: string): Promise<T> {
  const ct = res.headers.get("content-type") ?? "";
  if (!ct.includes("application/json") && !ct.includes("text/json")) {
    const preview = (await res.text()).slice(0, 120);
    throw new Error(`${label} returned non-JSON (${res.status}): ${preview}`);
  }
  return res.json() as Promise<T>;
}

// ─── Synthetic option chain (Black-Scholes, no credentials needed) ────────────

const SQRT2PI = Math.sqrt(2 * Math.PI);

function normCDF(x: number): number {
  return 0.5 * (1 + erf(x / Math.SQRT2));
}

function erf(x: number): number {
  // Abramowitz & Stegun approximation
  const t = 1 / (1 + 0.3275911 * Math.abs(x));
  const poly = t * (0.254829592 + t * (-0.284496736 + t * (1.421413741 + t * (-1.453152027 + t * 1.061405429))));
  const result = 1 - poly * Math.exp(-x * x);
  return x >= 0 ? result : -result;
}

function normPDF(x: number): number {
  return Math.exp(-0.5 * x * x) / SQRT2PI;
}

function bsPrice(spot: number, strike: number, T: number, iv: number, isCall: boolean): { mark: number; delta: number; gamma: number; theta: number; vega: number } {
  if (T <= 0 || iv <= 0 || spot <= 0 || strike <= 0) {
    const intrinsic = isCall ? Math.max(spot - strike, 0) : Math.max(strike - spot, 0);
    return { mark: intrinsic, delta: isCall ? (spot > strike ? 1 : 0) : (spot < strike ? -1 : 0), gamma: 0, theta: 0, vega: 0 };
  }
  const r = 0.065; // Indian risk-free rate ~6.5%
  const sqrtT = Math.sqrt(T);
  const d1 = (Math.log(spot / strike) + (r + iv * iv / 2) * T) / (iv * sqrtT);
  const d2 = d1 - iv * sqrtT;
  let mark: number, delta: number;
  if (isCall) {
    mark = spot * normCDF(d1) - strike * Math.exp(-r * T) * normCDF(d2);
    delta = normCDF(d1);
  } else {
    mark = strike * Math.exp(-r * T) * normCDF(-d2) - spot * normCDF(-d1);
    delta = normCDF(d1) - 1;
  }
  const gamma = normPDF(d1) / (spot * iv * sqrtT);
  const vega = spot * normPDF(d1) * sqrtT / 100;
  const theta = isCall
    ? -(spot * normPDF(d1) * iv / (2 * sqrtT) + r * strike * Math.exp(-r * T) * normCDF(d2)) / 365
    : -(spot * normPDF(d1) * iv / (2 * sqrtT) - r * strike * Math.exp(-r * T) * normCDF(-d2)) / 365;
  return { mark: Math.max(0, mark), delta, gamma, theta, vega };
}

/** Fetch NIFTY 50 spot price from Yahoo Finance — no credentials needed */
async function fetchNiftySpotYahoo(): Promise<number> {
  const url = "https://query1.finance.yahoo.com/v8/finance/chart/%5ENSEI?interval=1m&range=1d&includePrePost=false";
  const res = await fetch(url, {
    headers: {
      "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
      Accept: "application/json",
    },
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Yahoo NIFTY returned ${res.status}`);
  const data = await res.json() as { chart?: { result?: Array<{ meta?: { regularMarketPrice?: number } }> } };
  const price = data?.chart?.result?.[0]?.meta?.regularMarketPrice ?? 0;
  if (!price) throw new Error("Yahoo NIFTY price unavailable");
  return price;
}

/** Build a synthetic chain using Black-Scholes from Yahoo spot + India VIX */
async function buildSyntheticChain(expiry: string, expiries: ExpiryMeta[]): Promise<ChainData & { source: string }> {
  const spot = await fetchNiftySpotYahoo();

  // Derive IV from India VIX if available, otherwise use 15%
  let iv = 0.15;
  try {
    const vixRes = await fetch("https://query1.finance.yahoo.com/v8/finance/chart/%5EINDIAVIX?interval=1m&range=1d", {
      headers: { "User-Agent": "Mozilla/5.0", Accept: "application/json" },
      cache: "no-store",
    });
    if (vixRes.ok) {
      const vd = await vixRes.json() as { chart?: { result?: Array<{ meta?: { regularMarketPrice?: number } }> } };
      const vix = vd?.chart?.result?.[0]?.meta?.regularMarketPrice ?? 0;
      if (vix > 0) iv = vix / 100; // VIX is in %, convert to decimal
    }
  } catch { /* use default */ }

  const selectedMeta = expiries.find((e) => e.value === expiry) ?? expiries[0];
  const dte = selectedMeta?.dte ?? 7;
  const T = dte / 365;

  // NIFTY strikes in 50pt increments, ATM ± 15 strikes
  const atmRounded = Math.round(spot / 50) * 50;
  const strikes: number[] = [];
  for (let i = -15; i <= 15; i++) {
    strikes.push(atmRounded + i * 50);
  }

  const chain: ChainRow[] = strikes.map((strike) => {
    const isAtm = strike === atmRounded;
    const moneynessPC = ((strike - spot) / spot) * 100;
    const callItm = strike < spot;
    const putItm = strike > spot;

    const callBs = bsPrice(spot, strike, T, iv, true);
    const putBs = bsPrice(spot, strike, T, iv, false);

    const call: ChainLeg = {
      iv: iv * 100,
      delta: callBs.delta,
      gamma: callBs.gamma,
      theta: callBs.theta,
      vega: callBs.vega,
      mark: Math.round(callBs.mark * 20) / 20, // 0.05 tick rounding
      bid: Math.round((callBs.mark * 0.995) * 20) / 20,
      ask: Math.round((callBs.mark * 1.005) * 20) / 20,
      oi: 0,
      volume: 0,
      isItm: callItm,
    };
    const put: ChainLeg = {
      iv: iv * 100,
      delta: putBs.delta,
      gamma: putBs.gamma,
      theta: putBs.theta,
      vega: putBs.vega,
      mark: Math.round(putBs.mark * 20) / 20,
      bid: Math.round((putBs.mark * 0.995) * 20) / 20,
      ask: Math.round((putBs.mark * 1.005) * 20) / 20,
      oi: 0,
      volume: 0,
      isItm: putItm,
    };

    return { strike, isAtm, moneynessPC, call, put };
  });

  const atmCallIv = chain.find((r) => r.isAtm)?.call.iv ?? 0;
  const atmPutIv = chain.find((r) => r.isAtm)?.put.iv ?? 0;

  return {
    underlyingPrice: spot,
    baseIv: (atmCallIv + atmPutIv) / 2,
    expiries,
    selectedExpiry: selectedMeta?.value ?? expiry,
    expiryLabel: selectedMeta?.label ?? expiry,
    dte,
    chain,
    source: "synthetic_bs",
  };
}

// ─── Route handler ────────────────────────────────────────────────────────────

export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const expiries = computeExpiries(4);
  const expiry = searchParams.get("expiry") || expiries[0]?.value || "";

  // ── Try Angel One first if credentials are configured ─────────────────────
  if (isAngelConfigured()) {
    try {
      const jwt = await getAngelJWT();

      const chainRes = await fetch(`${BASE_URL}/rest/secure/angelbroking/market/v1/optionChain`, {
        method: "POST",
        headers: commonHeaders(jwt),
        body: JSON.stringify({ name: "NIFTY", expirydate: expiry }),
        next: { revalidate: 0 },
      });

      if (!chainRes.ok) {
        throw new Error(`Angel One option chain returned HTTP ${chainRes.status}`);
      }

      const chainPayload = await safeJson<{
        status?: boolean;
        message?: string;
        errorcode?: string;
        data?: AngelChainItem[];
      }>(chainRes, "Angel One option chain");

      if (!chainPayload.status || !Array.isArray(chainPayload.data)) {
        const msg = [chainPayload.message, chainPayload.errorcode].filter(Boolean).join(" ") || "Option chain fetch failed";
        throw new Error(msg);
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
        try {
          const spotPayload = await safeJson<{ status?: boolean; data?: { ltp?: unknown } }>(spotRes, "Angel One LTP");
          if (spotPayload.status) spotPrice = n(spotPayload.data?.ltp);
        } catch { /* spot unavailable, keep 0 */ }
      }

      const sorted = [...chainPayload.data].sort((a, b) => n(a.strikePrice) - n(b.strikePrice));
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
        return {
          strike,
          isAtm,
          moneynessPC,
          call: buildLeg(item.CE, spotPrice, strike < spotPrice),
          put: buildLeg(item.PE, spotPrice, strike > spotPrice),
        };
      });

      const atmRow = chain.find((r) => r.isAtm);
      const baseIv = atmRow ? (atmRow.call.iv + atmRow.put.iv) / 2 : 0;
      const selectedMeta = expiries.find((e) => e.value === expiry) ?? expiries[0];

      return NextResponse.json({
        underlyingPrice: spotPrice,
        baseIv,
        expiries,
        selectedExpiry: expiry,
        expiryLabel: selectedMeta?.label ?? expiry,
        dte: selectedMeta?.dte ?? 0,
        chain,
        source: "angel_one",
      } satisfies ChainData & { source: string });

    } catch (angelErr) {
      // Angel One failed — log and fall through to synthetic chain
      const msg = angelErr instanceof Error ? angelErr.message : String(angelErr);
      console.warn("[nifty/option-chain] Angel One failed, using synthetic chain:", msg);
    }
  }

  // ── Synthetic fallback: Black-Scholes from Yahoo Finance ──────────────────
  try {
    const synth = await buildSyntheticChain(expiry, expiries);
    return NextResponse.json(synth);
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json(
      { ok: false, error: `Option chain unavailable: ${message}. Configure ANGELONE_* env vars for live data.`, source: "error" },
    );
  }
}
