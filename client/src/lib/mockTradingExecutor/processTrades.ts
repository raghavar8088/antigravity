import type { StrategySignalTraceRow } from "@/lib/ai/strategySignalTrace";
import {
  applyPriceTickToTrade,
  buildMockTradeFromTrace,
  computeAccountState,
  evaluateMockTradeOpenRisk,
  isExecutableTraceRow,
  maxSignalsPerBatchFromConfig,
  scoreMockTraceRow,
  type MockTrade,
  type MockTradingConfig,
} from "@/lib/trading/mockTradingEngine";
import {
  listMockTrades,
  upsertMockTrade,
} from "@/lib/trading/mockTradingMongo";

export type ExecutorEntryResult = {
  opened: MockTrade[];
  rejected: number;
};

/**
 * Server-side mirror of useMockTradingEngine.ingestTraceRows — opens trades in MongoDB.
 */
export async function openTradesFromTraceRows(args: {
  accountKey: string;
  rows: StrategySignalTraceRow[];
  markPrice: number;
  config: MockTradingConfig;
  now?: number;
  existingOpenTrades?: MockTrade[];
  knownTraceIds?: Set<string>;
}): Promise<ExecutorEntryResult> {
  const now = args.now ?? Date.now();
  const livePrice = args.markPrice;
  if (!Number.isFinite(livePrice) || livePrice <= 0) {
    return { opened: [], rejected: 0 };
  }

  const openTrades =
    args.existingOpenTrades ??
    (
      await listMockTrades({
        account_key: args.accountKey,
        page: 1,
        limit: 200,
        status: "OPEN",
        sort: "newest",
      })
    ).trades;

  const knownTraceIds = args.knownTraceIds ?? new Set<string>();
  for (const trade of openTrades) {
    if (trade.traceId) knownTraceIds.add(trade.traceId);
  }

  const closedRecent = (
    await listMockTrades({
      account_key: args.accountKey,
      page: 1,
      limit: 500,
      status: "CLOSED",
      sort: "newest",
    })
  ).trades;
  for (const trade of closedRecent) {
    if (trade.traceId) knownTraceIds.add(trade.traceId);
  }

  const portfolioTrades = [...openTrades, ...closedRecent.slice(0, 50)];
  const equity = computeAccountState(portfolioTrades, args.config).equity;

  const raisedRows = args.rows.filter(
    (row) => isExecutableTraceRow(row) && !knownTraceIds.has(row.traceId),
  );
  const rankedRows = [...raisedRows].sort((a, b) => scoreMockTraceRow(b) - scoreMockTraceRow(a));
  const selectedRows = rankedRows.slice(0, maxSignalsPerBatchFromConfig(args.config));

  const opened: MockTrade[] = [];
  let rejected = rankedRows.length - selectedRows.length;
  const pendingOpens: MockTrade[] = [];

  for (const row of selectedRows) {
    const trade = buildMockTradeFromTrace({
      row,
      currentPrice: livePrice,
      config: args.config,
      now,
      equity,
    });
    if (!trade) {
      knownTraceIds.add(row.traceId);
      rejected += 1;
      continue;
    }

    const decision = evaluateMockTradeOpenRisk({
      trade,
      existingTrades: openTrades,
      pendingTrades: pendingOpens,
      config: args.config,
      now,
    });
    if (!decision.allowed) {
      knownTraceIds.add(row.traceId);
      rejected += 1;
      continue;
    }

    await upsertMockTrade(args.accountKey, trade, args.config, "MOCK_TRADE_CREATED");
    knownTraceIds.add(row.traceId);
    pendingOpens.push(trade);
    opened.push(trade);
  }

  return { opened, rejected };
}

export type ExecutorExitResult = {
  closed: MockTrade[];
  updated: number;
};

/** Mark open trades at live price; persist closes and open mark updates. */
export async function closeOpenTradesAtPrice(args: {
  accountKey: string;
  markPrice: number;
  config: MockTradingConfig;
  now?: number;
  openTrades?: MockTrade[];
}): Promise<ExecutorExitResult> {
  const now = args.now ?? Date.now();
  const price = args.markPrice;
  if (!Number.isFinite(price) || price <= 0) {
    return { closed: [], updated: 0 };
  }

  const openTrades =
    args.openTrades ??
    (
      await listMockTrades({
        account_key: args.accountKey,
        page: 1,
        limit: 200,
        status: "OPEN",
        sort: "newest",
      })
    ).trades;

  const closed: MockTrade[] = [];
  let updated = 0;

  for (const trade of openTrades) {
    const before = trade.status;
    const next = applyPriceTickToTrade({
      trade,
      price,
      config: args.config,
      now,
    });
    if (next === trade) continue;

    if (before === "OPEN" && next.status === "CLOSED") {
      await upsertMockTrade(args.accountKey, next, args.config, "MOCK_TRADE_CLOSED");
      closed.push(next);
      continue;
    }

    if (next.status === "OPEN") {
      await upsertMockTrade(args.accountKey, next, args.config, "MOCK_TRADE_UPDATED");
      updated += 1;
    }
  }

  return { closed, updated };
}
