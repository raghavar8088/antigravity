/**
 * The venue adapter: Delta Exchange India's public market data.
 *
 * ONE SNAPSHOT, MANY READERS. Every tab on this page — the chain, the
 * perpetuals list, top movers, and the mark used to value open positions —
 * reads the SAME cached snapshot. Building it costs two paginated product
 * calls and one ticker call; letting each tab fetch its own would multiply that
 * by five and still show five slightly different prices on one screen, which is
 * worse than being a few seconds stale.
 *
 * STALE BEATS EMPTY. If a refresh fails the previous snapshot is served with
 * its original timestamp rather than an error. A desk that blanks its chain
 * because one poll timed out is unusable, and the timestamp is what tells the
 * reader how old the numbers are.
 *
 * No key is held here and no authenticated endpoint is called. Everything below
 * is public market data.
 */

import type { Greeks, Instrument, OptionChain, OptionType, TopMover } from "./types";
import { displayNameOf } from "./types";

const BASE = process.env.DELTA_API_BASE?.trim() || "https://api.india.delta.exchange";

/** Underlyings Delta lists options on. Discovered, never hardcoded. */
export type Underlying = { symbol: string; spot: number | null; optionCount: number };

type Snapshot = {
  bySymbol: Map<string, Instrument>;
  options: Instrument[];
  perpetuals: Instrument[];
  underlyings: Underlying[];
  builtAt: number;
};

const TTL_MS = 20_000;
let cached: Snapshot | null = null;
let inFlight: Promise<Snapshot> | null = null;

/** The current snapshot, rebuilt at most once per TTL. */
export async function getSnapshot(force = false): Promise<Snapshot> {
  if (!force && cached && Date.now() - cached.builtAt < TTL_MS) return cached;
  if (inFlight) return inFlight;
  inFlight = build()
    .then((s) => {
      cached = s;
      return s;
    })
    .catch((e) => {
      // Stale beats empty — see the module docstring.
      if (cached) return cached;
      throw e;
    })
    .finally(() => {
      inFlight = null;
    });
  return inFlight;
}

async function getJson(url: string): Promise<Record<string, unknown>> {
  const r = await fetch(url, {
    headers: { Accept: "application/json" },
    cache: "no-store",
    signal: AbortSignal.timeout(25_000),
  });
  if (!r.ok) throw new Error(`Delta ${r.status} for ${url.replace(BASE, "")}`);
  return (await r.json()) as Record<string, unknown>;
}

type RawProduct = {
  symbol?: string;
  contract_type?: string;
  strike_price?: string | number | null;
  settlement_time?: string | null;
  contract_value?: string | number | null;
  tick_size?: string | number | null;
  underlying_asset?: { symbol?: string } | null;
  initial_margin?: string | number | null;
  maintenance_margin?: string | number | null;
  initial_margin_scaling_factor?: string | number | null;
  maintenance_margin_scaling_factor?: string | number | null;
  default_leverage?: string | number | null;
  liquidation_penalty_factor?: string | number | null;
};

/**
 * Every product, following Delta's cursor.
 *
 * The catalogue is larger than one page: a single page_size=1000 call returns
 * exactly 1000 and silently drops the rest, which would leave whole expiries
 * missing from the chain with nothing to indicate they existed.
 */
async function allProducts(): Promise<RawProduct[]> {
  const out: RawProduct[] = [];
  let after: string | null = null;
  for (let page = 0; page < 12; page++) {
    const url = `${BASE}/v2/products?page_size=1000${after ? `&after=${encodeURIComponent(after)}` : ""}`;
    const body = await getJson(url);
    const rows = (body.result as RawProduct[] | undefined) ?? [];
    out.push(...rows);
    const meta = body.meta as { after?: string | null } | undefined;
    after = meta?.after ?? null;
    if (!after || rows.length === 0) break;
  }
  return out;
}

type RawTicker = {
  symbol?: string;
  mark_price?: string | number | null;
  spot_price?: string | number | null;
  oi?: string | number | null;
  turnover_usd?: string | number | null;
  mark_vol?: string | number | null;
  funding_rate?: string | number | null;
  greeks?: Record<string, string | number> | null;
  quotes?: { best_bid?: string | number | null; best_ask?: string | number | null } | null;
  close?: string | number | null;
  open?: string | number | null;
};

function num(v: unknown): number | null {
  if (v === null || v === undefined || v === "") return null;
  const n = typeof v === "number" ? v : Number(v);
  return Number.isFinite(n) ? n : null;
}

function greeksOf(g: Record<string, string | number> | null | undefined): Greeks | null {
  if (!g) return null;
  const d = num(g.delta);
  if (d === null) return null;
  return {
    delta: d,
    gamma: num(g.gamma) ?? 0,
    theta: num(g.theta) ?? 0,
    vega: num(g.vega) ?? 0,
    rho: num(g.rho) ?? 0,
  };
}

/** "2026-09-26T12:00:00Z" -> "2026-09-26". */
function isoDate(t: string | null | undefined): string | null {
  if (!t) return null;
  const d = new Date(t);
  return Number.isNaN(d.getTime()) ? null : d.toISOString().slice(0, 10);
}

async function build(): Promise<Snapshot> {
  const [products, tickerBody] = await Promise.all([
    allProducts(),
    getJson(`${BASE}/v2/tickers?contract_types=call_options,put_options,perpetual_futures`),
  ]);

  const tickers = new Map<string, RawTicker>();
  for (const t of ((tickerBody.result as RawTicker[] | undefined) ?? [])) {
    if (t.symbol) tickers.set(t.symbol, t);
  }

  const options: Instrument[] = [];
  const perpetuals: Instrument[] = [];

  for (const p of products) {
    const symbol = p.symbol;
    const ct = p.contract_type;
    if (!symbol || !ct) continue;
    const isOption = ct === "call_options" || ct === "put_options";
    const isPerp = ct === "perpetual_futures";
    if (!isOption && !isPerp) continue;

    const t = tickers.get(symbol);
    // A product with no ticker has no price, and a contract with no price
    // cannot be traded or valued. Dropping it is correct; showing it with a
    // blank price would let someone put an order on a number that isn't there.
    const mark = num(t?.mark_price);
    if (mark === null) continue;

    // The multiplier for premium AND P&L. Never assumed — see types.ts.
    const contractValue = num(p.contract_value) ?? 1;
    const quotes = t?.quotes ?? null;
    const open = num(t?.open);
    const close = num(t?.close);

    const inst: Instrument = {
      symbol,
      kind: isOption ? "OPTION" : "PERPETUAL",
      underlying: p.underlying_asset?.symbol ?? symbol.replace(/USD$/, ""),
      expiry: isOption ? isoDate(p.settlement_time) : null,
      strike: isOption ? num(p.strike_price) : null,
      optionType: isOption ? (ct === "call_options" ? "CALL" : "PUT") : null,
      contractValue,
      tickSize: num(p.tick_size) ?? 0.01,
      markPrice: mark,
      bid: num(quotes?.best_bid),
      ask: num(quotes?.best_ask),
      spot: num(t?.spot_price) ?? num(t?.greeks?.spot),
      openInterest: num(t?.oi),
      turnoverUsd: num(t?.turnover_usd),
      change24hPct: open && close && open !== 0 ? ((close - open) / open) * 100 : null,
      // Delta reports option vol as a fraction; the chain shows a percent.
      ivPct: isOption ? (num(t?.mark_vol) !== null ? (num(t?.mark_vol) as number) * 100 : null) : null,
      greeks: isOption ? greeksOf(t?.greeks) : null,
      fundingRatePct8h: isPerp ? num(t?.funding_rate) : null,
      // The venue's own margin parameters. The fallbacks are the most
      // CONSERVATIVE reading (2% initial, 1% maintenance = 50x), not a typical
      // one: if the feed ever stops publishing these, the desk should offer
      // less leverage than the venue would, never more.
      initialMarginPct: num(p.initial_margin) ?? 2,
      maintenanceMarginPct: num(p.maintenance_margin) ?? 1,
      imScalingFactor: num(p.initial_margin_scaling_factor) ?? 0,
      mmScalingFactor: num(p.maintenance_margin_scaling_factor) ?? 0,
      defaultLeverage: num(p.default_leverage) ?? 10,
      // Never a number of ours: the venue's floor on initial margin IS the
      // ceiling on leverage, so 0.5% initial margin means exactly 200x.
      maxLeverage: Math.max(1, Math.floor(100 / (num(p.initial_margin) ?? 2))),
      penaltyFactor: num(p.liquidation_penalty_factor) ?? 0,
    };

    if (isOption) options.push(inst);
    else perpetuals.push(inst);
  }

  // Underlyings are whatever actually has listed options, with the spot the
  // chain itself reports, so the header cannot disagree with the ladder.
  const byUnderlying = new Map<string, { spot: number | null; n: number }>();
  for (const o of options) {
    const cur = byUnderlying.get(o.underlying) ?? { spot: null, n: 0 };
    cur.n += 1;
    if (cur.spot === null && o.spot !== null) cur.spot = o.spot;
    byUnderlying.set(o.underlying, cur);
  }
  const underlyings: Underlying[] = [...byUnderlying.entries()]
    .map(([symbol, v]) => ({ symbol, spot: v.spot, optionCount: v.n }))
    .sort((a, b) => b.optionCount - a.optionCount);

  const bySymbol = new Map<string, Instrument>();
  for (const i of [...options, ...perpetuals]) bySymbol.set(i.symbol, i);

  return { bySymbol, options, perpetuals, underlyings, builtAt: Date.now() };
}

export async function listUnderlyings(): Promise<Underlying[]> {
  return (await getSnapshot()).underlyings;
}

/** Expiries that actually have listed options, soonest first. */
export async function listOptionExpiries(underlying: string): Promise<string[]> {
  const s = await getSnapshot();
  const set = new Set<string>();
  for (const o of s.options) {
    if (o.underlying === underlying && o.expiry) set.add(o.expiry);
  }
  return [...set].sort();
}

export async function getInstrument(symbol: string): Promise<Instrument | null> {
  const s = await getSnapshot();
  return s.bySymbol.get(symbol) ?? null;
}

/** Marks for a set of symbols, for valuing open positions in one pass. */
export async function marksFor(symbols: string[]): Promise<Map<string, Instrument>> {
  const s = await getSnapshot();
  const out = new Map<string, Instrument>();
  for (const sym of symbols) {
    const i = s.bySymbol.get(sym);
    if (i) out.set(sym, i);
  }
  return out;
}

export async function getOptionChain(underlying: string, expiry: string): Promise<OptionChain> {
  const s = await getSnapshot();
  const byStrike = new Map<number, { call: Instrument | null; put: Instrument | null }>();
  let spot: number | null = null;

  for (const o of s.options) {
    if (o.underlying !== underlying || o.expiry !== expiry || o.strike === null) continue;
    if (spot === null && o.spot !== null) spot = o.spot;
    const cur = byStrike.get(o.strike) ?? { call: null, put: null };
    if (o.optionType === "CALL") cur.call = o;
    else cur.put = o;
    byStrike.set(o.strike, cur);
  }

  const rows = [...byStrike.entries()]
    .map(([strike, v]) => ({ strike, call: v.call, put: v.put }))
    .sort((a, b) => a.strike - b.strike);

  let atmStrike: number | null = null;
  if (spot !== null && rows.length > 0) {
    atmStrike = rows.reduce((best, r) =>
      Math.abs(r.strike - (spot as number)) < Math.abs(best.strike - (spot as number)) ? r : best,
    ).strike;
  }

  return { underlying, expiry, spot, rows, atmStrike, asOf: s.builtAt };
}

/**
 * The listed strike nearest spot for one expiry, off the cached snapshot.
 *
 * Exists so a position can say whether it is ALREADY at the money without
 * building a whole chain per row. A roll of a leg that is already there is a
 * no-op, and the UI needs to know that before the button is pressed rather than
 * after — a confirm dialog promising to close and re-open, followed by nothing
 * happening, reads as a broken button.
 */
export async function atmStrikeFor(underlying: string, expiry: string): Promise<number | null> {
  const s = await getSnapshot();
  let spot: number | null = null;
  const strikes: number[] = [];
  for (const o of s.options) {
    if (o.underlying !== underlying || o.expiry !== expiry || o.strike === null) continue;
    if (spot === null && o.spot !== null) spot = o.spot;
    strikes.push(o.strike);
  }
  if (spot === null || strikes.length === 0) return null;
  return strikes.reduce((best, k) => (Math.abs(k - spot!) < Math.abs(best - spot!) ? k : best));
}

/**
 * An EXPIRED option, priced at what it actually settled for.
 *
 * WHY THIS EXISTS. Delta drops a contract from the ticker feed the moment it
 * expires, so an expired leg vanished from the snapshot and every path that
 * needed a price refused it — including Close. A position that cannot be
 * closed is stuck in the book forever, holding margin against a contract that
 * no longer exists, and the only message was "no longer listed, so it cannot be
 * priced to close". Expiry is the most ordinary thing that happens to an
 * option; it must not be the one event the desk cannot handle.
 *
 * Delta still serves two things after expiry, and between them the settlement
 * is recoverable exactly:
 *
 *   /v2/products/{symbol}   the spec — strike, contract value, settlement_time
 *   /v2/history/candles     the underlying's price at that settlement minute
 *
 * An option settles at INTRINSIC against the underlying at settlement, so that
 * is what is computed here. Not the option's own last trade, which is a price
 * someone paid before expiry and is not what the contract paid out; and not the
 * CURRENT spot, which has moved since and would settle the contract against a
 * market that did not exist when it expired.
 */
const settlementCache = new Map<string, Instrument | null>();

export async function getSettledInstrument(symbol: string): Promise<Instrument | null> {
  if (settlementCache.has(symbol)) return settlementCache.get(symbol) ?? null;
  const resolved = await resolveSettled(symbol);
  // Settlement never changes once it has happened, so this is cached for the
  // life of the process rather than re-fetched per read.
  settlementCache.set(symbol, resolved);
  return resolved;
}

async function resolveSettled(symbol: string): Promise<Instrument | null> {
  let p: RawProduct & { strike_price?: string | number | null };
  try {
    const body = await getJson(`${BASE}/v2/products/${encodeURIComponent(symbol)}`);
    p = (body.result as typeof p) ?? ({} as typeof p);
  } catch {
    return null;
  }
  if (!p.symbol || !p.settlement_time) return null;
  const settleAt = new Date(p.settlement_time).getTime();
  if (!Number.isFinite(settleAt) || settleAt > Date.now()) return null; // not expired

  const ct = p.contract_type;
  const isOption = ct === "call_options" || ct === "put_options";
  if (!isOption) return null;
  const strike = num(p.strike_price);
  const underlying = p.underlying_asset?.symbol;
  if (strike === null || !underlying) return null;

  const settleSpot = await spotAt(`${underlying}USD`, Math.floor(settleAt / 1000));
  if (settleSpot === null) return null;

  const isCall = ct === "call_options";
  const intrinsic = isCall ? Math.max(0, settleSpot - strike) : Math.max(0, strike - settleSpot);

  return {
    symbol: p.symbol,
    kind: "OPTION",
    underlying,
    expiry: isoDate(p.settlement_time),
    strike,
    optionType: isCall ? "CALL" : "PUT",
    contractValue: num(p.contract_value) ?? 1,
    tickSize: num(p.tick_size) ?? 0.01,
    // Settled contracts trade at exactly their settlement value, both sides.
    markPrice: intrinsic,
    bid: intrinsic,
    ask: intrinsic,
    spot: settleSpot,
    openInterest: null,
    turnoverUsd: null,
    change24hPct: null,
    ivPct: null,
    // No greeks on a settled contract: it has no time value and cannot move.
    greeks: { delta: isCall ? (intrinsic > 0 ? 1 : 0) : intrinsic > 0 ? -1 : 0, gamma: 0, theta: 0, vega: 0, rho: 0 },
    fundingRatePct8h: null,
    initialMarginPct: num(p.initial_margin) ?? 2,
    maintenanceMarginPct: num(p.maintenance_margin) ?? 1,
    imScalingFactor: num(p.initial_margin_scaling_factor) ?? 0,
    mmScalingFactor: num(p.maintenance_margin_scaling_factor) ?? 0,
    defaultLeverage: num(p.default_leverage) ?? 1,
    maxLeverage: 1,
    penaltyFactor: num(p.liquidation_penalty_factor) ?? 0,
  };
}

/** The underlying's close at or just before a unix second. */
async function spotAt(symbol: string, unixSec: number): Promise<number | null> {
  try {
    const body = await getJson(
      `${BASE}/v2/history/candles?resolution=1m&symbol=${encodeURIComponent(symbol)}&start=${unixSec - 900}&end=${unixSec + 120}`,
    );
    const rows = ((body.result as { time?: number; close?: string | number }[] | undefined) ?? [])
      .filter((r) => typeof r.time === "number" && (r.time as number) <= unixSec)
      .sort((a, b) => (a.time as number) - (b.time as number));
    const last = rows[rows.length - 1];
    return last ? num(last.close) : null;
  } catch {
    return null;
  }
}

export async function listPerpetuals(): Promise<Instrument[]> {
  const s = await getSnapshot();
  return [...s.perpetuals].sort((a, b) => (b.turnoverUsd ?? 0) - (a.turnoverUsd ?? 0));
}

/**
 * The busiest calls and puts.
 *
 * Ranked by TURNOVER, not by percentage change. A far-OTM option that ticked
 * from $0.05 to $0.15 is up 200% and is not a mover in any sense a trader
 * cares about; ranking by traded value surfaces the contracts with a real book
 * behind them, which is what makes the tab actionable.
 */
export async function getTopMovers(limit = 10): Promise<{ topCalls: TopMover[]; topPuts: TopMover[] }> {
  const s = await getSnapshot();
  const toMover = (i: Instrument): TopMover => ({
    symbol: i.symbol,
    displayName: displayNameOf(i),
    underlying: i.underlying,
    expiry: i.expiry,
    strike: i.strike,
    optionType: i.optionType,
    markPrice: i.markPrice,
    change24hPct: i.change24hPct,
    turnoverUsd: i.turnoverUsd,
    openInterest: i.openInterest,
  });
  const pick = (t: OptionType) =>
    s.options
      .filter((o) => o.optionType === t)
      .sort((a, b) => (b.turnoverUsd ?? 0) - (a.turnoverUsd ?? 0))
      .slice(0, limit)
      .map(toMover);
  return { topCalls: pick("CALL"), topPuts: pick("PUT") };
}
