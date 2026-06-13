import { z } from "zod";

/** Max closed trades kept in React state / localStorage mirror. */
export const PAPER_TRADES_MAX_LOCAL = 2_000;

export const PAPER_TRADE_EXIT_REASONS = [
  "TP",
  "SL",
  "TIME",
  "TRAIL",
  "BREAKEVEN",
  "LIQUIDATION_RISK",
  "PROFIT_LOCK",
  "MOM_DECAY",
] as const;

/**
 * Module identifier persisted alongside each paper trade so the dashboard can
 * filter leaderboards, exports, and analytics per workspace tab.
 *
 * Stable string keys (never renumber — they become DB column values).
 */
export const PAPER_TRADE_MODULE_KEYS = [
  "btc_future_trading",   // BTC Future Trading (specific BTC FT desk)
  "btc_futures_scalper",  // Future Trading (multi-symbol futures scalper)
  "btc_option_buying",    // BTC Option Buying (OptionsScalper)
  "btc_option_selling",   // BTC Option Selling (OptionsSellingScalper)
] as const;

export type PaperTradeModuleKey = (typeof PAPER_TRADE_MODULE_KEYS)[number];

export const paperTradeModuleKeySchema = z.enum(PAPER_TRADE_MODULE_KEYS);

export const paperTradeSideSchema = z.enum(["LONG", "SHORT"]);
export const paperTradeExitReasonSchema = z.enum(PAPER_TRADE_EXIT_REASONS);

/** Client → API POST body (subset of BTCFuturesTrade + account). */
export const paperTradeClientSchema = z.object({
  clientTradeId: z.string().uuid(),
  id: z.string().min(1).max(256),
  symbol: z.string().min(1).max(32),
  strategyId: z.number().int().nonnegative(),
  strategyName: z.string().min(1).max(256),
  side: paperTradeSideSchema,
  entryPrice: z.number().finite(),
  exitPrice: z.number().finite(),
  contracts: z.number().finite().nonnegative(),
  notional: z.number().finite().nonnegative(),
  marginUsed: z.number().finite().nonnegative(),
  realizedPnl: z.number().finite(),
  fees: z.number().finite().nonnegative(),
  netPnl: z.number().finite(),
  netPnlPct: z.number().finite(),
  priceMovePct: z.number().finite(),
  fundingCosts: z.number().finite(),
  openedAt: z.string().datetime(),
  closedAt: z.string().datetime(),
  exitReason: paperTradeExitReasonSchema,
  liquidationPrice: z.number().finite(),
  liquidationDistancePct: z.number().finite(),
  lastFundingAppliedAt: z.number().finite().optional(),
  fundingSinceOpenMs: z.number().finite().nonnegative().optional(),
});

export const paperTradePostBodySchema = z.object({
  /** Ignored when authenticated — server sets `account_key` from session user id. */
  accountKey: z.string().min(1).max(128).optional(),
  /** Optional module identifier — persisted with the row so dashboards can filter per-tab. */
  moduleKey: paperTradeModuleKeySchema.optional(),
  /**
   * Parent algorithm family (e.g. "momentum_reversion", "vol_breakout").
   * Used for cross-strategy grouping in research leaderboards; more stable than
   * individual strategy IDs which may be renumbered.
   */
  templateFamily: z.string().min(1).max(128).optional(),
  /** Exchange where the simulated execution occurred (e.g. "binance", "delta", "nse"). */
  exchange: z.string().min(1).max(64).optional(),
  /**
   * Target MongoDB collection. Generated when user clears trade history in the UI
   * (format: paper_trades_YYYYMMDD_HHMMSS). Defaults to "paper_trades".
   * Server validates the name to prevent injection.
   */
  collectionName: z.string().min(1).max(80).regex(/^(paper_trades|loop_trades)(_[a-z0-9_]+)?$/).optional(),
  trade: paperTradeClientSchema,
});

export type PaperTradeClientPayload = z.infer<typeof paperTradeClientSchema>;
export type PaperTradePostBody = z.infer<typeof paperTradePostBodySchema>;

/** Row shape returned from Supabase `paper_trades`. */
export type PaperTradeDbRow = {
  id: string;
  created_at: string;
  account_key: string;
  client_trade_id: string;
  opened_at: string;
  closed_at: string;
  symbol: string;
  strategy_id: number;
  strategy_name: string;
  side: string;
  entry_price: number;
  exit_price: number;
  contracts: number;
  notional: number;
  margin_used: number;
  gross_pnl: number;
  fees: number;
  funding_costs: number;
  net_pnl: number;
  exit_reason: string;
  payload: unknown;
  /** Module identifier (added in v3 schema). */
  module_key?: string | null;
  /**
   * Parent algorithm family (added in v4 schema). Groups related strategy IDs
   * for cross-strategy leaderboards and production gating.
   */
  template_family?: string | null;
  /** Exchange where execution was simulated (e.g. "binance", "delta", "nse"). */
  exchange?: string | null;
};

export const paperTradeGetQuerySchema = z.object({
  account_key: z.string().min(1).max(128).optional(),
  limit: z.coerce.number().int().min(1).max(500).default(100),
  cursor: z.string().datetime().optional(),
  /** Filter by module (e.g. only show trades for the BTC Option Buying desk). */
  module_key: paperTradeModuleKeySchema.optional(),
});

export const paperTradeStrategyStatsQuerySchema = z.object({
  account_key: z.string().min(1).max(128).optional(),
  window_days: z.coerce.number().int().min(1).max(90).default(14),
});

export const paperTradeLeaderboardQuerySchema = z.object({
  account_key: z.string().min(1).max(128).optional(),
  window_days: z.coerce.number().int().min(1).max(90).default(30),
  limit: z.coerce.number().int().min(1).max(50).default(15),
});

export const paperTradeExportQuerySchema = z.object({
  account_key: z.string().min(1).max(128).optional(),
  window_days: z.coerce.number().int().min(1).max(90).default(30),
});

/**
 * Query schema for the `/api/paper-trades/analytics` aggregation endpoint.
 * Supports filtering by module and/or templateFamily for cross-desk research.
 */
export const paperTradeAnalyticsQuerySchema = z.object({
  account_key: z.string().min(1).max(128).optional(),
  window_days: z.coerce.number().int().min(1).max(365).default(30),
  module_key: paperTradeModuleKeySchema.optional(),
  /** Filter to a specific templateFamily (e.g. "momentum_reversion"). */
  template_family: z.string().min(1).max(128).optional(),
  /**
   * Minimum closed trades before a strategy is included in results.
   * Default 100 — the statistical threshold for reliable expectancy.
   */
  min_trades: z.coerce.number().int().min(1).max(10000).default(100),
});
