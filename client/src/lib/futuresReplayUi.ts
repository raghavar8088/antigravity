import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
import type { PaperReplayStats } from "@/lib/futuresReplayEngine";

export type DeskReplayFixtureKind = "sample" | "live";

export type DeskReplayFetchParams = {
  symbol?: string;
  bars?: number;
  fixture?: DeskReplayFixtureKind;
  slippageBps?: number;
  volSized?: boolean;
  drawdownLock?: boolean;
  autoDisable?: boolean;
  accountKey?: string;
};

export type ReplayTradeTableRow = {
  closedAt: string;
  strategyName: string;
  side: string;
  netPnl: number;
  exitReason: string;
};

export type PaperReplayApiSuccess = {
  symbol: string;
  bars: number;
  trades: BTCFuturesTrade[];
  stats: PaperReplayStats;
  finalBalance: number;
};

export type ReplaySummaryView = {
  tradeCount: number;
  sumNet: number;
  expectancy: number;
  exitReasonCounts: Record<string, number>;
  exitReasonLine: string;
};

/** Client may call replay API in dev or when explicitly enabled for staging. */
export function isDeskReplayUiEnabled(): boolean {
  return process.env.NODE_ENV === "development" || process.env.NEXT_PUBLIC_DESK_REPLAY_UI === "1";
}

export function isPaperReplayApiAllowed(): boolean {
  return isDeskReplayUiEnabled();
}

export function buildDeskReplaySearchParams(params: DeskReplayFetchParams): URLSearchParams {
  const q = new URLSearchParams();
  q.set("symbol", (params.symbol ?? "BTCUSD").toUpperCase());
  q.set("bars", String(params.bars ?? 500));
  q.set("fixture", params.fixture ?? "live");
  if (params.slippageBps !== undefined) q.set("slippageBps", String(params.slippageBps));
  if (params.volSized) q.set("volSized", "1");
  if (params.drawdownLock) q.set("drawdownLock", "1");
  if (params.autoDisable) q.set("autoDisable", "1");
  if (params.accountKey) q.set("account_key", params.accountKey);
  return q;
}

export function mapReplayTradesToTableRows(trades: ReadonlyArray<BTCFuturesTrade>): ReplayTradeTableRow[] {
  return [...trades]
    .sort((a, b) => new Date(b.closedAt).getTime() - new Date(a.closedAt).getTime())
    .map((t) => ({
      closedAt: t.closedAt,
      strategyName: t.strategyName,
      side: t.side,
      netPnl: t.netPnl,
      exitReason: t.exitReason,
    }));
}

export function formatReplaySummary(stats: PaperReplayStats): ReplaySummaryView {
  const exitReasonLine = Object.entries(stats.exitReasonCounts)
    .sort((a, b) => b[1] - a[1])
    .map(([reason, count]) => `${reason}×${count}`)
    .join(" · ");
  return {
    tradeCount: stats.count,
    sumNet: stats.sumNet,
    expectancy: stats.expectancy,
    exitReasonCounts: stats.exitReasonCounts,
    exitReasonLine: exitReasonLine || "—",
  };
}

function isReplayStats(v: unknown): v is PaperReplayStats {
  if (!v || typeof v !== "object") return false;
  const s = v as PaperReplayStats;
  return (
    typeof s.count === "number" &&
    typeof s.sumNet === "number" &&
    typeof s.expectancy === "number" &&
    s.exitReasonCounts !== null &&
    typeof s.exitReasonCounts === "object"
  );
}

function isReplayTrade(v: unknown): v is BTCFuturesTrade {
  if (!v || typeof v !== "object") return false;
  const t = v as BTCFuturesTrade;
  return (
    typeof t.closedAt === "string" &&
    typeof t.strategyName === "string" &&
    typeof t.side === "string" &&
    typeof t.netPnl === "number" &&
    typeof t.exitReason === "string"
  );
}

export function parsePaperReplayApiResponse(
  body: unknown,
): { ok: true; data: PaperReplayApiSuccess } | { ok: false; error: string } {
  if (!body || typeof body !== "object") {
    return { ok: false, error: "Invalid response" };
  }
  const b = body as { ok?: boolean; error?: string; trades?: unknown; stats?: unknown };
  if (b.ok !== true) {
    return { ok: false, error: typeof b.error === "string" ? b.error : "Replay failed" };
  }
  if (!Array.isArray(b.trades) || !isReplayStats(b.stats)) {
    return { ok: false, error: "Malformed replay payload" };
  }
  if (!b.trades.every(isReplayTrade)) {
    return { ok: false, error: "Malformed trade rows" };
  }
  const symbol = typeof (body as { symbol?: string }).symbol === "string" ? (body as { symbol: string }).symbol : "BTCUSD";
  const bars = typeof (body as { bars?: number }).bars === "number" ? (body as { bars: number }).bars : 0;
  const finalBalance =
    typeof (body as { finalBalance?: number }).finalBalance === "number"
      ? (body as { finalBalance: number }).finalBalance
      : 0;
  return {
    ok: true,
    data: {
      symbol,
      bars,
      trades: b.trades,
      stats: b.stats,
      finalBalance,
    },
  };
}

export const REPLAY_EMPTY_FIXTURE_HINT =
  "No live fixture loaded. From client/: npm run replay:fetch -- --symbol=BTCUSD --bars=500";

export function replayErrorWithFixtureHint(error: string, fixture: DeskReplayFixtureKind): string {
  if (fixture === "live" && /fixture|ENOENT|not found|load/i.test(error)) {
    return `${error}\n${REPLAY_EMPTY_FIXTURE_HINT}`;
  }
  return error;
}
