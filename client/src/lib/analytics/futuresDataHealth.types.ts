/** Milliseconds non-`ok` before desks show the subtle feed warning (tunable).
 *
 * Dev: set `NEXT_PUBLIC_SIMULATE_FUTURES_502=1` so the client appends `debugFutures502=1` to
 * `/api/btc/futures-klines` requests (the Next route returns 502 in development only).
 */
export const FUTURES_FEED_WARNING_AFTER_MS = 15_000;

export type FuturesDataHealthStatus = "ok" | "degraded" | "stale";

export type FuturesDataHealthSymbolIssue = {
  symbol: string;
  reason: "fetch_failed" | "http_error" | "payload_not_ok" | "insufficient_bars";
  detail?: string;
};

export type FuturesDataHealth = {
  status: FuturesDataHealthStatus;
  lastError: string | null;
  /** Last poll time when at least one symbol had ≥ min bars (same gate as `hasMarketData`). */
  lastOkAt: number | null;
  lastPollAt: number;
  /** Symbols requested this poll that did not land in the ready payload map. */
  failingSymbols: readonly string[];
  symbolIssues: readonly FuturesDataHealthSymbolIssue[];
  payloadsReady: number;
  symbolsRequested: number;
  /** True when status has been non-`ok` continuously for ≥ {@link FUTURES_FEED_WARNING_AFTER_MS} (evaluated each poll). */
  showFeedWarning: boolean;
};
