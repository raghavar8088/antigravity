/**
 * randomTrader.ts — Random Trade Scheduler core logic.
 *
 * Pure functions + shared types for the every-minute random BTC trade system.
 * No strategy filters, no confluence rules, no cooldowns.
 *
 * Paper mode (default): records trades in MongoDB `random_trades` collection.
 * Live mode (LIVE_TRADING_ENABLED=true): places real market orders on Delta Exchange.
 *
 * NEVER modifies the existing strategy engine or paper_trades collection.
 */

// ── Types ──────────────────────────────────────────────────────────────────

export type RandomTradeSide = "BUY" | "SELL";
export type RandomTradeMode = "paper" | "live";
export type RandomTradeStatus = "open";

export type RandomTradeRecord = {
  /** Internal unique ID (rt-<timestamp>-<random>). */
  id: string;
  account_key: string;
  side: RandomTradeSide;
  btc_price: number;
  /** BTC quantity. */
  quantity: number;
  /** USD notional = quantity × btc_price. */
  notional_usd: number;
  mode: RandomTradeMode;
  status: RandomTradeStatus;
  opened_at: string;
  /** Broker/exchange order ID (live mode). Null in paper mode. */
  broker_order_id: string | null;
  /** Full broker response JSON (stringified). */
  broker_response: string | null;
  /** TTL anchor for MongoDB expiry. */
  created_at: Date;
};

export type TickLog = {
  timestamp: string;
  btcPrice: number;
  side: RandomTradeSide;
  quantity: number;
  notionalUsd: number;
  mode: RandomTradeMode;
  openTradesCount: number;
  maxOpenTrades: number;
  /** True when the trade was skipped (at cap, price fetch failed, etc.). */
  skipped: boolean;
  skipReason: string | null;
  brokerOrderId: string | null;
  /** Full broker response JSON string. */
  brokerResponse: string | null;
  error: string | null;
};

export type TickResult = {
  ok: boolean;
  log: TickLog;
};

export type RandomTraderStatus = {
  mode: RandomTradeMode;
  liveEnabled: boolean;
  openTradesCount: number;
  maxOpenTrades: number;
  tradeSizeUsd: number;
  btcPrice: number | null;
  lastTrade: {
    side: RandomTradeSide;
    btcPrice: number;
    quantity: number;
    openedAt: string;
    brokerOrderId: string | null;
  } | null;
  priceSource: string;
};

// ── Env helpers (server-side only) ─────────────────────────────────────────

/** Max concurrent open random trades. Env: RANDOM_TRADER_MAX_OPEN. Default 100. */
export function maxOpenTradesFromEnv(): number {
  const raw = process.env.RANDOM_TRADER_MAX_OPEN;
  if (!raw) return 100;
  const n = parseInt(raw, 10);
  return Number.isFinite(n) && n > 0 ? Math.min(10_000, n) : 100;
}

/** Trade size in USD per order. Env: RANDOM_TRADER_SIZE_USD. Default 100. */
export function tradeSizeUsdFromEnv(): number {
  const raw = process.env.RANDOM_TRADER_SIZE_USD;
  if (!raw) return 100;
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? Math.min(100_000, Math.max(10, n)) : 100;
}

/** True only when LIVE_TRADING_ENABLED=true. Default false (paper mode). */
export function isLiveTradingEnabled(): boolean {
  return process.env.LIVE_TRADING_ENABLED === "true";
}

import { resolveAuthoritativeAccountKey } from "../broker/authoritativeAccountKey";

/** Account key for MongoDB storage — always the authoritative owner key. */
export function randomTraderAccountKey(): string {
  return (
    process.env.RANDOM_TRADER_ACCOUNT_KEY?.trim() ||
    resolveAuthoritativeAccountKey()
  );
}

// ── Pure helpers ──────────────────────────────────────────────────────────

export function pickRandomSide(): RandomTradeSide {
  return Math.random() < 0.5 ? "BUY" : "SELL";
}

export function generateTradeId(): string {
  return `rt-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

/**
 * BTC quantity to buy/sell for a given USD notional and price.
 * Rounded to 4 decimal places (0.0001 BTC minimum).
 */
export function quantityFromUsd(usdAmount: number, btcPrice: number): number {
  if (!Number.isFinite(btcPrice) || btcPrice <= 0) return 0;
  return Math.max(0.0001, Math.round((usdAmount / btcPrice) * 10_000) / 10_000);
}

/**
 * Format a TickLog as a single-line string for console output.
 *
 * Example:
 *   [2026-05-27T14:01:00.000Z] BUY 0.0010 BTC @ $100,500 ($100 notional) — paper | open=5/100
 */
export function formatTickLog(log: TickLog): string {
  if (log.skipped) {
    return `[${log.timestamp}] SKIPPED — ${log.skipReason ?? "unknown"} | open=${log.openTradesCount}/${log.maxOpenTrades} mode=${log.mode}`;
  }
  if (log.error) {
    return `[${log.timestamp}] ERROR — ${log.error} | side=${log.side} price=$${log.btcPrice} mode=${log.mode}`;
  }
  const price = log.btcPrice.toLocaleString("en-US", { maximumFractionDigits: 2 });
  const notional = log.notionalUsd.toFixed(2);
  const orderId = log.brokerOrderId ? ` orderId=${log.brokerOrderId}` : "";
  return `[${log.timestamp}] ${log.side} ${log.quantity.toFixed(4)} BTC @ $${price} ($${notional} notional) — ${log.mode} | open=${log.openTradesCount}/${log.maxOpenTrades}${orderId}`;
}

/** MongoDB collection name for random trades. */
export const RANDOM_TRADES_COLLECTION = "random_trades";

/** TTL for random trade records: 90 days. */
export const RANDOM_TRADES_TTL_SECONDS = 90 * 24 * 60 * 60;
