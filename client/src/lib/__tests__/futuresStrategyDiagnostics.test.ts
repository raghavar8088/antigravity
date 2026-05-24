import { describe, expect, it } from "vitest";
import {
  computeStrategyDiagnostics,
  computeRollingHealthCheck,
  computeStrategyDiagnosticsFromEngineTrades,
  computeRollingHealthCheckFromEngineTrades,
} from "../futuresStrategyDiagnostics";
import type { PaperTradeDbRow } from "../paperTradesTypes";
import type { BTCFuturesTrade } from "../btcFuturesTrade.types";

// ─── Factories ────────────────────────────────────────────────────────────────

const BASE_ROW: PaperTradeDbRow = {
  id: "t1",
  created_at: "2024-01-01T00:00:00Z",
  account_key: "acc",
  client_trade_id: "c1",
  opened_at: "2024-01-01T00:00:00Z",
  closed_at: "2024-01-01T00:10:00Z",
  symbol: "BTCUSD",
  strategy_id: 1,
  strategy_name: "btc_scalp_v1",
  side: "LONG",
  entry_price: 50000,
  exit_price: 50100,
  contracts: 1,
  notional: 100,
  margin_used: 4,
  gross_pnl: 10,
  fees: 0.2,
  funding_costs: 0,
  net_pnl: 9.8,
  exit_reason: "TP",
  payload: null,
  template_family: "scalp",
};

function makeRow(
  overrides: Partial<PaperTradeDbRow> & { id?: string },
): PaperTradeDbRow {
  return { ...BASE_ROW, ...overrides };
}

// ─── computeStrategyDiagnostics ───────────────────────────────────────────────

describe("computeStrategyDiagnostics", () => {
  it("returns empty rows for empty input", () => {
    const result = computeStrategyDiagnostics([]);
    expect(result.rows).toHaveLength(0);
    expect(result.totalProduction).toBe(0);
    expect(result.computedAt).toBeGreaterThan(0);
  });

  it("groups trades by strategy_id", () => {
    const trades = [
      makeRow({ id: "a", strategy_id: 1, net_pnl: 10 }),
      makeRow({ id: "b", strategy_id: 2, net_pnl: -5 }),
      makeRow({ id: "c", strategy_id: 1, net_pnl: 5 }),
    ];
    const result = computeStrategyDiagnostics(trades);
    expect(result.rows).toHaveLength(2);
    const row1 = result.rows.find((r) => r.strategyId === 1)!;
    expect(row1.totalTrades).toBe(2);
    expect(row1.totalNetPnl).toBeCloseTo(15);
  });

  it("computes win/loss counts and winRate correctly", () => {
    const trades = [
      makeRow({ id: "a", net_pnl: 10, exit_reason: "TP" }),
      makeRow({ id: "b", net_pnl: -5, exit_reason: "SL" }),
      makeRow({ id: "c", net_pnl: 8, exit_reason: "TP" }),
      makeRow({ id: "d", net_pnl: -3, exit_reason: "SL" }),
    ];
    const result = computeStrategyDiagnostics(trades);
    const row = result.rows[0];
    expect(row.wins).toBe(2);
    expect(row.losses).toBe(2);
    expect(row.winRate).toBeCloseTo(0.5);
    expect(row.slCount).toBe(2);
    expect(row.tpCount).toBe(2);
  });

  it("computes profitFactor as sumWins / |sumLosses|", () => {
    const trades = [
      makeRow({ id: "a", net_pnl: 20, gross_pnl: 20.2, fees: 0.2 }),
      makeRow({ id: "b", net_pnl: -10, gross_pnl: -9.8, fees: 0.2 }),
    ];
    const result = computeStrategyDiagnostics(trades);
    expect(result.rows[0].profitFactor).toBeCloseTo(2.0);
  });

  it("profitFactor is Infinity when there are no losses", () => {
    const trades = [
      makeRow({ id: "a", net_pnl: 10 }),
      makeRow({ id: "b", net_pnl: 5 }),
    ];
    const result = computeStrategyDiagnostics(trades);
    expect(result.rows[0].profitFactor).toBe(Infinity);
  });

  it("profitFactor is 0 when there are no wins and no losses", () => {
    const trades = [makeRow({ id: "a", net_pnl: 0 })];
    const result = computeStrategyDiagnostics(trades);
    expect(result.rows[0].profitFactor).toBe(0);
  });

  it("computes avgHoldMinutes from opened_at / closed_at", () => {
    const openedAt = "2024-01-01T00:00:00Z";
    const closedAt = "2024-01-01T00:30:00Z"; // 30 min
    const trades = [makeRow({ id: "a", opened_at: openedAt, closed_at: closedAt })];
    const result = computeStrategyDiagnostics(trades);
    expect(result.rows[0].avgHoldMinutes).toBeCloseTo(30);
  });

  it("feePctOfAbsGross is fees / sum(|gross|)", () => {
    const trades = [
      makeRow({ id: "a", gross_pnl: 10, fees: 1 }),
      makeRow({ id: "b", gross_pnl: -10, fees: 1 }),
    ];
    const result = computeStrategyDiagnostics(trades);
    expect(result.rows[0].feePctOfAbsGross).toBeCloseTo(0.1); // 2 / 20
  });

  it("marks probe strategies with isProbe=true", () => {
    const trades = [
      makeRow({ id: "a", strategy_name: "BOOTSTRAP_v1", net_pnl: 100 }),
    ];
    const result = computeStrategyDiagnostics(trades);
    expect(result.rows[0].isProbe).toBe(true);
    expect(result.totalProduction).toBe(0);
  });

  it("excludes probe rows from topByExpectancy / totalProduction", () => {
    const trades = [
      makeRow({ id: "a", strategy_id: 1, strategy_name: "btc_scalp_v1", net_pnl: 5 }),
      makeRow({ id: "b", strategy_id: 99, strategy_name: "PROBE_entry", net_pnl: 999 }),
    ];
    const result = computeStrategyDiagnostics(trades);
    expect(result.totalProduction).toBe(1);
    expect(result.topByExpectancy.every((r) => !r.isProbe)).toBe(true);
  });

  it("highFeeStrategies flags feePctOfAbsGross > 0.5", () => {
    const trades = [
      makeRow({ id: "a", gross_pnl: 1, fees: 0.8, net_pnl: 0.2 }),
    ];
    const result = computeStrategyDiagnostics(trades);
    expect(result.highFeeStrategies).toHaveLength(1);
  });

  it("slDominatedStrats requires min 3 trades and > 60% SL", () => {
    const trades = [
      makeRow({ id: "a", exit_reason: "SL", net_pnl: -5 }),
      makeRow({ id: "b", exit_reason: "SL", net_pnl: -5 }),
      makeRow({ id: "c", exit_reason: "TP", net_pnl: 10 }),
    ];
    // 2/3 = 67% SL — should appear
    const result = computeStrategyDiagnostics(trades);
    expect(result.slDominatedStrats).toHaveLength(1);

    // Only 2 trades — should NOT appear even at 100% SL
    const trades2 = [
      makeRow({ id: "d", exit_reason: "SL", net_pnl: -5 }),
      makeRow({ id: "e", exit_reason: "SL", net_pnl: -5 }),
    ];
    const result2 = computeStrategyDiagnostics(trades2);
    expect(result2.slDominatedStrats).toHaveLength(0);
  });

  it("lastTradeAt is the most recent closed_at", () => {
    const trades = [
      makeRow({ id: "a", closed_at: "2024-01-01T00:05:00Z" }),
      makeRow({ id: "b", closed_at: "2024-01-01T00:15:00Z" }),
      makeRow({ id: "c", closed_at: "2024-01-01T00:10:00Z" }),
    ];
    const result = computeStrategyDiagnostics(trades);
    expect(result.rows[0].lastTradeAt).toBe("2024-01-01T00:15:00Z");
  });
});

// ─── computeRollingHealthCheck ────────────────────────────────────────────────

describe("computeRollingHealthCheck", () => {
  it("returns zeroes and grade F for empty trades", () => {
    const result = computeRollingHealthCheck([]);
    expect(result.window).toBe(0);
    expect(result.expectancy).toBe(0);
    expect(result.grade).toBe("F");
    expect(result.overallPass).toBe(false);
  });

  it("uses last windowN trades only", () => {
    const trades = Array.from({ length: 25 }, (_, i) =>
      makeRow({
        id: `t${i}`,
        net_pnl: i < 5 ? -100 : 10,  // first 5 (oldest by closed_at) are big losses
        exit_reason: i < 5 ? "SL" : "TP",
        closed_at: new Date(Date.UTC(2024, 0, i + 1)).toISOString(),
      }),
    );
    const result = computeRollingHealthCheck(trades, 20);
    // Only the most recent 20 are included — all are +10, so expectancy > 0
    expect(result.expectancy).toBeGreaterThan(0);
    expect(result.window).toBe(20);
  });

  it("excludes probe trades from the window", () => {
    const trades = [
      makeRow({ id: "a", strategy_name: "btc_scalp_v1", net_pnl: 10, exit_reason: "TP" }),
      makeRow({ id: "b", strategy_name: "BOOTSTRAP_long", net_pnl: -200, exit_reason: "SL" }),
    ];
    const result = computeRollingHealthCheck(trades);
    expect(result.window).toBe(1);
    expect(result.expectancy).toBeCloseTo(10);
  });

  it("all 5 checks pass → grade A and overallPass true", () => {
    // winRate > 0.35, expectancy > 0, feePct < 0.5, PF > 1, tpHits >= 3
    const trades = Array.from({ length: 6 }, (_, i) =>
      makeRow({
        id: `t${i}`,
        net_pnl: 10,
        gross_pnl: 10.2,
        fees: 0.2,
        exit_reason: "TP",
        closed_at: new Date(Date.UTC(2024, 0, i + 1)).toISOString(),
      }),
    );
    const result = computeRollingHealthCheck(trades);
    expect(result.expectancyPass).toBe(true);
    expect(result.winRatePass).toBe(true);
    expect(result.feePass).toBe(true);
    expect(result.pfPass).toBe(true);
    expect(result.tpHitPass).toBe(true);
    expect(result.grade).toBe("A");
    expect(result.overallPass).toBe(true);
  });

  it("4 checks pass → grade B", () => {
    // Make tpHits = 2 (< 3) to fail tpHitPass; everything else passes
    const trades = [
      makeRow({ id: "a", net_pnl: 10, gross_pnl: 10.2, fees: 0.2, exit_reason: "TP" }),
      makeRow({ id: "b", net_pnl: 10, gross_pnl: 10.2, fees: 0.2, exit_reason: "TP" }),
      makeRow({ id: "c", net_pnl: 10, gross_pnl: 10.2, fees: 0.2, exit_reason: "TRAIL" }),
      makeRow({ id: "d", net_pnl: 10, gross_pnl: 10.2, fees: 0.2, exit_reason: "TRAIL" }),
    ];
    const result = computeRollingHealthCheck(trades);
    expect(result.tpHitPass).toBe(false);
    expect(result.tpHits).toBe(2);
    expect(result.grade).toBe("B");
  });

  it("3 checks pass → grade C", () => {
    // 3 TP wins at $2, 3 SL losses at $2:
    //   winRate=0.5 ✓, feePass ✓, tpHits=3 ✓, expectancy=0 ✗, pfPass=1.0 ✗ → C
    const trades = [
      makeRow({ id: "a", net_pnl: 2, gross_pnl: 2.2, fees: 0.2, exit_reason: "TP" }),
      makeRow({ id: "b", net_pnl: 2, gross_pnl: 2.2, fees: 0.2, exit_reason: "TP" }),
      makeRow({ id: "c", net_pnl: 2, gross_pnl: 2.2, fees: 0.2, exit_reason: "TP" }),
      makeRow({ id: "d", net_pnl: -2, gross_pnl: -1.8, fees: 0.2, exit_reason: "SL" }),
      makeRow({ id: "e", net_pnl: -2, gross_pnl: -1.8, fees: 0.2, exit_reason: "SL" }),
      makeRow({ id: "f", net_pnl: -2, gross_pnl: -1.8, fees: 0.2, exit_reason: "SL" }),
    ];
    const result = computeRollingHealthCheck(trades);
    expect(result.winRatePass).toBe(true);   // 0.5 > 0.35
    expect(result.expectancyPass).toBe(false); // 0 is not > 0
    expect(result.pfPass).toBe(false);        // 6/6 = 1.0, not > 1.0
    expect(result.tpHitPass).toBe(true);     // 3 TPs
    expect(result.grade).toBe("C");
  });

  it("fewer than 3 checks pass → grade F", () => {
    // All losses — expectancyPass=false, winRatePass=false, pfPass=false; feePct and tpHits also bad
    const trades = Array.from({ length: 5 }, (_, i) =>
      makeRow({ id: `t${i}`, net_pnl: -10, gross_pnl: -9.8, fees: 0.2, exit_reason: "SL" }),
    );
    const result = computeRollingHealthCheck(trades);
    expect(result.grade).toBe("F");
  });

  it("feePctOfAbsGross is a fraction (not percentage)", () => {
    const trades = [
      makeRow({ id: "a", gross_pnl: 10, fees: 1, net_pnl: 9, exit_reason: "TP" }),
    ];
    const result = computeRollingHealthCheck(trades);
    // fees=1, absGross=10 → 0.10 fraction (not 10%)
    expect(result.feePctOfAbsGross).toBeCloseTo(0.1);
    expect(result.feePass).toBe(true); // < 0.5
  });

  it("counts slCount and timeCount correctly", () => {
    const trades = [
      makeRow({ id: "a", exit_reason: "SL", net_pnl: -5 }),
      makeRow({ id: "b", exit_reason: "SL", net_pnl: -5 }),
      makeRow({ id: "c", exit_reason: "TP", net_pnl: 15, gross_pnl: 15.2, fees: 0.2 }),
      makeRow({ id: "d", exit_reason: "TP", net_pnl: 15, gross_pnl: 15.2, fees: 0.2 }),
      makeRow({ id: "e", exit_reason: "TP", net_pnl: 15, gross_pnl: 15.2, fees: 0.2 }),
    ];
    const result = computeRollingHealthCheck(trades);
    expect(result.slCount).toBe(2);
    expect(result.timeCount).toBe(0);
    expect(result.tpHits).toBe(3);
    expect(result.tpHitPass).toBe(true);
  });
});

// ─── Engine-trade adapter wrappers ────────────────────────────────────────────

describe("computeStrategyDiagnosticsFromEngineTrades", () => {
  it("produces same result as the PaperTradeDbRow path", () => {
    const engineTrades: BTCFuturesTrade[] = [
      {
        id: "e1",
        clientTradeId: "c1",
        openedAt: "2024-01-01T00:00:00Z",
        closedAt: "2024-01-01T00:10:00Z",
        symbol: "BTCUSD",
        strategyId: 1,
        strategyName: "btc_scalp_v1",
        side: "LONG",
        entryPrice: 50000,
        exitPrice: 50100,
        contracts: 1,
        notional: 100,
        marginUsed: 4,
        realizedPnl: 10,
        fees: 0.2,
        fundingCosts: 0,
        netPnl: 9.8,
        netPnlPct: 0.1,
        priceMovePct: 0.2,
        exitReason: "TP",
        liquidationPrice: 48000,
        liquidationDistancePct: 4,
      },
    ];
    const result = computeStrategyDiagnosticsFromEngineTrades(engineTrades);
    expect(result.rows).toHaveLength(1);
    expect(result.rows[0].strategyId).toBe(1);
    expect(result.rows[0].tpCount).toBe(1);
    expect(result.rows[0].totalNetPnl).toBeCloseTo(9.8);
  });
});

describe("computeRollingHealthCheckFromEngineTrades", () => {
  it("grades an all-TP engine trade session as passing tpHitPass when >= 3 TPs", () => {
    const make = (id: string): BTCFuturesTrade => ({
      id,
      clientTradeId: id,
      openedAt: "2024-01-01T00:00:00Z",
      closedAt: "2024-01-01T00:10:00Z",
      symbol: "BTCUSD",
      strategyId: 1,
      strategyName: "btc_scalp_v1",
      side: "LONG",
      entryPrice: 50000,
      exitPrice: 50100,
      contracts: 1,
      notional: 100,
      marginUsed: 4,
      realizedPnl: 10,
      fees: 0.2,
      fundingCosts: 0,
      netPnl: 9.8,
      netPnlPct: 0.1,
      priceMovePct: 0.2,
      exitReason: "TP",
      liquidationPrice: 48000,
      liquidationDistancePct: 4,
    });
    const result = computeRollingHealthCheckFromEngineTrades([make("e1"), make("e2"), make("e3")]);
    expect(result.tpHitPass).toBe(true);
    expect(result.tpHits).toBe(3);
    expect(result.grade).toBe("A");
  });
});
