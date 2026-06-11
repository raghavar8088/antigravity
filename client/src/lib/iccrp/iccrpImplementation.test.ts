import { describe, expect, it } from "vitest";
import {
  buildCorrelationMatrix,
  computePortfolioRiskMetrics,
  aggregateByRegime,
  type TradeRow,
} from "@/lib/paperTradeAnalyticsApi";
import { mapSnapshotToTerminalDelta } from "@/lib/terminal/mapSnapshotToTerminalDelta";
import { terminalHasAuthority } from "@/lib/terminal/terminalAuthority";
import { initialTerminalSnapshot } from "@/lib/terminal/terminalSnapshot";

describe("mapSnapshotToTerminalDelta", () => {
  it("maps Mongo snapshot to terminal delta without synthetic defaults", () => {
    const delta = mapSnapshotToTerminalDelta({
      snapshot: {
        server_time: "2026-06-11T12:00:00.000Z",
        state: { balance: 950000, equity: 960000, current_drawdown: 0.02, win_rate: 0.55, total_fees: 120 },
        open_positions: [{
          position_id: "p1",
          strategy_id: "FundingMR",
          side: "LONG",
          entry_price: 100000,
          size: 0.1,
        }],
        recent_trades: [],
        health_summary: { critical: 1, warning: 2 },
        portfolio: {
          exposure: { gross_exposure_btc: 0.2, net_exposure_btc: 0.1, exposure_usd: 20000 },
          drawdown: { current_drawdown: 0.02 },
          sharpe: 1.2,
          profit_factor: 1.3,
        },
      },
      btcPrice: { price: 100500, change24h: 1.5 },
      strategies: [{
        strategy_id: "FundingMR",
        status: "HEALTHY",
        enabled: true,
        total_pnl: 1000,
        expectancy: 12,
        profit_factor: 1.4,
        win_rate: 0.6,
        max_drawdown: 0.05,
        sample_size: 50,
        evidence_score: 80,
        allocation_tier: "A",
      }],
    });

    expect(delta.price).toBe(100500);
    expect(delta.positions?.length).toBe(1);
    expect(delta.strategies?.length).toBe(1);
    expect(delta.strategies?.[0]?.sharpe).toBeNull();
    expect(delta.analytics?.rollingSharpe30d).toBe(1.2);
    expect(delta.analytics?.profitFactorTrend).toBe(1.3);
    expect(delta.alerts?.some((a: { severity: string }) => a.severity === "CRITICAL")).toBe(true);
    expect(delta.updatedAt).toBe("2026-06-11T12:00:00.000Z");
  });
});

describe("terminalHasAuthority", () => {
  it("requires REST updatedAt when not on WS", () => {
    expect(terminalHasAuthority({
      ...initialTerminalSnapshot,
      authoritySource: "rest",
      restUnavailable: false,
      hasAuthority: false,
      updatedAt: "2026-06-11T12:00:00.000Z",
    })).toBe(true);
  });

  it("rejects WS connected before first delta", () => {
    expect(terminalHasAuthority({
      ...initialTerminalSnapshot,
      authoritySource: "ws",
      connected: true,
      restUnavailable: false,
      hasAuthority: false,
      updatedAt: "",
    })).toBe(false);
  });

  it("accepts WS after first delta", () => {
    expect(terminalHasAuthority({
      ...initialTerminalSnapshot,
      authoritySource: "ws",
      connected: true,
      restUnavailable: false,
      hasAuthority: true,
      updatedAt: "2026-06-11T12:00:00.000Z",
    })).toBe(true);
  });
});

describe("paperTradeAnalyticsApi", () => {
  const trades: TradeRow[] = [
    { strategy_id: "A", net_pnl: 100, closed_at: "2026-06-01T00:00:00.000Z" },
    { strategy_id: "A", net_pnl: -50, closed_at: "2026-06-02T00:00:00.000Z" },
    { strategy_id: "B", net_pnl: 20, closed_at: "2026-06-01T12:00:00.000Z", regime_at_entry: "TRENDING" },
  ];

  it("builds correlation matrix with diagonal 1", () => {
    const { labels, matrix } = buildCorrelationMatrix(trades);
    expect(labels.length).toBeGreaterThan(0);
    expect(matrix[0][0]).toBe(1);
  });

  it("aggregates regime stats", () => {
    const regimes = aggregateByRegime(trades);
    expect(regimes.some((r) => r.regime === "TRENDING")).toBe(true);
  });

  it("computes risk metrics from trades", () => {
    const m = computePortfolioRiskMetrics(trades);
    expect(m.maxDrawdownPct).toBeGreaterThanOrEqual(0);
  });
});
