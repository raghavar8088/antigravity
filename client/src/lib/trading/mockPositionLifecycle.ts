/**
 * Position Lifecycle Engine for the BTC futures mock trading module.
 *
 * Manages the full lifecycle of a mock position:
 *   - Entry (initial fill)
 *   - Scale-in (adding to a winner/loser)
 *   - Scale-out (partial profit taking)
 *   - Breakeven stop movement (move SL to entry after a threshold is hit)
 *   - Dynamic trailing stop (widen/tighten based on profit)
 *   - Emergency exit (kill-switch, drawdown breach, news event)
 *   - Time-based exit (max hold, session close, funding event)
 *
 * Pure functions — no React, no I/O.
 */

import type { MockSide } from "@/lib/trading/mockTradingEngine";

// ── Position definition ───────────────────────────────────────────────────────

export type PositionStatus = "FLAT" | "OPEN" | "SCALING" | "REDUCING" | "CLOSED";
export type ExitTrigger =
  | "TAKE_PROFIT"
  | "STOP_LOSS"
  | "TRAILING_STOP"
  | "BREAKEVEN_STOP"
  | "SCALE_OUT"
  | "MAX_HOLD"
  | "TIME_EXIT"
  | "EMERGENCY"
  | "MANUAL";

export interface PositionLeg {
  id: string;
  /** Fill price for this leg. */
  fillPrice: number;
  /** USD notional of this leg. */
  notionalUsd: number;
  /** Quantity in base currency. */
  quantity: number;
  filledAt: number;
  /** Is this a scale-in addition or scale-out reduction? */
  legType: "ENTRY" | "SCALE_IN" | "SCALE_OUT" | "EXIT";
}

export interface ManagedPosition {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: MockSide;
  status: PositionStatus;

  /** All legs (entries + exits) chronologically. */
  legs: PositionLeg[];

  /** Volume-weighted average entry price across all ENTRY + SCALE_IN legs. */
  avgEntryPrice: number;
  /** Current total USD notional (entry legs minus exit legs). */
  notionalUsd: number;
  /** Current total quantity. */
  quantity: number;

  /** Active stop loss price. Moves up for longs as breakeven/trailing fires. */
  stopLossPrice: number;
  /** Take profit price. */
  takeProfitPrice: number;
  /** Trailing stop active flag. */
  trailingStopActive: boolean;
  /** Trailing stop distance in USD. */
  trailingStopDistUsd: number;
  /** High-water mark for trailing stop (highest price seen for longs, lowest for shorts). */
  trailingHighWater: number;
  /** Whether breakeven stop has been activated. */
  breakevenActive: boolean;

  openedAt: number;
  closedAt: number | null;
  exitTrigger: ExitTrigger | null;
  exitPrice: number | null;

  /** Net realized PnL from scale-out and close legs. */
  realizedPnl: number;
  /** Fees accrued across all legs (entry + exit). */
  totalFeesUsd: number;

  /** Emergency exit pending flag. */
  emergencyExit: boolean;

  /** Regime at entry for research tagging. */
  regimeAtEntry?: string;
}

// ── Lifecycle config ──────────────────────────────────────────────────────────

export interface LifecycleConfig {
  /** Move SL to entry when unrealized profit reaches this % of entry. Default 0.5%. */
  breakevenTriggerPct: number;
  /** Offset above/below entry for breakeven SL (bps). Default 5 bps. */
  breakevenOffsetBps: number;
  /** Activate trailing stop when profit reaches this % of entry. Default 0.75%. */
  trailingStopActivatePct: number;
  /** Trailing stop distance as % of current price. Default 0.3%. */
  trailingStopDistPct: number;
  /** Max scale-in additions allowed per position. Default 2. */
  maxScaleIns: number;
  /** Scale-in trigger: add when position is profitable by this %. Default 0.5%. */
  scaleInTriggerPct: number;
  /** Scale-in size as % of original notional. Default 50%. */
  scaleInSizePct: number;
  /** Scale-out trigger: take partial profit at this % of original TP. Default 50%. */
  scaleOutTriggerPct: number;
  /** Scale-out size as % of current notional. Default 50%. */
  scaleOutSizePct: number;
  /** Max hold time in minutes. */
  maxHoldMinutes: number;
  /** Taker fee per leg fraction. */
  takerFeeFraction: number;
  /** Slippage per side in bps. */
  slippageBpsPerSide: number;
}

export const DEFAULT_LIFECYCLE_CONFIG: LifecycleConfig = {
  breakevenTriggerPct: 0.5,
  breakevenOffsetBps: 5,
  trailingStopActivatePct: 0.75,
  trailingStopDistPct: 0.3,
  maxScaleIns: 2,
  scaleInTriggerPct: 0.5,
  scaleInSizePct: 50,
  scaleOutTriggerPct: 50,
  scaleOutSizePct: 50,
  maxHoldMinutes: 120,
  takerFeeFraction: 0.0005,
  slippageBpsPerSide: 5,
};

// ── Position creation ─────────────────────────────────────────────────────────

export function openPosition(args: {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: MockSide;
  fillPrice: number;
  notionalUsd: number;
  stopLossPrice: number;
  takeProfitPrice: number;
  now: number;
  config: LifecycleConfig;
  regimeAtEntry?: string;
}): ManagedPosition {
  const quantity = args.fillPrice > 0 ? args.notionalUsd / args.fillPrice : 0;
  const entryFee = args.notionalUsd * args.config.takerFeeFraction;
  const leg: PositionLeg = {
    id: `${args.id}-entry`,
    fillPrice: args.fillPrice,
    notionalUsd: args.notionalUsd,
    quantity,
    filledAt: args.now,
    legType: "ENTRY",
  };
  return {
    id: args.id,
    strategyId: args.strategyId,
    strategyName: args.strategyName,
    symbol: args.symbol,
    side: args.side,
    status: "OPEN",
    legs: [leg],
    avgEntryPrice: args.fillPrice,
    notionalUsd: args.notionalUsd,
    quantity,
    stopLossPrice: args.stopLossPrice,
    takeProfitPrice: args.takeProfitPrice,
    trailingStopActive: false,
    trailingStopDistUsd: (args.fillPrice * args.config.trailingStopDistPct) / 100,
    trailingHighWater: args.fillPrice,
    breakevenActive: false,
    openedAt: args.now,
    closedAt: null,
    exitTrigger: null,
    exitPrice: null,
    realizedPnl: 0,
    totalFeesUsd: entryFee,
    emergencyExit: false,
    regimeAtEntry: args.regimeAtEntry,
  };
}

// ── Scale-in ──────────────────────────────────────────────────────────────────

export interface ScaleInResult {
  position: ManagedPosition;
  scaled: boolean;
  reason: string;
}

/**
 * Attempt to add to an existing position (scale-in).
 * Only adds when the position is profitable by scaleInTriggerPct
 * and the max scale-in count hasn't been reached.
 */
export function tryScaleIn(args: {
  position: ManagedPosition;
  currentPrice: number;
  config: LifecycleConfig;
  now: number;
}): ScaleInResult {
  const { position, currentPrice, config } = args;
  if (position.status !== "OPEN") return { position, scaled: false, reason: "Position not OPEN" };

  const scaleIns = position.legs.filter((l) => l.legType === "SCALE_IN").length;
  if (scaleIns >= config.maxScaleIns) {
    return { position, scaled: false, reason: `Max scale-ins (${config.maxScaleIns}) reached` };
  }

  const pnlPct = _unrealizedPnlPct(position, currentPrice);
  if (pnlPct < config.scaleInTriggerPct) {
    return { position, scaled: false, reason: `Profit ${pnlPct.toFixed(2)}% below trigger ${config.scaleInTriggerPct}%` };
  }

  const addNotional = position.notionalUsd * (config.scaleInSizePct / 100);
  const addQty = currentPrice > 0 ? addNotional / currentPrice : 0;
  const fee = addNotional * config.takerFeeFraction;

  const leg: PositionLeg = {
    id: `${position.id}-si-${scaleIns + 1}`,
    fillPrice: currentPrice,
    notionalUsd: addNotional,
    quantity: addQty,
    filledAt: args.now,
    legType: "SCALE_IN",
  };

  const totalNotional = position.notionalUsd + addNotional;
  const totalQty = position.quantity + addQty;
  const avgEntry = totalQty > 0
    ? (position.avgEntryPrice * position.quantity + currentPrice * addQty) / totalQty
    : position.avgEntryPrice;

  return {
    position: {
      ...position,
      legs: [...position.legs, leg],
      notionalUsd: totalNotional,
      quantity: totalQty,
      avgEntryPrice: avgEntry,
      totalFeesUsd: position.totalFeesUsd + fee,
    },
    scaled: true,
    reason: `Scale-in ${scaleIns + 1} at ${currentPrice.toFixed(2)} (profit ${pnlPct.toFixed(2)}%)`,
  };
}

// ── Scale-out (partial profit taking) ────────────────────────────────────────

export interface ScaleOutResult {
  position: ManagedPosition;
  scaled: boolean;
  pnlRealized: number;
  reason: string;
}

/**
 * Attempt a partial profit-taking exit.
 * Reduces the position by scaleOutSizePct when price reaches scaleOutTriggerPct
 * of the way from entry to take-profit.
 */
export function tryScaleOut(args: {
  position: ManagedPosition;
  currentPrice: number;
  config: LifecycleConfig;
  now: number;
}): ScaleOutResult {
  const { position, currentPrice, config } = args;
  if (position.status !== "OPEN") {
    return { position, scaled: false, pnlRealized: 0, reason: "Position not OPEN" };
  }

  const alreadyScaledOut = position.legs.some((l) => l.legType === "SCALE_OUT");
  if (alreadyScaledOut) {
    return { position, scaled: false, pnlRealized: 0, reason: "Already scaled out" };
  }

  const tpDist = Math.abs(position.takeProfitPrice - position.avgEntryPrice);
  if (tpDist <= 0) return { position, scaled: false, pnlRealized: 0, reason: "No TP set" };

  const progressToTp = position.side === "BUY"
    ? (currentPrice - position.avgEntryPrice) / tpDist
    : (position.avgEntryPrice - currentPrice) / tpDist;

  const triggerFraction = config.scaleOutTriggerPct / 100;
  if (progressToTp < triggerFraction) {
    return {
      position,
      scaled: false,
      pnlRealized: 0,
      reason: `Progress ${(progressToTp * 100).toFixed(1)}% below trigger ${config.scaleOutTriggerPct}%`,
    };
  }

  const exitNotional = position.notionalUsd * (config.scaleOutSizePct / 100);
  const exitQty = exitNotional / Math.max(0.01, currentPrice);
  const fee = exitNotional * config.takerFeeFraction;

  const gross = position.side === "BUY"
    ? (currentPrice - position.avgEntryPrice) * exitQty
    : (position.avgEntryPrice - currentPrice) * exitQty;
  const net = gross - fee;

  const leg: PositionLeg = {
    id: `${position.id}-so-1`,
    fillPrice: currentPrice,
    notionalUsd: exitNotional,
    quantity: exitQty,
    filledAt: args.now,
    legType: "SCALE_OUT",
  };

  return {
    position: {
      ...position,
      legs: [...position.legs, leg],
      notionalUsd: position.notionalUsd - exitNotional,
      quantity: position.quantity - exitQty,
      realizedPnl: position.realizedPnl + net,
      totalFeesUsd: position.totalFeesUsd + fee,
    },
    scaled: true,
    pnlRealized: net,
    reason: `Scale-out at ${currentPrice.toFixed(2)} (${config.scaleOutSizePct}% of position)`,
  };
}

// ── Breakeven stop ────────────────────────────────────────────────────────────

/**
 * Move the stop-loss to entry + offset when profit exceeds the breakeven trigger.
 * Idempotent — no-ops if already active.
 */
export function applyBreakevenStop(
  position: ManagedPosition,
  currentPrice: number,
  config: LifecycleConfig,
): ManagedPosition {
  if (position.breakevenActive || position.status !== "OPEN") return position;

  const pnlPct = _unrealizedPnlPct(position, currentPrice);
  if (pnlPct < config.breakevenTriggerPct) return position;

  const offsetFraction = config.breakevenOffsetBps / 10_000;
  const newSl = position.side === "BUY"
    ? position.avgEntryPrice * (1 + offsetFraction)
    : position.avgEntryPrice * (1 - offsetFraction);

  // Only move SL in the favourable direction
  const improved = position.side === "BUY"
    ? newSl > position.stopLossPrice
    : newSl < position.stopLossPrice;

  if (!improved) return position;

  return { ...position, stopLossPrice: newSl, breakevenActive: true };
}

// ── Trailing stop ─────────────────────────────────────────────────────────────

/**
 * Activate and update a trailing stop on the position.
 * Once active, the stop trails the high-water mark by trailingStopDistUsd.
 */
export function applyTrailingStop(
  position: ManagedPosition,
  currentPrice: number,
  config: LifecycleConfig,
): ManagedPosition {
  if (position.status !== "OPEN") return position;

  // Activate when profit threshold reached
  if (!position.trailingStopActive) {
    const pnlPct = _unrealizedPnlPct(position, currentPrice);
    if (pnlPct < config.trailingStopActivatePct) return position;
    const distUsd = (currentPrice * config.trailingStopDistPct) / 100;
    return {
      ...position,
      trailingStopActive: true,
      trailingStopDistUsd: distUsd,
      trailingHighWater: currentPrice,
    };
  }

  // Update high-water mark and trailing stop level
  let { trailingHighWater, trailingStopDistUsd, stopLossPrice } = position;

  if (position.side === "BUY") {
    if (currentPrice > trailingHighWater) {
      trailingHighWater = currentPrice;
      const newSl = trailingHighWater - trailingStopDistUsd;
      if (newSl > stopLossPrice) stopLossPrice = newSl;
    }
  } else {
    if (currentPrice < trailingHighWater) {
      trailingHighWater = currentPrice;
      const newSl = trailingHighWater + trailingStopDistUsd;
      if (newSl < stopLossPrice) stopLossPrice = newSl;
    }
  }

  return { ...position, trailingHighWater, stopLossPrice };
}

// ── Price tick ────────────────────────────────────────────────────────────────

export interface PositionTickResult {
  position: ManagedPosition;
  unrealizedPnl: number;
  closed: boolean;
  exitTrigger: ExitTrigger | null;
}

/**
 * Apply a price tick to a managed position:
 *  1. Apply breakeven stop
 *  2. Apply trailing stop update
 *  3. Check TP/SL/max-hold triggers
 *  4. Apply scale-out at mid-point (if configured)
 *
 * Returns the updated position + PnL status.
 */
export function applyTickToPosition(args: {
  position: ManagedPosition;
  price: number;
  now: number;
  config: LifecycleConfig;
  enableScaleOut?: boolean;
}): PositionTickResult {
  let { position } = args;
  const { price, now, config } = args;

  if (position.status === "CLOSED") {
    return { position, unrealizedPnl: position.realizedPnl, closed: true, exitTrigger: position.exitTrigger };
  }
  if (position.emergencyExit) {
    return _closePosition(position, price, now, "EMERGENCY", config);
  }

  // Lifecycle updates
  position = applyBreakevenStop(position, price, config);
  position = applyTrailingStop(position, price, config);

  // Optional scale-out
  if (args.enableScaleOut) {
    const soResult = tryScaleOut({ position, currentPrice: price, config, now });
    if (soResult.scaled) position = soResult.position;
  }

  // Exit checks
  const ageMin = (now - position.openedAt) / 60_000;
  if (ageMin >= config.maxHoldMinutes) {
    return _closePosition(position, price, now, "MAX_HOLD", config);
  }

  const hitTp = position.side === "BUY"
    ? price >= position.takeProfitPrice
    : price <= position.takeProfitPrice;
  if (hitTp) return _closePosition(position, price, now, "TAKE_PROFIT", config);

  const hitSl = position.side === "BUY"
    ? price <= position.stopLossPrice
    : price >= position.stopLossPrice;
  if (hitSl) {
    const reason: ExitTrigger = position.trailingStopActive ? "TRAILING_STOP"
      : position.breakevenActive ? "BREAKEVEN_STOP"
      : "STOP_LOSS";
    return _closePosition(position, price, now, reason, config);
  }

  const unrealizedPnl = _computeUnrealizedPnl(position, price, config);
  return { position: { ...position, status: "OPEN" }, unrealizedPnl, closed: false, exitTrigger: null };
}

/** Mark position for emergency exit; will close on next tick. */
export function flagEmergencyExit(position: ManagedPosition): ManagedPosition {
  return { ...position, emergencyExit: true };
}

/** Manually close position at a given price. */
export function closePositionManual(
  position: ManagedPosition,
  price: number,
  now: number,
  config: LifecycleConfig,
): PositionTickResult {
  return _closePosition(position, price, now, "MANUAL", config);
}

// ── Session / time-based exit ─────────────────────────────────────────────────

/**
 * Check if the position should be closed due to time constraints:
 *   - Hour-based session close (e.g., end of Asia session)
 *   - Funding event window (close before 8h funding mark)
 */
export function shouldTimeExit(args: {
  position: ManagedPosition;
  now: number;
  /** Hour of day UTC (0–23) to close positions before funding. */
  fundingHoursUtc?: number[];
  /** Session hours UTC — close if outside [sessionStartH, sessionEndH]. */
  sessionWindowUtc?: [number, number];
}): boolean {
  if (args.position.status !== "OPEN") return false;
  const h = new Date(args.now).getUTCHours();

  if (args.fundingHoursUtc?.includes(h)) return true;

  const sw = args.sessionWindowUtc;
  if (sw != null) {
    if (h < sw[0] || h >= sw[1]) return true;
  }
  return false;
}

// ── Internal helpers ──────────────────────────────────────────────────────────

function _unrealizedPnlPct(position: ManagedPosition, currentPrice: number): number {
  const entry = position.avgEntryPrice;
  if (entry <= 0) return 0;
  return position.side === "BUY"
    ? ((currentPrice - entry) / entry) * 100
    : ((entry - currentPrice) / entry) * 100;
}

function _computeUnrealizedPnl(position: ManagedPosition, price: number, config: LifecycleConfig): number {
  const entry = position.avgEntryPrice;
  const qty = position.quantity;
  if (qty <= 0 || entry <= 0) return 0;
  const gross = position.side === "BUY"
    ? (price - entry) * qty
    : (entry - price) * qty;
  const exitFee = position.notionalUsd * config.takerFeeFraction;
  return gross - exitFee;
}

function _closePosition(
  position: ManagedPosition,
  price: number,
  now: number,
  trigger: ExitTrigger,
  config: LifecycleConfig,
): PositionTickResult {
  const exitFee = position.notionalUsd * config.takerFeeFraction;
  const gross = position.side === "BUY"
    ? (price - position.avgEntryPrice) * position.quantity
    : (position.avgEntryPrice - price) * position.quantity;
  const net = gross - exitFee + position.realizedPnl;

  const exitLeg: PositionLeg = {
    id: `${position.id}-exit`,
    fillPrice: price,
    notionalUsd: position.notionalUsd,
    quantity: position.quantity,
    filledAt: now,
    legType: "EXIT",
  };

  const closed: ManagedPosition = {
    ...position,
    status: "CLOSED",
    legs: [...position.legs, exitLeg],
    closedAt: now,
    exitTrigger: trigger,
    exitPrice: price,
    realizedPnl: net,
    totalFeesUsd: position.totalFeesUsd + exitFee,
    emergencyExit: false,
  };

  return { position: closed, unrealizedPnl: 0, closed: true, exitTrigger: trigger };
}

// ── Portfolio summary ─────────────────────────────────────────────────────────

export interface PositionPortfolioSummary {
  openCount: number;
  closedCount: number;
  totalNotionalUsd: number;
  totalUnrealizedPnl: number;
  totalRealizedPnl: number;
  longExposureUsd: number;
  shortExposureUsd: number;
  netExposureUsd: number;
}

export function summarizePositions(
  positions: readonly ManagedPosition[],
  currentPrice: number,
  config: LifecycleConfig,
): PositionPortfolioSummary {
  let openCount = 0, closedCount = 0, totalNotional = 0;
  let unrealized = 0, realized = 0, longExp = 0, shortExp = 0;

  for (const pos of positions) {
    if (pos.status === "OPEN" || pos.status === "SCALING" || pos.status === "REDUCING") {
      openCount++;
      totalNotional += pos.notionalUsd;
      unrealized += _computeUnrealizedPnl(pos, currentPrice, config);
      if (pos.side === "BUY") longExp += pos.notionalUsd;
      else shortExp += pos.notionalUsd;
    } else if (pos.status === "CLOSED") {
      closedCount++;
      realized += pos.realizedPnl;
    }
  }

  return {
    openCount,
    closedCount,
    totalNotionalUsd: totalNotional,
    totalUnrealizedPnl: unrealized,
    totalRealizedPnl: realized,
    longExposureUsd: longExp,
    shortExposureUsd: shortExp,
    netExposureUsd: longExp - shortExp,
  };
}
