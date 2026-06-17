/**
 * Dry-run signal trace for diagnostics — evaluates strategies without opening trades.
 */
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";
import { mockConfigForStage } from "@/lib/strategyAuthority/gradeStageMockConfig";
import {
  fetchMockTradingKlines,
  sanitizeMockTradingSymbol,
} from "@/lib/trading/mockTradingMarketData";
import {
  computeAccountState,
  isExecutableTraceRow,
} from "@/lib/trading/mockTradingEngine";
import { getLatestMockAccountSnapshot, listMockTrades } from "@/lib/trading/mockTradingMongo";
import { evaluateMockTradingTick } from "@/lib/mockTradingExecutor/evaluateMockTradingTick";
import { funnelSnapshotFromTraceEval } from "@/lib/mockTradingExecutor/funnelFromTrace";

export type MockSignalTraceDiagnostic = {
  ok: boolean;
  accountKey: string;
  timestamp: number;
  symbol: string;
  markPrice: number;
  regime: string;
  bars: number;
  dataFresh: boolean;
  marketError: string | null;
  candidateCount: number;
  executableCount: number;
  blockerSummary: Record<string, number>;
  dominantBlocker: string;
  funnel: ReturnType<typeof funnelSnapshotFromTraceEval> | null;
  summary: ReturnType<typeof evaluateMockTradingTick>["summary"];
  rows: ReturnType<typeof evaluateMockTradingTick>["rows"];
};

const STALE_MS = 30_000;

export async function collectMockSignalTraceDiagnostic(args?: {
  accountKey?: string;
  symbol?: string;
}): Promise<MockSignalTraceDiagnostic> {
  const accountKey = args?.accountKey?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
  const symbol = sanitizeMockTradingSymbol(args?.symbol ?? "BTCUSD");
  const timestamp = Date.now();

  const base: MockSignalTraceDiagnostic = {
    ok: false,
    accountKey,
    timestamp,
    symbol,
    markPrice: 0,
    regime: "unknown",
    bars: 0,
    dataFresh: false,
    marketError: null,
    candidateCount: 0,
    executableCount: 0,
    blockerSummary: {},
    dominantBlocker: "noData",
    funnel: null,
    summary: {
      tickAt: timestamp,
      totalEvaluated: 0,
      fired: 0,
      candidates: 0,
      opened: 0,
      rejectedByGate: {},
      topRejectedGate: null,
      candidateCount: 0,
      rejectedCount: 0,
      topGates: [],
    },
    rows: [],
  };

  try {
    const market = await fetchMockTradingKlines(symbol);
    const lastBar = market.bars[market.bars.length - 1];
    const lastBarMs = (lastBar?.time ?? 0) * 1000;
    const dataFresh = lastBarMs > 0 && timestamp - lastBarMs <= STALE_MS;

    const evalResult = evaluateMockTradingTick({
      bars: market.bars,
      markPrice: market.markPrice,
      symbol,
      tickAt: timestamp,
    });

    const executableCount = evalResult.rows.filter((r) => isExecutableTraceRow(r)).length;
    const blockerSummary: Record<string, number> = { ...evalResult.summary.rejectedByGate };
    if (executableCount > 0) {
      blockerSummary.CANDIDATE = executableCount;
    }

    const funnel = funnelSnapshotFromTraceEval({
      tickAt: timestamp,
      symbol,
      markPrice: market.markPrice,
      bars: market.bars.length,
      activeStrategies: evalResult.activeStrategies,
      evaluatedStrategies: evalResult.evaluatedStrategies,
      rows: evalResult.rows,
      candidateCount: evalResult.candidateCount,
      opened: 0,
      workerFresh: dataFresh,
    });

    return {
      ok: !evalResult.error && dataFresh,
      accountKey,
      timestamp,
      symbol,
      markPrice: market.markPrice,
      regime: evalResult.regime,
      bars: market.bars.length,
      dataFresh,
      marketError: evalResult.error,
      candidateCount: evalResult.candidateCount,
      executableCount,
      blockerSummary,
      dominantBlocker: funnel.dominantBlocker,
      funnel,
      summary: evalResult.summary,
      rows: evalResult.rows,
    };
  } catch (err) {
    return {
      ...base,
      marketError: err instanceof Error ? err.message : "MARKET_FETCH_FAILED",
    };
  }
}

export async function collectMockAccountRiskContext(accountKey: string) {
  const { config: storedConfig } = await getLatestMockAccountSnapshot(accountKey);
  const config = storedConfig ?? mockConfigForStage("MAIN_ENGINE");
  const [openList, closedList] = await Promise.all([
    listMockTrades({ account_key: accountKey, page: 1, limit: 200, status: "OPEN", sort: "newest" }),
    listMockTrades({ account_key: accountKey, page: 1, limit: 50, status: "CLOSED", sort: "newest" }),
  ]);
  const account = computeAccountState([...openList.trades, ...closedList.trades], config);
  return { config, account, openCount: openList.trades.length };
}
