import { describe, expect, it } from "vitest";
import {
  computeRollingHealthCheck,
  computeStrategyDiagnostics,
} from "../futuresStrategyDiagnostics";
import type { PaperTradeDbRow } from "../paperTradesTypes";

// ── Fixtures ──────────────────────────────────────────────────────────────

const now = Date.now();
const ago = (m: number) => new Date(now - m * 60_000).toISOString();

const mkTrade = (
  overrides: Partial<{
    strategy_id: number;
    strategy_name: string;
    template_family: string;
    net_pnl: number;
    gross_pnl: number;
    fees: number;
    exit_reason: string;
    opened_at: string;
    closed_at: string;
  }> = {},
): PaperTradeDbRow => ({
  id: "t1",
  created_at: ago(30),
  account_key: "acc",
  client_trade_id: "c1",
  symbol: "BTCUSD",
  side: "LONG",
  entry_price: 50000,
  exit_price: 49900,
  contracts: 1,
  notional: 100,
  margin_used: 4,
  funding_costs: 0,
  strategy_id: 1,
  strategy_name: "MTF_Trend_Align_Short",
  template_family: "mtf",
  net_pnl: -20,
  gross_pnl: -15,
  fees: 5,
  exit_reason: "SL",
  opened_at: ago(30),
  closed_at: ago(10),
  payload: null,
  ...overrides,
});

// ── computeStrategyDiagnostics ────────────────────────────────────────────

describe("computeStrategyDiagnostics", () => {
  it("excludes probe trades from production count", () => {
    const trades = [
      mkTrade({ strategy_id: 99, strategy_name: "PAPER_BOOTSTRAP_PROBE", net_pnl: 99999 }),
      mkTrade({ strategy_id: 1, strategy_name: "MTF_Trend_Align_Short", net_pnl: -20 }),
    ];
    const result = computeStrategyDiagnostics(trades);
    expect(result.totalProduction).toBe(1);
  });

  it("computes winRate correctly", () => {
    const trades = [
      mkTrade({ net_pnl: 10, gross_pnl: 12, fees: 2, exit_reason: "TP" }),
      mkTrade({ net_pnl: -20, gross_pnl: -15, fees: 5, exit_reason: "SL" }),
      mkTrade({ net_pnl: 15, gross_pnl: 18, fees: 3, exit_reason: "TP" }),
      mkTrade({ net_pnl: -20, gross_pnl: -15, fees: 5, exit_reason: "SL" }),
    ];
    const result = computeStrategyDiagnostics(trades);
    const row = result.rows.find((r) => r.strategyId === 1)!;
    expect(row.winRate).toBeCloseTo(0.5, 2);
    expect(row.tpCount).toBe(2);
    expect(row.slCount).toBe(2);
  });

  it("profitFactor = Infinity when no losses", () => {
    const trades = [
      mkTrade({ net_pnl: 10, exit_reason: "TP" }),
      mkTrade({ net_pnl: 20, exit_reason: "TP" }),
    ];
    const result = computeStrategyDiagnostics(trades);
    const row = result.rows[0];
    expect(row.profitFactor).toBe(Infinity);
  });

  it("profitFactor = 0 when no wins", () => {
    const trades = [
      mkTrade({ net_pnl: -10, exit_reason: "SL" }),
      mkTrade({ net_pnl: -20, exit_reason: "SL" }),
    ];
    const result = computeStrategyDiagnostics(trades);
    const row = result.rows[0];
    expect(row.profitFactor).toBe(0);
  });

  it("feePctOfAbsGross is correct", () => {
    const trades = [
      mkTrade({ fees: 5, gross_pnl: -15 }),
      mkTrade({ fees: 5, gross_pnl: -15 }),
    ];
    const result = computeStrategyDiagnostics(trades);
    const row = result.rows[0];
    expect(row.feePctOfAbsGross).toBeCloseTo(10 / 30, 3);
  });

  it("topByExpectancy excludes probes", () => {
    const trades = [
      mkTrade({ strategy_name: "PAPER_BOOTSTRAP_PROBE", net_pnl: 99999 }),
      mkTrade({ net_pnl: 5, exit_reason: "TP" }),
    ];
    const result = computeStrategyDiagnostics(trades);
    expect(result.topByExpectancy.every((r) => !r.isProbe)).toBe(true);
  });

  it("slDominatedStrats flags when SL > 60% of trades", () => {
    const trades = [
      mkTrade({ exit_reason: "SL" }),
      mkTrade({ exit_reason: "SL" }),
      mkTrade({ exit_reason: "SL" }),
      mkTrade({ exit_reason: "TP", net_pnl: 10 }),
    ];
    const result = computeStrategyDiagnostics(trades);
    expect(result.slDominatedStrats.length).toBeGreaterThan(0);
  });

  it("exitReasonCounts maps all exit types", () => {
    const trades = [
      mkTrade({ exit_reason: "SL" }),
      mkTrade({ exit_reason: "TP", net_pnl: 10 }),
      mkTrade({ exit_reason: "TRAIL", net_pnl: 5 }),
    ];
    const result = computeStrategyDiagnostics(trades);
    const row = result.rows[0];
    expect(row.exitReasonCounts.SL).toBe(1);
    expect(row.exitReasonCounts.TP).toBe(1);
    expect(row.exitReasonCounts.TRAIL).toBe(1);
  });
});

// ── computeRollingHealthCheck ─────────────────────────────────────────────

describe("computeRollingHealthCheck", () => {
  it("grade A when all 5 checks pass", () => {
    const trades = Array.from({ length: 20 }, (_, i) =>
      mkTrade({
        net_pnl: i % 2 === 0 ? 15 : 5,
        gross_pnl: i % 2 === 0 ? 20 : 8,
        fees: 2,
        exit_reason: i % 2 === 0 ? "TP" : "TRAIL",
        closed_at: ago(i),
      }),
    );
    const result = computeRollingHealthCheck(trades, 20);
    expect(result.grade).toBe("A");
    expect(result.overallPass).toBe(true);
  });

  it("grade F when expectancy negative", () => {
    const trades = Array.from({ length: 10 }, () =>
      mkTrade({ net_pnl: -20, gross_pnl: -15, fees: 5, exit_reason: "SL" }),
    );
    const result = computeRollingHealthCheck(trades, 10);
    expect(result.expectancyPass).toBe(false);
    expect(result.grade).toBe("F");
  });

  it("uses only last N trades", () => {
    const old = mkTrade({ net_pnl: 100, closed_at: ago(1000), exit_reason: "TP" });
    const fresh = mkTrade({ net_pnl: -20, closed_at: ago(5), exit_reason: "SL" });
    const result = computeRollingHealthCheck([old, fresh], 1);
    expect(result.window).toBe(1);
    expect(result.expectancy).toBeCloseTo(-20, 0);
  });

  it("excludes probe trades from window", () => {
    const trades = [
      mkTrade({ strategy_name: "PAPER_BOOTSTRAP_PROBE", net_pnl: 99999 }),
      mkTrade({ net_pnl: -20, exit_reason: "SL" }),
    ];
    const result = computeRollingHealthCheck(trades, 20);
    expect(result.window).toBe(1);
    expect(result.expectancy).toBeCloseTo(-20, 0);
  });

  it("tpHitPass requires >= 3 TP in window", () => {
    const trades = [
      mkTrade({ net_pnl: 10, exit_reason: "TP" }),
      mkTrade({ net_pnl: 10, exit_reason: "TP" }),
      mkTrade({ net_pnl: -5, exit_reason: "SL" }),
    ];
    const result = computeRollingHealthCheck(trades, 20);
    expect(result.tpHits).toBe(2);
    expect(result.tpHitPass).toBe(false);
  });
});
