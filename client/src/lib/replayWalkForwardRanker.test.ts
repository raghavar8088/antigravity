/**
 * Tests for replayWalkForwardRanker — pure, no I/O.
 */
import { describe, it, expect } from "vitest";
import { rankReplayStrategies } from "./replayWalkForwardRanker";
import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";

// ─── Helpers ─────────────────────────────────────────────────────────────────

let counter = 0;
function makeTrade(
  strategyId: number,
  strategyName: string,
  netPnl: number,
  overrides: Partial<BTCFuturesTrade> = {},
): BTCFuturesTrade {
  counter++;
  const now = new Date(Date.UTC(2026, 0, counter)).toISOString();
  return {
    id: `trade-${counter}`,
    symbol: "BTCUSD",
    strategyId,
    strategyName,
    side: "LONG",
    entryPrice: 65000,
    exitPrice: netPnl >= 0 ? 65500 : 64500,
    contracts: 1,
    notional: 100,
    marginUsed: 4,
    realizedPnl: netPnl * 1.2,
    fees: Math.abs(netPnl) * 0.2,
    netPnl,
    netPnlPct: netPnl / 100,
    priceMovePct: 0.5,
    fundingCosts: 0,
    openedAt: now,
    closedAt: now,
    exitReason: netPnl >= 0 ? "TP" : "SL",
    liquidationPrice: 60000,
    liquidationDistancePct: 7.7,
    ...overrides,
  };
}

function makeTrades(strategyId: number, strategyName: string, count: number, winRate = 0.6): BTCFuturesTrade[] {
  return Array.from({ length: count }, (_, i) =>
    makeTrade(
      strategyId,
      strategyName,
      i / count < winRate ? 5 : -3,
      { closedAt: new Date(Date.UTC(2026, 0, i + 1)).toISOString() },
    ),
  );
}

// ─── Empty input ──────────────────────────────────────────────────────────────

describe("rankReplayStrategies — empty input", () => {
  it("returns empty array for no trades", () => {
    expect(rankReplayStrategies([])).toEqual([]);
  });
});

// ─── Grouping ─────────────────────────────────────────────────────────────────

describe("rankReplayStrategies — grouping", () => {
  it("groups trades by strategyId", () => {
    const trades = [
      ...makeTrades(91, "strat-91", 10),
      ...makeTrades(92, "strat-92", 10),
    ];
    const ranks = rankReplayStrategies(trades);
    expect(ranks).toHaveLength(2);
    expect(ranks.map((r) => r.strategyId)).toContain(91);
    expect(ranks.map((r) => r.strategyId)).toContain(92);
  });
});

// ─── INSUFFICIENT ─────────────────────────────────────────────────────────────

describe("rankReplayStrategies — INSUFFICIENT", () => {
  it("returns INSUFFICIENT for < 5 trades", () => {
    const trades = makeTrades(91, "strat-91", 3);
    const [rank] = rankReplayStrategies(trades);
    expect(rank!.recommendation).toBe("INSUFFICIENT");
  });
});

// ─── DISABLE ─────────────────────────────────────────────────────────────────

describe("rankReplayStrategies — DISABLE", () => {
  it("returns DISABLE for negative expectancy with very high fee/gross", () => {
    // Manufacture a case: negative pnl, fees > gross (fee/gross > 100%)
    const trades = Array.from({ length: 10 }, (_, i) =>
      makeTrade(91, "strat-91", -5, {
        closedAt: new Date(Date.UTC(2026, 0, i + 1)).toISOString(),
        realizedPnl: -1, // near-zero gross
        fees: 10,        // fee >> gross → fee/gross > 100%
      }),
    );
    const [rank] = rankReplayStrategies(trades);
    expect(rank!.recommendation).toBe("DISABLE");
  });
});

// ─── Recommendation shape ─────────────────────────────────────────────────────

describe("rankReplayStrategies — shape", () => {
  it("each rank has required fields", () => {
    const trades = makeTrades(91, "strat-91", 8);
    const [rank] = rankReplayStrategies(trades);
    expect(typeof rank!.strategyId).toBe("number");
    expect(typeof rank!.strategyName).toBe("string");
    expect(typeof rank!.replayTrades).toBe("number");
    expect(typeof rank!.replayExpectancy).toBe("number");
    expect(typeof rank!.replayWinRate).toBe("number");
    expect(typeof rank!.replaySumNet).toBe("number");
    expect(typeof rank!.replayFeePctOfAbsGross).toBe("number");
    expect(rank!.walkForward).toBeDefined();
    expect(typeof rank!.recommendation).toBe("string");
    expect(typeof rank!.recommendationReason).toBe("string");
  });
});

// ─── Sorting ─────────────────────────────────────────────────────────────────

describe("rankReplayStrategies — sorting", () => {
  it("sorts PROMOTE before WATCH", () => {
    // Enough trades for a verdict, first one positive, second negative
    const t91 = makeTrades(91, "strat-91", 25, 0.75); // likely PROMOTE or KEEP
    const t92 = makeTrades(92, "strat-92", 25, 0.2);  // likely WATCH or DISABLE
    const ranks = rankReplayStrategies([...t91, ...t92]);
    const r91 = ranks.find((r) => r.strategyId === 91)!;
    const r92 = ranks.find((r) => r.strategyId === 92)!;
    const ORDER: Record<string, number> = { PROMOTE: 0, KEEP: 1, WATCH: 2, DISABLE: 3, INSUFFICIENT: 4 };
    expect((ORDER[r91.recommendation] ?? 99) <= (ORDER[r92.recommendation] ?? 99)).toBe(true);
  });
});

// ─── walkForward field ────────────────────────────────────────────────────────

describe("rankReplayStrategies — walkForward", () => {
  it("walkForward.status is one of PASS/FAIL/COLLECT_DATA", () => {
    const trades = makeTrades(91, "strat-91", 20);
    const [rank] = rankReplayStrategies(trades);
    expect(["PASS", "FAIL", "COLLECT_DATA"]).toContain(rank!.walkForward.status);
  });
});
