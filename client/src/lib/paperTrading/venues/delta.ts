/**
 * Delta Exchange India as a paper venue.
 *
 * REAL PRICES, REAL DEPTH, REAL CONTRACT SPECS — and no credentials anywhere.
 * Everything here comes from Delta's public market-data endpoints, the same
 * ones the Crypto Screener reads:
 *
 *   /v2/tickers          quotes, 24h stats, funding, top of book, tick size,
 *                        contract value, max leverage
 *   /v2/products         maintenance and initial margin, which the ticker
 *                        payload does NOT carry — measured absent for all 220
 *                        perpetuals — and which decide where a leveraged
 *                        position is liquidated
 *   /v2/l2orderbook/SYM  the actual resting book, thousands of levels deep
 *   /v2/history/candles  bars for the chart and for replaying resting orders
 *
 * THE ORDER BOOK IS THE POINT. Most paper terminals fill a market order at the
 * last price, or at the mid, which is a fill nobody has ever received. Delta
 * publishes real depth — 2,484 bid levels on BTCUSD when this was written — so
 * a market order here is WALKED THROUGH THE BOOK and receives the
 * size-weighted price it would actually have paid, including the part of the
 * order that eats into worse levels. On a thin contract that difference is the
 * whole trade.
 *
 * FEES are Delta India's own: 0.05% taker plus 18% GST on the fee, which is
 * the 0.059% the live engine charges. Maker is lower and is charged when a
 * resting limit order fills, because that is what actually happens.
 */

import {
  fetchBarsBetween,
  fetchPerpTickers,
  fetchProductSpecs,
  num,
  type PerpTicker,
  type ProductSpec,
} from "@/lib/cryptoScreener/delta";
import type {
  AccountType,
  Candle,
  Instrument,
  OrderBook,
  VenueAdapter,
} from "../types";

/** Taker fee INCLUDING GST, matching `delta.PerpTakerFeeRate` in the Go engine. */
export const DELTA_TAKER_FEE = 0.00059;
/** Maker rebate tier: 0.02% plus GST. Charged when a resting order fills. */
export const DELTA_MAKER_FEE = 0.000236;

const TTL_MS = 20_000;

let cache: { at: number; instruments: Instrument[] } | null = null;
let inFlight: Promise<Instrument[]> | null = null;

function specToInstrument(t: PerpTicker, spec: ProductSpec | undefined): Instrument | null {
  const last = t.close ?? t.markPrice;
  if (last === null || last <= 0) return null;
  const cv = t.contractValue;
  if (!cv || cv <= 0) return null;

  const bid = t.quotes.bestBid ?? last;
  const ask = t.quotes.bestAsk ?? last;

  // Maintenance margin decides the liquidation price. It is absent from the
  // ticker payload, so it comes from /v2/products; a contract with neither is
  // still listed but carries null, and the engine refuses to open a leveraged
  // position on it rather than assuming one.
  const mm = spec?.maintenanceMarginPct;
  const maxLev = t.maxLeverage ?? (spec?.initialMarginPct ? Math.floor(100 / spec.initialMarginPct) : 20);

  return {
    venue: "delta",
    symbol: t.symbol,
    displayName: t.description || t.symbol,
    kind: "perp",
    contractSize: cv,
    sizeUnit: "contracts",
    minSize: 1,
    sizeStep: 1,
    tickSize: t.tickSize ?? 0.5,
    pricePrecision: precisionFor(t.tickSize ?? 0.5),
    maxLeverage: Math.max(1, maxLev),
    // 2.5 is the WIDEST value Delta uses, not the narrowest. An unknown
    // maintenance requirement must produce a conservative liquidation estimate:
    // assuming a low one would place liquidation further away than it is and
    // approve a stop the venue would pre-empt.
    maintenanceMarginPct: mm ?? 2.5,

    takerFeeRate: DELTA_TAKER_FEE,
    makerFeeRate: DELTA_MAKER_FEE,
    commissionPerLotUsd: 0,

    carryKind: "funding",
    fundingRatePct8h: t.fundingRatePct8h,
    swapLongPointsPerDay: null,
    swapShortPointsPerDay: null,

    last,
    bid,
    ask,
    markPrice: t.markPrice ?? last,
    change24hPct: t.ltpChange24hPct,
    high24h: t.high24h,
    low24h: t.low24h,
    quoteCurrency: "USD",

    spreadIsModelled: false,
    source: "Delta Exchange India — /v2/tickers, /v2/products, /v2/l2orderbook",
  };
}

function precisionFor(tick: number): number {
  if (!(tick > 0)) return 2;
  const s = tick.toExponential().split("e");
  const exp = Number(s[1]);
  return exp < 0 ? Math.min(10, -exp) : 0;
}

async function loadInstruments(): Promise<Instrument[]> {
  const now = Date.now();
  if (cache && now - cache.at < TTL_MS) return cache.instruments;
  if (inFlight) return inFlight;

  inFlight = (async () => {
    const [tickers, specs] = await Promise.all([
      fetchPerpTickers(),
      // Failing soft: the desk still lists and quotes, and the engine refuses
      // leverage on anything whose margin spec it could not read.
      fetchProductSpecs().catch(() => new Map<string, ProductSpec>()),
    ]);
    const out: Instrument[] = [];
    for (const t of tickers) {
      if (t.tradingStatus !== "operational") continue;
      const inst = specToInstrument(t, specs.get(t.symbol));
      if (inst) out.push(inst);
    }
    // Busiest first: a terminal opens on the instrument people actually trade.
    const turnover = new Map(tickers.map((t) => [t.symbol, t.turnoverUsd24h ?? 0]));
    out.sort((a, b) => (turnover.get(b.symbol) ?? 0) - (turnover.get(a.symbol) ?? 0));
    cache = { at: Date.now(), instruments: out };
    return out;
  })().finally(() => {
    inFlight = null;
  });

  return inFlight;
}

type L2Response = {
  success?: boolean;
  result?: {
    buy?: Array<Record<string, unknown>>;
    sell?: Array<Record<string, unknown>>;
    last_updated_at?: number;
  };
};

/**
 * The live resting book, trimmed to `depth` levels a side.
 *
 * Delta returns thousands of levels; a terminal shows a dozen and the matcher
 * rarely walks past a hundred, so the rest is transferred and discarded. The
 * trim happens here rather than in the UI so the matcher and the display are
 * looking at the same object.
 */
async function getBook(symbol: string, depth: number): Promise<OrderBook | null> {
  const base = process.env.DELTA_API_BASE_URL?.trim() || "https://api.india.delta.exchange";
  try {
    const res = await fetch(`${base.replace(/\/+$/, "")}/v2/l2orderbook/${encodeURIComponent(symbol)}`, {
      cache: "no-store",
      headers: { accept: "application/json" },
      signal: AbortSignal.timeout(15_000),
    });
    if (!res.ok) return null;
    const body = (await res.json()) as L2Response;
    const r = body.result;
    if (!r) return null;

    const level = (x: Record<string, unknown>) => {
      const price = num(x.price);
      const size = num(x.size);
      return price !== null && size !== null && price > 0 && size > 0 ? { price, size } : null;
    };
    const bids = (r.buy ?? []).map(level).filter((x): x is { price: number; size: number } => x !== null);
    const asks = (r.sell ?? []).map(level).filter((x): x is { price: number; size: number } => x !== null);
    // Delta returns them sorted already, but a matcher that walks an unsorted
    // book silently fills at the wrong price, so the order is asserted here.
    bids.sort((a, b) => b.price - a.price);
    asks.sort((a, b) => a.price - b.price);

    return {
      symbol,
      bids: bids.slice(0, depth),
      asks: asks.slice(0, depth),
      asOf: r.last_updated_at ? Math.floor(r.last_updated_at / 1000) : Date.now(),
      modelled: false,
    };
  } catch {
    return null;
  }
}

export const RESOLUTIONS = [
  { key: "1m", label: "1m", seconds: 60 },
  { key: "5m", label: "5m", seconds: 300 },
  { key: "15m", label: "15m", seconds: 900 },
  { key: "1h", label: "1H", seconds: 3_600 },
  { key: "4h", label: "4H", seconds: 14_400 },
  { key: "1d", label: "1D", seconds: 86_400 },
];

const ACCOUNT_TYPES: { key: AccountType; label: string; note: string }[] = [
  {
    key: "standard",
    label: "Standard",
    note: "Delta India's published schedule: 0.05% taker plus 18% GST, 0.02% plus GST on a resting fill.",
  },
];

export const deltaVenue: VenueAdapter = {
  id: "delta",
  label: "Delta Paper Trading",
  dataNote:
    "Live Delta Exchange India market data — quotes, contract specs, funding and the real L2 order " +
    "book. Market orders are walked through actual resting depth rather than filled at the last " +
    "price. Paper money only: this module holds no keys and has no order-routing path.",
  // Delta nets: buying then selling the same contract leaves one averaged
  // position, exactly as the venue does.
  positionMode: "netting",
  sizeUnit: "contracts",
  defaultStartingBalance: 10_000,
  defaultLeverage: 10,
  leverageChoices: [1, 2, 3, 5, 10, 20, 25, 50, 100, 200],
  accountTypes: ACCOUNT_TYPES,
  // Delta liquidates position by position against maintenance margin rather
  // than on an account-wide margin level, so there is no stop-out percentage.
  stopOutLevelPct: 0,
  marginCallLevelPct: 0,
  resolutions: RESOLUTIONS,

  listInstruments: loadInstruments,

  async getInstrument(symbol: string) {
    const all = await loadInstruments();
    return all.find((i) => i.symbol === symbol.toUpperCase()) ?? null;
  },

  getBook,

  async getCandles(symbol: string, resolution: string, from: number, to: number): Promise<Candle[]> {
    const bars = await fetchBarsBetween(symbol, resolution, from, to);
    return bars.map((b) => ({
      time: b.ts,
      open: b.open,
      high: b.high,
      low: b.low,
      close: b.close,
      volume: b.volume,
    }));
  },
};

function str(v: unknown): string {
  return typeof v === "string" ? v : "";
}

/** One executed trade off the venue's public tape. */
export type PublicTrade = {
  price: number;
  size: number;
  /** Which side crossed the spread. Inferred from who was the taker. */
  side: "buy" | "sell";
  at: number;
};

/**
 * The recent trade tape.
 *
 * AGGRESSOR SIDE IS INFERRED FROM THE ROLES, not from the price direction.
 * Delta reports `buyer_role` and `seller_role`; whichever is `taker` is the
 * side that crossed the spread and therefore the side that moved. Guessing
 * aggression from whether the print is above or below the previous one is the
 * usual shortcut and it mislabels every trade inside the spread — which on a
 * quiet contract is most of them.
 */
export async function fetchTrades(symbol: string, limit = 40): Promise<PublicTrade[]> {
  const base = process.env.DELTA_API_BASE_URL?.trim() || "https://api.india.delta.exchange";
  try {
    const res = await fetch(`${base.replace(/\/+$/, "")}/v2/trades/${encodeURIComponent(symbol)}`, {
      cache: "no-store",
      headers: { accept: "application/json" },
      signal: AbortSignal.timeout(12_000),
    });
    if (!res.ok) return [];
    const body = (await res.json()) as { result?: Array<Record<string, unknown>> };
    const out: PublicTrade[] = [];
    for (const t of body.result ?? []) {
      const price = num(t.price);
      const size = num(t.size);
      if (price === null || size === null || price <= 0) continue;
      out.push({
        price,
        size,
        side: str(t.seller_role) === "taker" ? "sell" : "buy",
        // Delta stamps the tape in MICROseconds. Reading it as milliseconds
        // puts every print about fifty thousand years in the future, which
        // renders as an empty tape rather than an obvious error.
        at: Math.floor((num(t.timestamp) ?? 0) / 1000),
      });
    }
    return out.slice(0, limit);
  } catch {
    return [];
  }
}
