/**
 * Verification Track — append-only observability for the BTC futures paper desk.
 *
 * Purpose: Give Cursor AI (and human operators) a compact, factual history of
 * signals, gates, decisions, worker health, repairs, and outcomes so future
 * debugging requires less re-discovery and fewer tokens.
 *
 * Hard rules:
 * - Paper only. Never records live trading actions.
 * - Never stores full account_key — only a safe suffix.
 * - Never stores API keys, secrets, or sensitive payloads.
 * - Writes are best-effort and non-fatal; trading continues if verification fails.
 * - TTL: events auto-expire after 30 days.
 */

export type VerificationTrackEventType =
  | "WORKER_TICK"
  | "MARKET_DATA"
  | "STRATEGY_EVALUATED"
  | "SIGNAL_FIRED"
  | "SIGNAL_REJECTED"
  | "CANDIDATE_BUILT"
  | "OPEN_ATTEMPTED"
  | "POSITION_OPENED"
  | "POSITION_CLOSED"
  | "ENTRY_FUNNEL"
  | "SIGNAL_TRACE"
  | "NO_TRADE_ROOT_CAUSE"
  | "ROTATION_DECISION"
  | "REPAIR_STATE"
  | "WORKER_HEALTH"
  | "CRON_BACKUP"
  | "OMS_ORDER_NEW"
  | "OMS_ORDER_REJECTED"
  | "OMS_ORDER_FILLED"
  | "OMS_POSITION_OPENED"
  | "OMS_POSITION_CLOSED"
  | "REPLAY_WALKFORWARD_RUN"
  | "ERROR";

export type VerificationSeverity = "info" | "warning" | "danger";

export interface VerificationTrackEvent {
  event_id: string;                 // unique, e.g. `${tickAt}-${type}-${random}`
  created_at: string;               // ISO string
  module: "btc_future_trading";
  account_key_suffix: string | null; // last 4–8 chars only — never full key
  worker_owner?: "browser" | "vps" | "cron" | null;
  build_sha?: string | null;

  type: VerificationTrackEventType;
  severity: VerificationSeverity;
  summary: string;                  // human-readable one-liner

  // Optional context fields (populated when relevant)
  symbol?: string;
  strategy_id?: number;
  strategy_name?: string;
  side?: "LONG" | "SHORT";

  signal_score?: number;
  required_threshold?: number;
  gate?: string;
  reject_reason?: string;
  dominant_blocker?: string;

  position_id?: string;
  trade_id?: string;
  net_pnl?: number;
  exit_reason?: string;

  // Compact snapshot (never full raw payload)
  snapshot?: Record<string, unknown>;
}
