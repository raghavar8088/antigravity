import { isMongoConfigured, upsertSignalTrace } from "@/lib/broker/mongoTradesClient";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";
import { mockConfigForStage } from "@/lib/strategyAuthority/gradeStageMockConfig";
import {
  fetchMockTradingKlines,
  sanitizeMockTradingSymbol,
} from "@/lib/trading/mockTradingMarketData";
import {
  getLatestMockAccountSnapshot,
  insertMockAccountSnapshot,
  listMockTrades,
} from "@/lib/trading/mockTradingMongo";
import { computeAccountState, type MockTrade } from "@/lib/trading/mockTradingEngine";
import { evaluateMockTradingTick } from "@/lib/mockTradingExecutor/evaluateMockTradingTick";
import { loadExecutorThresholdConfig } from "@/lib/mockTradingExecutor/executorConfig";
import { funnelSnapshotFromTraceEval } from "@/lib/mockTradingExecutor/funnelFromTrace";
import {
  closeOpenTradesAtPrice,
  openTradesFromTraceRows,
} from "@/lib/mockTradingExecutor/processTrades";
import {
  ensureExecutorIndexes,
  persistExecutorCycle,
} from "@/lib/mockTradingExecutor/persistExecutorState";
import type {
  MockExecutorCycleResult,
  MockExecutorMode,
} from "@/lib/mockTradingExecutor/types";

export type RunMockTradingExecutorCycleArgs = {
  accountKey?: string;
  symbol?: string;
  mode?: MockExecutorMode;
  now?: number;
};

export async function runMockTradingExecutorCycle(
  args: RunMockTradingExecutorCycleArgs = {},
): Promise<MockExecutorCycleResult> {
  const start = Date.now();
  const tickAt = args.now ?? start;
  const accountKey = args.accountKey?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
  const mode: MockExecutorMode = args.mode ?? "worker";
  const symbol = sanitizeMockTradingSymbol(args.symbol ?? "BTCUSD");
  const errors: string[] = [];

  const emptyResult = (partial?: Partial<MockExecutorCycleResult["diagnostics"]>): MockExecutorCycleResult => ({
    ok: false,
    accountKey,
    mode,
    opened: [],
    closed: [],
    updatedOpen: 0,
    funnel: null,
    diagnostics: {
      tickAt,
      durationMs: Date.now() - start,
      errors,
      markPrice: 0,
      regime: "unknown",
      bars: 0,
      candidateCount: 0,
      evaluatedStrategies: 0,
      activeStrategies: 0,
      dominantBlocker: "noData",
      blockerCounts: {},
      signalSummary: null,
      ...partial,
    },
  });

  if (!isMongoConfigured()) {
    errors.push("MONGO_NOT_CONFIGURED");
    return emptyResult();
  }

  await ensureExecutorIndexes();

  const [{ config: storedConfig }, thresholdBundle] = await Promise.all([
    getLatestMockAccountSnapshot(accountKey),
    loadExecutorThresholdConfig(accountKey),
  ]);
  const baseConfig = storedConfig ?? mockConfigForStage("MAIN_ENGINE");
  const config = {
    ...baseConfig,
    minSignalScore: thresholdBundle.doc.min_signal_score,
  };
  const signalThreshold = thresholdBundle.doc.signal_threshold;

  let markPrice = 0;
  let bars = 0;
  let evalResult: ReturnType<typeof evaluateMockTradingTick> | null = null;

  try {
    const market = await fetchMockTradingKlines(symbol);
    markPrice = market.markPrice;
    bars = market.bars.length;
    evalResult = evaluateMockTradingTick({
      bars: market.bars,
      markPrice,
      symbol,
      tickAt,
      signalThreshold,
    });

    if (evalResult.error) errors.push(evalResult.error);

    await upsertSignalTrace(accountKey, {
      tickAt: evalResult.tickAt,
      mode: "worker",
      symbol: evalResult.symbol,
      summary: evalResult.summary,
      rows: evalResult.rows,
    });
  } catch (err) {
    errors.push(err instanceof Error ? err.message : "MARKET_FETCH_FAILED");
    const result = emptyResult({ errors: [...errors] });
    await persistExecutorCycle(result);
    return result;
  }

  const exitResult = await closeOpenTradesAtPrice({
    accountKey,
    markPrice,
    config,
    now: tickAt,
  });

  const entryResult = await openTradesFromTraceRows({
    accountKey,
    rows: evalResult.rows,
    markPrice,
    config,
    now: tickAt,
  });

  const funnel = funnelSnapshotFromTraceEval({
    tickAt: evalResult.tickAt,
    symbol: evalResult.symbol,
    markPrice,
    bars,
    activeStrategies: evalResult.activeStrategies,
    evaluatedStrategies: evalResult.evaluatedStrategies,
    rows: evalResult.rows,
    candidateCount: evalResult.candidateCount,
    opened: entryResult.opened.length,
    workerFresh: true,
  });

  const blockerCounts: Record<string, number> = {};
  for (const [gate, count] of Object.entries(evalResult.summary.rejectedByGate)) {
    blockerCounts[gate] = count;
  }

  const result: MockExecutorCycleResult = {
    ok: errors.length === 0,
    accountKey,
    mode,
    opened: entryResult.opened,
    closed: exitResult.closed,
    updatedOpen: exitResult.updated,
    funnel,
    diagnostics: {
      tickAt,
      durationMs: Date.now() - start,
      errors,
      markPrice,
      regime: evalResult.regime,
      bars,
      candidateCount: evalResult.candidateCount,
      evaluatedStrategies: evalResult.evaluatedStrategies,
      activeStrategies: evalResult.activeStrategies,
      dominantBlocker: funnel.dominantBlocker,
      blockerCounts,
      signalSummary: evalResult.summary,
    },
  };

  await persistExecutorCycle(result);

  try {
    const [openList, closedList] = await Promise.all([
      listMockTrades({
        account_key: accountKey,
        page: 1,
        limit: 200,
        status: "OPEN",
        sort: "newest",
      }),
      listMockTrades({
        account_key: accountKey,
        page: 1,
        limit: 50,
        status: "CLOSED",
        sort: "newest",
      }),
    ]);
    const portfolio: MockTrade[] = [...openList.trades, ...closedList.trades];
    const account = computeAccountState(portfolio, config);
    await insertMockAccountSnapshot(accountKey, account, config);
  } catch (err) {
    errors.push(err instanceof Error ? err.message : "ACCOUNT_SNAPSHOT_FAILED");
    result.diagnostics.errors = [...errors];
    result.ok = false;
  }

  return result;
}
