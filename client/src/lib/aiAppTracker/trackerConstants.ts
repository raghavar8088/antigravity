/** Thresholds used by the AI App Tracker. */

/** Worker heartbeat older than this → stale_worker warning. */
export const STALE_WORKER_THRESHOLD_MS = 90_000;

/** No new trade opened in this window → no_trades_30min warning. */
export const NO_TRADE_WINDOW_MS = 30 * 60 * 1000;

/** Balance deviation from day-start balance above this USD → balance_drift warning. */
export const BALANCE_DRIFT_WARN_USD = 50;

/** Fee/gross ratio above this percent → high_fee_gross warning. */
export const HIGH_FEE_GROSS_PCT = 80;

/** Funnel snapshot older than this → treat as stale. */
export const STALE_FUNNEL_THRESHOLD_MS = 120_000;

/** Deployment build SHA mismatch older than this → stale_deployment warning. */
export const STALE_DEPLOY_WARN_WINDOW_MS = 12 * 60 * 60 * 1000;

/** MongoDB collection name for tracker reports. */
export const TRACKER_COLLECTION = "ai_app_tracker_reports";

/** Module key written to every report. */
export const TRACKER_MODULE = "btc_future_trading" as const;

/** Report retention TTL (30 days in seconds) for Mongo TTL index. */
export const TRACKER_TTL_SECONDS = 30 * 24 * 3600;
