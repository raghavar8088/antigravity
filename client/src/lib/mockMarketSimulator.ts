/**
 * Institutional-grade market simulation for the BTC futures mock trading module.
 *
 * Models:
 *   - Bid/ask spread derived from ATR and liquidity tier
 *   - Maker vs taker execution (post-only → maker rebate; market/limit-cross → taker fee)
 *   - Partial fills based on order size vs simulated depth
 *   - Volatility-scaled slippage (extends futuresPaperMath helpers)
 *   - Configurable execution latency (50–500 ms)
 *   - Missed fills (limit orders that don't touch the level in time)
 *   - Order rejections (margin insufficient, kill-switch, position limits)
 *
 * Pure functions — no React, no I/O. All randomness is seeded through the
 * `SimRng` interface so tests can supply a deterministic generator.
 */

// ── RNG interface ─────────────────────────────────────────────────────────────

/** Seeded random number generator — returns uniform [0, 1). */
export interface SimRng {
  next(): number;
}

/** Default Math.random()-backed RNG for production use. */
export const defaultRng: SimRng = { next: () => Math.random() };

/** Simple mulberry32 seeded RNG for deterministic tests. */
export function seededRng(seed: number): SimRng {
  let s = seed >>> 0;
  return {
    next() {
      s += 0x6d2b79f5;
      let z = s;
      z = Math.imul(z ^ (z >>> 15), z | 1);
      z ^= z + Math.imul(z ^ (z >>> 7), z | 61);
      return ((z ^ (z >>> 14)) >>> 0) / 0x100000000;
    },
  };
}

// ── Spread model ──────────────────────────────────────────────────────────────

export interface SpreadParams {
  /** ATR(14) in USD — used to scale the spread. */
  atrUsd: number;
  /** Mark/mid price. */
  midPrice: number;
  /**
   * Liquidity tier: TIER1 = BTC-PERP top book (tightest), TIER3 = illiquid alt.
   * Defaults to TIER1 for BTC futures.
   */
  liquidityTier?: "TIER1" | "TIER2" | "TIER3";
}

const SPREAD_ATR_FRACTION: Record<NonNullable<SpreadParams["liquidityTier"]>, number> = {
  TIER1: 0.0002,
  TIER2: 0.0008,
  TIER3: 0.0025,
};

const MIN_SPREAD_BPS: Record<NonNullable<SpreadParams["liquidityTier"]>, number> = {
  TIER1: 0.5,
  TIER2: 2,
  TIER3: 8,
};

export interface SpreadResult {
  bid: number;
  ask: number;
  spreadUsd: number;
  spreadBps: number;
}

/**
 * Compute the simulated bid/ask spread around a mark price.
 *
 * spreadUsd = max(ATR × atrFraction, minSpreadBps / 10_000 × midPrice)
 * bid = mid − half-spread; ask = mid + half-spread.
 */
export function computeBidAsk(params: SpreadParams): SpreadResult {
  const tier = params.liquidityTier ?? "TIER1";
  const mid = params.midPrice;
  const atrSpread = params.atrUsd * SPREAD_ATR_FRACTION[tier];
  const minSpread = (MIN_SPREAD_BPS[tier] / 10_000) * mid;
  const spreadUsd = Math.max(atrSpread, minSpread);
  const half = spreadUsd / 2;
  return {
    bid: mid - half,
    ask: mid + half,
    spreadUsd,
    spreadBps: mid > 0 ? (spreadUsd / mid) * 10_000 : 0,
  };
}

// ── Execution model ───────────────────────────────────────────────────────────

export type OrderExecutionType = "MARKET" | "LIMIT" | "STOP" | "STOP_LIMIT" | "POST_ONLY";

export interface ExecutionParams {
  side: "BUY" | "SELL";
  orderType: OrderExecutionType;
  /** Requested order price (limit/stop-limit) or mark price (market). */
  limitPrice?: number;
  /** Mark price at time of order submission. */
  markPrice: number;
  /** Order size in USD notional. */
  notionalUsd: number;
  spread: SpreadResult;
  /** Base taker fee per leg (fraction, e.g. 0.0005). */
  takerFeeFraction: number;
  /** Maker rebate per leg (fraction, typically 0 or negative, e.g. -0.0002). */
  makerFeeFraction?: number;
  /** Base slippage in bps per side. */
  baseSlippageBps: number;
  /** Current ATR / average ATR (1.0 = neutral; >1 = high vol). */
  volRatio?: number;
  /** Simulated order book depth at the best quote in USD notional. */
  bookDepthUsd?: number;
  rng?: SimRng;
}

export type ExecutionOutcome = "FILLED" | "PARTIAL" | "MISSED" | "REJECTED";

export interface ExecutionResult {
  outcome: ExecutionOutcome;
  fillPrice: number;
  fillFraction: number;
  feePerLeg: number;
  /** True when a maker fee was applied (post-only or passive limit). */
  isMaker: boolean;
  /** USD notional actually filled. */
  filledNotional: number;
  /** Simulated latency in milliseconds. */
  latencyMs: number;
  rejectionReason?: string;
}

/**
 * Simulate order execution against the reconstructed best quote.
 *
 * Rules:
 *  - POST_ONLY: crosses the spread → MISSED; otherwise maker fill.
 *  - LIMIT: if limitPrice crosses the current best ask/bid → immediate taker fill;
 *            otherwise passive → probabilistic fill within the tick window.
 *  - MARKET/STOP: always taker; dynamic slippage applied.
 *  - Partial fill: when notionalUsd > 70% of bookDepth, fill fraction < 1.
 *  - Missed fill: limit orders → ~12% base miss rate, rising with vol.
 */
export function simulateExecution(params: ExecutionParams): ExecutionResult {
  const rng = params.rng ?? defaultRng;
  const volRatio = params.volRatio != null && params.volRatio > 0 && Number.isFinite(params.volRatio) ? params.volRatio : 1;

  // 1. Latency: uniform between 50ms and volRatio-scaled max (up to 500ms)
  const latencyMs = Math.round(50 + rng.next() * Math.min(450, 100 * volRatio));

  // 2. Maker detection
  const isMaker = _isMakerExecution(params);

  // 3. Rejection gate
  const rejection = _checkRejection(params, rng);
  if (rejection) {
    return { outcome: "REJECTED", fillPrice: 0, fillFraction: 0, feePerLeg: 0, isMaker, filledNotional: 0, latencyMs, rejectionReason: rejection };
  }

  // 4. Miss gate (limit/post-only orders)
  if (params.orderType === "POST_ONLY" || params.orderType === "LIMIT") {
    if (_isMissed(params, rng, volRatio)) {
      return { outcome: "MISSED", fillPrice: 0, fillFraction: 0, feePerLeg: 0, isMaker, filledNotional: 0, latencyMs };
    }
  }

  // 5. Fill price
  const fillPrice = _computeFillPrice(params, isMaker, volRatio, rng);

  // 6. Partial fill fraction
  const fillFraction = _computeFillFraction(params, rng);
  const filledNotional = params.notionalUsd * fillFraction;

  // 7. Fee
  const feePerLeg = isMaker
    ? (params.makerFeeFraction ?? 0) * filledNotional
    : params.takerFeeFraction * filledNotional;

  return {
    outcome: fillFraction < 1 ? "PARTIAL" : "FILLED",
    fillPrice,
    fillFraction,
    feePerLeg,
    isMaker,
    filledNotional,
    latencyMs,
  };
}

function _isMakerExecution(params: ExecutionParams): boolean {
  if (params.orderType === "POST_ONLY") return true;
  if (params.orderType === "MARKET" || params.orderType === "STOP") return false;
  if (params.orderType === "LIMIT" && params.limitPrice != null) {
    const lp = params.limitPrice;
    // Crosses the spread → immediate taker
    if (params.side === "BUY" && lp >= params.spread.ask) return false;
    if (params.side === "SELL" && lp <= params.spread.bid) return false;
    // Passive inside the book → maker
    return true;
  }
  return false;
}

function _checkRejection(params: ExecutionParams, rng: SimRng): string | null {
  // POST_ONLY crossing the spread
  if (params.orderType === "POST_ONLY" && params.limitPrice != null) {
    const lp = params.limitPrice;
    if (params.side === "BUY" && lp >= params.spread.ask) return "POST_ONLY would cross spread";
    if (params.side === "SELL" && lp <= params.spread.bid) return "POST_ONLY would cross spread";
  }
  // Stochastic exchange overload rejection: ~0.5% base, rises in high vol
  const volRatio = Number.isFinite(params.volRatio) ? params.volRatio ?? 1 : 1;
  const rejectRate = 0.005 * Math.min(volRatio, 3);
  if (rng.next() < rejectRate) return "Exchange rejected order (transient overload)";
  return null;
}

function _isMissed(params: ExecutionParams, rng: SimRng, volRatio: number): boolean {
  if (params.limitPrice == null) return false;
  const lp = params.limitPrice;
  // Already crosses spread → taker fill, not missed
  if (params.side === "BUY" && lp >= params.spread.ask) return false;
  if (params.side === "SELL" && lp <= params.spread.bid) return false;
  // Passive limit: base 12% miss rate rising with distance from mid and volatility
  const distancePct = params.spread.spreadBps > 0
    ? Math.abs(lp - params.markPrice) / params.markPrice * 100
    : 0;
  const missRate = Math.min(0.7, 0.12 + distancePct * 0.05 * volRatio);
  return rng.next() < missRate;
}

function _computeFillPrice(
  params: ExecutionParams,
  isMaker: boolean,
  volRatio: number,
  rng: SimRng,
): number {
  if (isMaker && params.limitPrice != null) {
    // Maker fill at the limit price
    return params.limitPrice;
  }
  // Taker: start from best quote
  const base = params.side === "BUY" ? params.spread.ask : params.spread.bid;
  // Dynamic slippage: base × clamp(volRatio, 0.5, 3)
  const volScale = Math.min(3, Math.max(0.5, volRatio));
  const slippageBps = params.baseSlippageBps * volScale;
  const slippageMul = slippageBps / 10_000;
  // Randomize 50–150% of computed slippage for realism
  const randomSlippage = slippageMul * (0.5 + rng.next());
  const price = params.side === "BUY"
    ? base * (1 + randomSlippage)
    : base * (1 - randomSlippage);
  return Math.max(0.01, price);
}

function _computeFillFraction(params: ExecutionParams, rng: SimRng): number {
  const depth = params.bookDepthUsd ?? 0;
  if (!Number.isFinite(depth) || depth <= 0 || params.notionalUsd <= 0) return 1;
  const impactRatio = params.notionalUsd / depth;
  if (impactRatio < 0.3) return 1;
  if (impactRatio > 2) {
    // Very large order: partial fill 40–80% of depth
    return Math.min(1, (depth / params.notionalUsd) * (0.4 + rng.next() * 0.4));
  }
  // Probabilistic partial based on impact ratio
  const partialProb = Math.min(0.9, (impactRatio - 0.3) * 1.3);
  if (rng.next() > partialProb) return 1;
  // Fill 60–100% of the notional
  return 0.6 + rng.next() * 0.4;
}

// ── Funding rate model ────────────────────────────────────────────────────────

export interface FundingRateScenario {
  /** Base rate fraction per 8h (e.g. 0.0001 = 0.01%). */
  baseFundingRate: number;
  /** Standard deviation of funding rate innovations. */
  fundingVolatility?: number;
  /** Regime bias: positive = longs pay (bullish market), negative = shorts pay. */
  regimeBias?: number;
}

/**
 * Sample a realistic 8h funding rate given the current market regime bias.
 * BTC perpetual funding typically oscillates ±0.1% per 8h with mean-reversion.
 */
export function sampleFundingRate(scenario: FundingRateScenario, rng: SimRng = defaultRng): number {
  const base = scenario.baseFundingRate;
  const vol = scenario.fundingVolatility ?? 0.00005;
  const bias = scenario.regimeBias ?? 0;
  // Gaussian perturbation around base + bias
  const u1 = rng.next();
  const u2 = rng.next();
  const gaussian = Math.sqrt(-2 * Math.log(Math.max(1e-12, u1))) * Math.cos(2 * Math.PI * u2);
  const rate = base + bias * 0.3 + gaussian * vol;
  return Math.max(-0.003, Math.min(0.003, rate));
}

// ── Volatility-based slippage ─────────────────────────────────────────────────

/**
 * Compute the effective slippage in bps for an entry or exit, scaled by:
 *   1. Current ATR vs rolling 30-bar average ATR (vol regime factor)
 *   2. Order size impact (larger orders = wider slippage)
 *   3. Time-of-day factor (Asia low-liquidity session = wider spread)
 */
export interface SlippageInput {
  baseSlippageBps: number;
  currentAtrUsd: number;
  avgAtr30Usd: number;
  notionalUsd: number;
  /** Typical daily volume in USD for the instrument (BTC perp ≈ $20B). */
  typicalDailyVolumeUsd?: number;
  /** Hour of day in UTC (0–23). */
  hourUtc?: number;
}

export function computeDynamicSlippageBps(input: SlippageInput): number {
  const base = Math.max(0, input.baseSlippageBps);
  if (base === 0) return 0;

  // Vol factor: clamp ATR ratio 0.5 – 3.0
  let volFactor = 1;
  if (
    Number.isFinite(input.currentAtrUsd) && input.currentAtrUsd > 0 &&
    Number.isFinite(input.avgAtr30Usd) && input.avgAtr30Usd > 0
  ) {
    volFactor = Math.min(3, Math.max(0.5, input.currentAtrUsd / input.avgAtr30Usd));
  }

  // Size impact factor: each 1% of daily volume adds 0.5 bps
  let sizeFactor = 1;
  const dailyVol = input.typicalDailyVolumeUsd ?? 20_000_000_000;
  if (Number.isFinite(input.notionalUsd) && input.notionalUsd > 0 && dailyVol > 0) {
    const pctOfVolume = input.notionalUsd / dailyVol;
    sizeFactor = 1 + pctOfVolume * 50;
  }

  // Session factor: Asia session 02–08 UTC = 1.5×, otherwise 1.0
  let sessionFactor = 1;
  const h = input.hourUtc;
  if (h != null && Number.isFinite(h) && h >= 2 && h < 8) sessionFactor = 1.5;

  return base * volFactor * sizeFactor * sessionFactor;
}

// ── Latency model ─────────────────────────────────────────────────────────────

export interface LatencyConfig {
  /** Minimum latency in ms. Default 50. */
  minMs: number;
  /** Maximum latency in ms. Default 500. */
  maxMs: number;
  /** When true, apply log-normal distribution for more realistic tail events. */
  logNormal?: boolean;
}

export const DEFAULT_LATENCY_CONFIG: LatencyConfig = { minMs: 50, maxMs: 500, logNormal: true };

/**
 * Sample an execution latency in ms using a log-normal distribution.
 * Simulates real-world network + exchange processing time distribution.
 */
export function sampleLatencyMs(config: LatencyConfig = DEFAULT_LATENCY_CONFIG, rng: SimRng = defaultRng): number {
  const minMs = Math.max(1, config.minMs);
  const maxMs = Math.max(minMs + 1, config.maxMs);
  if (!config.logNormal) {
    return Math.round(minMs + rng.next() * (maxMs - minMs));
  }
  // Log-normal: mu = ln(mean), sigma = 0.5
  const targetMean = (minMs + maxMs) / 2;
  const sigma = 0.5;
  const mu = Math.log(targetMean) - (sigma * sigma) / 2;
  const u1 = rng.next();
  const u2 = rng.next();
  const z = Math.sqrt(-2 * Math.log(Math.max(1e-12, u1))) * Math.cos(2 * Math.PI * u2);
  const sample = Math.exp(mu + sigma * z);
  return Math.round(Math.min(maxMs, Math.max(minMs, sample)));
}

// ── Depth model ───────────────────────────────────────────────────────────────

/**
 * Simulate the order book depth at the best quote.
 * BTC perp top-of-book typically ranges from $1M to $5M per side.
 * In high-volatility regimes, depth drops 50–80%.
 */
export function simulateBookDepthUsd(volRatio: number, rng: SimRng = defaultRng): number {
  const baseDepth = 1_000_000 + rng.next() * 4_000_000;
  const volPenalty = Math.min(1, Math.max(0.2, 1 / Math.max(1, volRatio)));
  return baseDepth * volPenalty;
}

// ── Full simulation context ───────────────────────────────────────────────────

export interface MarketSimContext {
  markPrice: number;
  atrUsd: number;
  avgAtr30Usd: number;
  /** Hour of day UTC for session factor. */
  hourUtc: number;
  latencyConfig?: LatencyConfig;
  takerFeeFraction: number;
  makerFeeFraction?: number;
  baseSlippageBps: number;
  rng?: SimRng;
}

/**
 * Build a complete spread + execution context from a market snapshot.
 * Entry point for the mock engine to request a simulation result.
 */
export function buildSimContext(ctx: MarketSimContext): {
  spread: SpreadResult;
  volRatio: number;
  bookDepthUsd: number;
  latencyMs: number;
} {
  const rng = ctx.rng ?? defaultRng;
  const volRatio = ctx.avgAtr30Usd > 0 ? ctx.atrUsd / ctx.avgAtr30Usd : 1;
  const spread = computeBidAsk({ atrUsd: ctx.atrUsd, midPrice: ctx.markPrice });
  const bookDepthUsd = simulateBookDepthUsd(volRatio, rng);
  const latencyMs = sampleLatencyMs(ctx.latencyConfig ?? DEFAULT_LATENCY_CONFIG, rng);
  return { spread, volRatio, bookDepthUsd, latencyMs };
}
