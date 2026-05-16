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
] as const;

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
};

export const paperTradeGetQuerySchema = z.object({
  account_key: z.string().min(1).max(128).optional(),
  limit: z.coerce.number().int().min(1).max(500).default(100),
  cursor: z.string().datetime().optional(),
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
