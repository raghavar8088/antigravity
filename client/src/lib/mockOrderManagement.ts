/**
 * Advanced order management for the BTC futures mock trading module.
 *
 * Supports:
 *   - Market orders
 *   - Limit orders
 *   - Stop (market) orders
 *   - Stop-limit orders
 *   - Trailing stop orders (price-based and percentage-based)
 *   - Reduce-only orders
 *   - Post-only orders
 *   - OCO (One-Cancels-Other) pairs
 *
 * Pure functions — no React, no I/O. The order book is a value type that
 * callers replace on each price tick. Order IDs are strings; caller supplies.
 */

import type { MockSide } from "@/lib/mockTradingEngine";

// ── Order types ───────────────────────────────────────────────────────────────

export type OrderType =
  | "MARKET"
  | "LIMIT"
  | "STOP"
  | "STOP_LIMIT"
  | "TRAILING_STOP"
  | "REDUCE_ONLY"
  | "POST_ONLY"
  | "OCO";

export type OrderStatus = "PENDING" | "OPEN" | "TRIGGERED" | "FILLED" | "PARTIAL" | "CANCELLED" | "EXPIRED" | "REJECTED";

export interface BaseOrder {
  id: string;
  clientOrderId?: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: MockSide;
  orderType: OrderType;
  status: OrderStatus;
  /** USD notional of the original order. */
  notionalUsd: number;
  /** Fraction filled so far (0–1). */
  filledFraction: number;
  /** Average fill price across all partial fills. */
  avgFillPrice: number;
  /** Fees accrued in USD. */
  feesUsd: number;
  createdAt: number;
  updatedAt: number;
  /** Expire at this timestamp (ms). 0 = GTC (good-till-cancelled). */
  expireAt: number;
  /** OCO group ID — when one leg fills/cancels the other is cancelled. */
  ocoGroupId?: string;
  rejectionReason?: string;
  /** Metadata for why the order was created (diagnostic). */
  reason?: string;
}

// ── Market order ─────────────────────────────────────────────────────────────

export interface MarketOrder extends BaseOrder {
  orderType: "MARKET";
}

// ── Limit order ───────────────────────────────────────────────────────────────

export interface LimitOrder extends BaseOrder {
  orderType: "LIMIT";
  limitPrice: number;
  /** When true, the order becomes POST_ONLY (rejected if it would cross immediately). */
  postOnly: boolean;
}

// ── Stop (market) order ───────────────────────────────────────────────────────

export interface StopOrder extends BaseOrder {
  orderType: "STOP";
  /** Trigger price; once touched, converts to a market order. */
  stopPrice: number;
  triggered: boolean;
}

// ── Stop-limit order ──────────────────────────────────────────────────────────

export interface StopLimitOrder extends BaseOrder {
  orderType: "STOP_LIMIT";
  stopPrice: number;
  limitPrice: number;
  triggered: boolean;
}

// ── Trailing stop order ───────────────────────────────────────────────────────

export interface TrailingStopOrder extends BaseOrder {
  orderType: "TRAILING_STOP";
  /** Trail distance in percentage of price. Mutually exclusive with trailUsd. */
  trailPct?: number;
  /** Trail distance in USD. Mutually exclusive with trailPct. */
  trailUsd?: number;
  /** Current activation price (high-water mark for SELL, low-water mark for BUY). */
  activationPrice: number;
  /** Current trailing stop trigger price. */
  currentStopPrice: number;
  triggered: boolean;
}

// ── Reduce-only order ────────────────────────────────────────────────────────

export interface ReduceOnlyOrder extends BaseOrder {
  orderType: "REDUCE_ONLY";
  limitPrice?: number;
  /** Position ID this order reduces. */
  positionId: string;
}

// ── Post-only order ───────────────────────────────────────────────────────────

export interface PostOnlyOrder extends BaseOrder {
  orderType: "POST_ONLY";
  limitPrice: number;
}

// ── OCO order ─────────────────────────────────────────────────────────────────

export interface OcoOrder extends BaseOrder {
  orderType: "OCO";
  /** The take-profit leg (limit order). */
  takeProfitLeg: LimitOrder;
  /** The stop-loss leg (stop order). */
  stopLossLeg: StopOrder;
  /** Which leg filled (if any). */
  filledLeg?: "TP" | "SL";
}

export type AnyOrder =
  | MarketOrder
  | LimitOrder
  | StopOrder
  | StopLimitOrder
  | TrailingStopOrder
  | ReduceOnlyOrder
  | PostOnlyOrder
  | OcoOrder;

// ── Order book ────────────────────────────────────────────────────────────────

export interface MockOrderBook {
  orders: AnyOrder[];
  /** Next OCO group counter. */
  ocoCounter: number;
}

export function emptyOrderBook(): MockOrderBook {
  return { orders: [], ocoCounter: 0 };
}

// ── Order creation helpers ────────────────────────────────────────────────────

export function createMarketOrder(args: {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: MockSide;
  notionalUsd: number;
  now: number;
  reason?: string;
}): MarketOrder {
  return {
    id: args.id,
    strategyId: args.strategyId,
    strategyName: args.strategyName,
    symbol: args.symbol,
    side: args.side,
    orderType: "MARKET",
    status: "OPEN",
    notionalUsd: args.notionalUsd,
    filledFraction: 0,
    avgFillPrice: 0,
    feesUsd: 0,
    createdAt: args.now,
    updatedAt: args.now,
    expireAt: 0,
    reason: args.reason,
  };
}

export function createLimitOrder(args: {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: MockSide;
  notionalUsd: number;
  limitPrice: number;
  postOnly?: boolean;
  expireAt?: number;
  now: number;
  reason?: string;
}): LimitOrder {
  return {
    id: args.id,
    strategyId: args.strategyId,
    strategyName: args.strategyName,
    symbol: args.symbol,
    side: args.side,
    orderType: "LIMIT",
    status: "OPEN",
    notionalUsd: args.notionalUsd,
    filledFraction: 0,
    avgFillPrice: 0,
    feesUsd: 0,
    createdAt: args.now,
    updatedAt: args.now,
    expireAt: args.expireAt ?? 0,
    limitPrice: args.limitPrice,
    postOnly: args.postOnly ?? false,
    reason: args.reason,
  };
}

export function createStopOrder(args: {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: MockSide;
  notionalUsd: number;
  stopPrice: number;
  expireAt?: number;
  now: number;
  reason?: string;
}): StopOrder {
  return {
    id: args.id,
    strategyId: args.strategyId,
    strategyName: args.strategyName,
    symbol: args.symbol,
    side: args.side,
    orderType: "STOP",
    status: "OPEN",
    notionalUsd: args.notionalUsd,
    filledFraction: 0,
    avgFillPrice: 0,
    feesUsd: 0,
    createdAt: args.now,
    updatedAt: args.now,
    expireAt: args.expireAt ?? 0,
    stopPrice: args.stopPrice,
    triggered: false,
    reason: args.reason,
  };
}

export function createStopLimitOrder(args: {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: MockSide;
  notionalUsd: number;
  stopPrice: number;
  limitPrice: number;
  expireAt?: number;
  now: number;
  reason?: string;
}): StopLimitOrder {
  return {
    id: args.id,
    strategyId: args.strategyId,
    strategyName: args.strategyName,
    symbol: args.symbol,
    side: args.side,
    orderType: "STOP_LIMIT",
    status: "OPEN",
    notionalUsd: args.notionalUsd,
    filledFraction: 0,
    avgFillPrice: 0,
    feesUsd: 0,
    createdAt: args.now,
    updatedAt: args.now,
    expireAt: args.expireAt ?? 0,
    stopPrice: args.stopPrice,
    limitPrice: args.limitPrice,
    triggered: false,
    reason: args.reason,
  };
}

export function createTrailingStopOrder(args: {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: MockSide;
  notionalUsd: number;
  /** Current mark price at time of order creation. */
  currentPrice: number;
  /** Trail in % of price (e.g. 0.5 = 0.5%). */
  trailPct?: number;
  /** Trail in USD (e.g. 100). */
  trailUsd?: number;
  expireAt?: number;
  now: number;
  reason?: string;
}): TrailingStopOrder {
  const trailDistance = _computeTrailDistance(args.currentPrice, args.trailPct, args.trailUsd);
  const currentStopPrice = args.side === "BUY"
    ? args.currentPrice + trailDistance
    : args.currentPrice - trailDistance;
  return {
    id: args.id,
    strategyId: args.strategyId,
    strategyName: args.strategyName,
    symbol: args.symbol,
    side: args.side,
    orderType: "TRAILING_STOP",
    status: "OPEN",
    notionalUsd: args.notionalUsd,
    filledFraction: 0,
    avgFillPrice: 0,
    feesUsd: 0,
    createdAt: args.now,
    updatedAt: args.now,
    expireAt: args.expireAt ?? 0,
    trailPct: args.trailPct,
    trailUsd: args.trailUsd,
    activationPrice: args.currentPrice,
    currentStopPrice,
    triggered: false,
    reason: args.reason,
  };
}

export function createOcoOrder(args: {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  /** Side of the underlying position (BUY = long position; TP sells, SL sells). */
  positionSide: MockSide;
  notionalUsd: number;
  takeProfitPrice: number;
  stopPrice: number;
  now: number;
  reason?: string;
}): OcoOrder {
  const closeSide: MockSide = args.positionSide === "BUY" ? "SELL" : "BUY";
  const ocoGroupId = `oco-${args.id}`;
  const tpLeg = createLimitOrder({
    id: `${args.id}-tp`,
    strategyId: args.strategyId,
    strategyName: args.strategyName,
    symbol: args.symbol,
    side: closeSide,
    notionalUsd: args.notionalUsd,
    limitPrice: args.takeProfitPrice,
    now: args.now,
    reason: "OCO TP leg",
  });
  const slLeg = createStopOrder({
    id: `${args.id}-sl`,
    strategyId: args.strategyId,
    strategyName: args.strategyName,
    symbol: args.symbol,
    side: closeSide,
    notionalUsd: args.notionalUsd,
    stopPrice: args.stopPrice,
    now: args.now,
    reason: "OCO SL leg",
  });
  return {
    id: args.id,
    strategyId: args.strategyId,
    strategyName: args.strategyName,
    symbol: args.symbol,
    side: closeSide,
    orderType: "OCO",
    status: "OPEN",
    notionalUsd: args.notionalUsd,
    filledFraction: 0,
    avgFillPrice: 0,
    feesUsd: 0,
    createdAt: args.now,
    updatedAt: args.now,
    expireAt: 0,
    ocoGroupId,
    takeProfitLeg: { ...tpLeg, ocoGroupId },
    stopLossLeg: { ...slLeg, ocoGroupId },
    reason: args.reason,
  };
}

// ── Price tick processing ─────────────────────────────────────────────────────

export interface OrderTickResult {
  order: AnyOrder;
  /** True when the order was triggered/filled during this tick. */
  triggered: boolean;
  /** True when the order was cancelled (OCO cancellation or expiry). */
  cancelled: boolean;
  /** Fill price if triggered. */
  fillPrice?: number;
}

/**
 * Apply a price tick to an individual order.
 * Returns a new order object (immutable update pattern).
 */
export function applyTickToOrder(order: AnyOrder, price: number, now: number): OrderTickResult {
  if (order.status === "FILLED" || order.status === "CANCELLED" || order.status === "EXPIRED" || order.status === "REJECTED") {
    return { order, triggered: false, cancelled: false };
  }
  // Expiry check
  if (order.expireAt > 0 && now >= order.expireAt) {
    return { order: { ...order, status: "EXPIRED", updatedAt: now }, triggered: false, cancelled: true };
  }

  switch (order.orderType) {
    case "MARKET":
      return _tickMarket(order as MarketOrder, price, now);
    case "LIMIT":
    case "POST_ONLY":
      return _tickLimit(order as LimitOrder, price, now);
    case "STOP":
      return _tickStop(order as StopOrder, price, now);
    case "STOP_LIMIT":
      return _tickStopLimit(order as StopLimitOrder, price, now);
    case "TRAILING_STOP":
      return _tickTrailingStop(order as TrailingStopOrder, price, now);
    case "OCO":
      return _tickOco(order as OcoOrder, price, now);
    case "REDUCE_ONLY":
      return _tickReduceOnly(order as ReduceOnlyOrder, price, now);
    default:
      return { order, triggered: false, cancelled: false };
  }
}

function _tickMarket(order: MarketOrder, price: number, now: number): OrderTickResult {
  const filled: MarketOrder = {
    ...order,
    status: "FILLED",
    filledFraction: 1,
    avgFillPrice: price,
    updatedAt: now,
  };
  return { order: filled, triggered: true, cancelled: false, fillPrice: price };
}

function _tickLimit(order: LimitOrder, price: number, now: number): OrderTickResult {
  const crosses = order.side === "BUY" ? price <= order.limitPrice : price >= order.limitPrice;
  if (!crosses) return { order, triggered: false, cancelled: false };
  const filled: LimitOrder = {
    ...order,
    status: "FILLED",
    filledFraction: 1,
    avgFillPrice: order.limitPrice,
    updatedAt: now,
  };
  return { order: filled, triggered: true, cancelled: false, fillPrice: order.limitPrice };
}

function _tickStop(order: StopOrder, price: number, now: number): OrderTickResult {
  if (order.triggered) {
    // Already triggered → immediate market fill
    const filled: StopOrder = { ...order, status: "FILLED", filledFraction: 1, avgFillPrice: price, updatedAt: now };
    return { order: filled, triggered: true, cancelled: false, fillPrice: price };
  }
  const triggered = order.side === "BUY" ? price >= order.stopPrice : price <= order.stopPrice;
  if (!triggered) return { order, triggered: false, cancelled: false };
  const next: StopOrder = { ...order, triggered: true, status: "TRIGGERED", updatedAt: now };
  return { order: next, triggered: false, cancelled: false };
}

function _tickStopLimit(order: StopLimitOrder, price: number, now: number): OrderTickResult {
  if (!order.triggered) {
    const hit = order.side === "BUY" ? price >= order.stopPrice : price <= order.stopPrice;
    if (!hit) return { order, triggered: false, cancelled: false };
    const next: StopLimitOrder = { ...order, triggered: true, status: "TRIGGERED", updatedAt: now };
    return { order: next, triggered: false, cancelled: false };
  }
  // Now a limit order
  const crosses = order.side === "BUY" ? price <= order.limitPrice : price >= order.limitPrice;
  if (!crosses) return { order, triggered: false, cancelled: false };
  const filled: StopLimitOrder = {
    ...order,
    status: "FILLED",
    filledFraction: 1,
    avgFillPrice: order.limitPrice,
    updatedAt: now,
  };
  return { order: filled, triggered: true, cancelled: false, fillPrice: order.limitPrice };
}

function _tickTrailingStop(order: TrailingStopOrder, price: number, now: number): OrderTickResult {
  if (order.triggered) {
    const filled: TrailingStopOrder = {
      ...order, status: "FILLED", filledFraction: 1, avgFillPrice: price, updatedAt: now,
    };
    return { order: filled, triggered: true, cancelled: false, fillPrice: price };
  }

  // Update trailing stop
  const trailDist = _computeTrailDistance(price, order.trailPct, order.trailUsd);
  let { activationPrice, currentStopPrice } = order;

  if (order.side === "SELL") {
    // Trailing stop for a LONG position: stop tracks upward
    if (price > activationPrice) {
      activationPrice = price;
      currentStopPrice = price - trailDist;
    }
    if (price <= currentStopPrice) {
      const triggered: TrailingStopOrder = {
        ...order, triggered: true, status: "TRIGGERED", activationPrice, currentStopPrice, updatedAt: now,
      };
      return { order: triggered, triggered: false, cancelled: false };
    }
  } else {
    // Trailing stop for a SHORT position: stop tracks downward
    if (price < activationPrice) {
      activationPrice = price;
      currentStopPrice = price + trailDist;
    }
    if (price >= currentStopPrice) {
      const triggered: TrailingStopOrder = {
        ...order, triggered: true, status: "TRIGGERED", activationPrice, currentStopPrice, updatedAt: now,
      };
      return { order: triggered, triggered: false, cancelled: false };
    }
  }

  const updated: TrailingStopOrder = { ...order, activationPrice, currentStopPrice, updatedAt: now };
  return { order: updated, triggered: false, cancelled: false };
}

function _tickOco(order: OcoOrder, price: number, now: number): OrderTickResult {
  const tpResult = _tickLimit(order.takeProfitLeg, price, now);
  const slResult = _tickStop(order.stopLossLeg, price, now);

  if (tpResult.triggered) {
    const filled: OcoOrder = {
      ...order,
      status: "FILLED",
      filledFraction: 1,
      avgFillPrice: tpResult.fillPrice ?? price,
      takeProfitLeg: tpResult.order as LimitOrder,
      stopLossLeg: { ...order.stopLossLeg, status: "CANCELLED", updatedAt: now },
      filledLeg: "TP",
      updatedAt: now,
    };
    return { order: filled, triggered: true, cancelled: false, fillPrice: tpResult.fillPrice };
  }
  if (slResult.triggered) {
    const filled: OcoOrder = {
      ...order,
      status: "FILLED",
      filledFraction: 1,
      avgFillPrice: slResult.fillPrice ?? price,
      takeProfitLeg: { ...order.takeProfitLeg, status: "CANCELLED", updatedAt: now },
      stopLossLeg: slResult.order as StopOrder,
      filledLeg: "SL",
      updatedAt: now,
    };
    return { order: filled, triggered: true, cancelled: false, fillPrice: slResult.fillPrice };
  }

  // Update sub-orders if stop was triggered (but not yet filled)
  const updatedOco: OcoOrder = {
    ...order,
    takeProfitLeg: tpResult.order as LimitOrder,
    stopLossLeg: slResult.order as StopOrder,
    updatedAt: now,
  };
  return { order: updatedOco, triggered: false, cancelled: false };
}

function _tickReduceOnly(order: ReduceOnlyOrder, price: number, now: number): OrderTickResult {
  const lp = order.limitPrice;
  if (lp == null) {
    // Market reduce-only
    const filled: ReduceOnlyOrder = {
      ...order, status: "FILLED", filledFraction: 1, avgFillPrice: price, updatedAt: now,
    };
    return { order: filled, triggered: true, cancelled: false, fillPrice: price };
  }
  const crosses = order.side === "BUY" ? price <= lp : price >= lp;
  if (!crosses) return { order, triggered: false, cancelled: false };
  const filled: ReduceOnlyOrder = {
    ...order, status: "FILLED", filledFraction: 1, avgFillPrice: lp, updatedAt: now,
  };
  return { order: filled, triggered: true, cancelled: false, fillPrice: lp };
}

// ── Order book tick ───────────────────────────────────────────────────────────

export interface OrderBookTickResult {
  book: MockOrderBook;
  fills: { orderId: string; fillPrice: number; notionalUsd: number }[];
  cancels: string[];
}

/**
 * Apply a price tick to the entire order book.
 * OCO cancellations are propagated automatically.
 * Returns a new book + events for the engine to act on.
 */
export function applyTickToOrderBook(book: MockOrderBook, price: number, now: number): OrderBookTickResult {
  const fills: OrderBookTickResult["fills"] = [];
  const cancels: string[] = [];
  const cancelledOcoGroups = new Set<string>();

  const nextOrders: AnyOrder[] = [];
  for (const order of book.orders) {
    const result = applyTickToOrder(order, price, now);
    nextOrders.push(result.order);
    if (result.triggered) {
      fills.push({
        orderId: result.order.id,
        fillPrice: result.fillPrice ?? price,
        notionalUsd: result.order.notionalUsd,
      });
      if (result.order.ocoGroupId) cancelledOcoGroups.add(result.order.ocoGroupId);
    }
    if (result.cancelled) cancels.push(result.order.id);
  }

  // Apply OCO cancellations — cancel any OPEN order in the same group
  const finalOrders = nextOrders.map((order) => {
    if (
      order.ocoGroupId &&
      cancelledOcoGroups.has(order.ocoGroupId) &&
      order.status === "OPEN"
    ) {
      cancels.push(order.id);
      return { ...order, status: "CANCELLED" as OrderStatus, updatedAt: now };
    }
    return order;
  });

  return { book: { ...book, orders: finalOrders }, fills, cancels };
}

/** Cancel an order by ID. Returns a new book. */
export function cancelOrder(book: MockOrderBook, orderId: string, now: number): MockOrderBook {
  return {
    ...book,
    orders: book.orders.map((o) =>
      o.id === orderId && o.status === "OPEN"
        ? { ...o, status: "CANCELLED" as OrderStatus, updatedAt: now }
        : o,
    ),
  };
}

/** Add an order to the book. */
export function addOrder(book: MockOrderBook, order: AnyOrder): MockOrderBook {
  return { ...book, orders: [...book.orders, order] };
}

/** Return only OPEN orders for a given side. */
export function openOrders(book: MockOrderBook, side?: MockSide): AnyOrder[] {
  return book.orders.filter((o) => o.status === "OPEN" && (side == null || o.side === side));
}

// ── Trailing stop utilities ───────────────────────────────────────────────────

function _computeTrailDistance(price: number, trailPct?: number, trailUsd?: number): number {
  if (Number.isFinite(trailUsd) && (trailUsd ?? 0) > 0) return trailUsd as number;
  if (Number.isFinite(trailPct) && (trailPct ?? 0) > 0) return price * ((trailPct as number) / 100);
  return price * 0.005; // default 0.5%
}

/**
 * Update a trailing stop order's activation and stop prices given a new mark.
 * Returns a new order if the stop moved; same reference otherwise.
 */
export function updateTrailingStop(order: TrailingStopOrder, price: number, now: number): TrailingStopOrder {
  const trailDist = _computeTrailDistance(price, order.trailPct, order.trailUsd);
  if (order.side === "SELL") {
    if (price > order.activationPrice) {
      return { ...order, activationPrice: price, currentStopPrice: price - trailDist, updatedAt: now };
    }
  } else {
    if (price < order.activationPrice) {
      return { ...order, activationPrice: price, currentStopPrice: price + trailDist, updatedAt: now };
    }
  }
  return order;
}

// ── Summary helpers ───────────────────────────────────────────────────────────

export interface OrderBookSummary {
  total: number;
  open: number;
  filled: number;
  cancelled: number;
  expired: number;
  rejected: number;
  pendingByType: Record<OrderType, number>;
}

export function summarizeOrderBook(book: MockOrderBook): OrderBookSummary {
  const pendingByType = {} as Record<OrderType, number>;
  let open = 0, filled = 0, cancelled = 0, expired = 0, rejected = 0;
  for (const o of book.orders) {
    if (o.status === "OPEN" || o.status === "TRIGGERED") {
      open++;
      pendingByType[o.orderType] = (pendingByType[o.orderType] ?? 0) + 1;
    } else if (o.status === "FILLED" || o.status === "PARTIAL") filled++;
    else if (o.status === "CANCELLED") cancelled++;
    else if (o.status === "EXPIRED") expired++;
    else if (o.status === "REJECTED") rejected++;
  }
  return { total: book.orders.length, open, filled, cancelled, expired, rejected, pendingByType };
}
