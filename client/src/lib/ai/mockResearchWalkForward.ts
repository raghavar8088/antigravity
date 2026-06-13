import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import { runWalkForwardValidation } from "@/lib/analytics/walkForwardValidation";

export interface MockWalkForwardRow {
  strategyId: number;
  strategyName: string;
  trainDays: number;
  validationDays: number;
  windows: number;
  inSampleExpectancy: number;
  outOfSampleExpectancy: number;
  outOfSampleNetPnl: number;
  outOfSampleTrades: number;
  walkForwardScore: number;
  status: "PASS" | "FAIL" | "COLLECT_DATA";
  reason: string;
}

export interface MockWalkForwardOptions {
  trainDays?: number;
  validationDays?: number;
  minTrainTrades?: number;
  minValidationTrades?: number;
}

function closedTrades(trades: readonly MockTrade[]): MockTrade[] {
  return trades.filter((trade) => trade.status === "CLOSED" && trade.closedAt != null);
}

export function computeMockWalkForwardRows(
  trades: readonly MockTrade[],
  options: MockWalkForwardOptions = {},
): MockWalkForwardRow[] {
  const trainDays = options.trainDays ?? 21;
  const validationDays = options.validationDays ?? 7;
  const minTrainTrades = options.minTrainTrades ?? 10;
  const minValidationTrades = options.minValidationTrades ?? 3;
  const grouped = new Map<number, MockTrade[]>();

  for (const trade of closedTrades(trades)) {
    const bucket = grouped.get(trade.strategyId) ?? [];
    bucket.push(trade);
    grouped.set(trade.strategyId, bucket);
  }

  return [...grouped.entries()]
    .map(([strategyId, strategyTrades]) => {
      const result = runWalkForwardValidation(
        strategyTrades.map((trade) => ({
          closedAt: new Date(trade.closedAt ?? trade.openedAt).toISOString(),
          netPnl: trade.realizedPnl,
        })),
        {
          trainDays,
          testDays: validationDays,
          minTrainTrades,
          minTestTrades: minValidationTrades,
        },
      );
      const inSampleTrades = result.windows.reduce((sum, window) => sum + window.trainTrades, 0);
      const outOfSampleTrades = result.windows.reduce((sum, window) => sum + window.testTrades, 0);
      const inSampleWeighted =
        result.windows.reduce((sum, window) => sum + window.trainExpectancy * window.trainTrades, 0) /
        Math.max(1, inSampleTrades);
      const outOfSampleWeighted =
        result.windows.reduce((sum, window) => sum + window.testExpectancy * window.testTrades, 0) /
        Math.max(1, outOfSampleTrades);
      const outOfSampleNetPnl = result.windows.reduce(
        (sum, window) => sum + window.testExpectancy * window.testTrades,
        0,
      );

      return {
        strategyId,
        strategyName: strategyTrades[0]?.strategyName ?? `Strategy ${strategyId}`,
        trainDays,
        validationDays,
        windows: result.windows.length,
        inSampleExpectancy: inSampleWeighted,
        outOfSampleExpectancy: outOfSampleWeighted,
        outOfSampleNetPnl,
        outOfSampleTrades,
        walkForwardScore: Math.max(0, Math.min(100, result.aggregateWFE * 100)),
        status: result.status,
        reason: result.reason,
      };
    })
    .sort((a, b) => b.walkForwardScore - a.walkForwardScore || b.outOfSampleNetPnl - a.outOfSampleNetPnl);
}
