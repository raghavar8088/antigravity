/**
 * Tests for researchEdgeScore — pure function, no I/O.
 */
import { describe, it, expect } from "vitest";
import { computeResearchEdgeScores } from "./researchEdgeScore";
import type { PaperTradeDbRow } from "@/lib/paperTradesTypes";

// ── Helpers ──────────────────────────────────────────────────────────────────

let _seq = 0;
function makeTrade(
  strategyId: number,
  netPnl: number,
  opts: {
    grossPnl?: number;
    fees?: number;
    exitReason?: string;
    strategyName?: string;
    templateFamily?: string;
    closedAt?: string;
    regime?: string;
  } = {},
): PaperTradeDbRow {
  const gross = opts.grossPnl ?? netPnl * 1.05;
  const fees = opts.fees ?? Math.abs(gross - netPnl);
  const closedAt = opts.closedAt ?? `2026-05-${String(1 + (_seq++ % 28)).padStart(2, "0")}T12:00:00Z`;
  return {
    id: `t-${_seq++}`,
    created_at: closedAt,
    account_key: "test",
    client_trade_id: `ct-${_seq++}`,
    opened_at: closedAt,
    closed_at: closedAt,
    symbol: "BTCUSDT",
    strategy_id: strategyId,
    strategy_name: opts.strategyName ?? `strat-${strategyId}`,
    side: "LONG",
    entry_price: 60000,
    exit_price: 60300,
    contracts: 1,
    notional: 100,
    margin_used: 4,
    gross_pnl: gross,
    fees,
    funding_costs: 0,
    net_pnl: netPnl,
    exit_reason: opts.exitReason ?? "TP",
    payload: opts.regime ? { regime: opts.regime } : null,
    module_key: null,
    template_family: opts.templateFamily ?? null,
    exchange: null,
  };
}

function makeTrades(
  strategyId: number,
  pnls: number[],
  opts: { grossMultiplier?: number; feeEach?: number } = {},
): PaperTradeDbRow[] {
  const gm = opts.grossMultiplier ?? 1.2;
  return pnls.map((p, i) =>
    makeTrade(strategyId, p, {
      grossPnl: p * gm,
      fees: opts.feeEach ?? Math.abs(p) * (gm - 1),
      closedAt: `2026-05-${String(1 + (i % 28)).padStart(2, "0")}T${String(i % 24).padStart(2, "0")}:00:00Z`,
    }),
  );
}

// ── Probe exclusion ───────────────────────────────────────────────────────────

describe("computeResearchEdgeScores — probe exclusion", () => {
  it("excludes trades with 'probe' in strategy_name", () => {
    const trades = [
      ...makeTrades(91, Array(15).fill(2)),
      makeTrade(99, 1, { strategyName: "bootstrap-probe" }),
      makeTrade(99, 1, { strategyName: "PAPER_ENTRY_CANARY" }),
    ];
    const scores = computeResearchEdgeScores(trades);
    expect(scores.map((s) => s.strategyId)).not.toContain(99);
    expect(scores.find((s) => s.strategyId === 91)).toBeDefined();
  });

  it("excludes trades with probe in template_family", () => {
    const trade = makeTrade(88, 5, { templateFamily: "BOOTSTRAP_PROBE" });
    const scores = computeResearchEdgeScores([trade, ...makeTrades(91, Array(12).fill(1))]);
    expect(scores.map((s) => s.strategyId)).not.toContain(88);
  });

  it("excludes canary and smoke by name", () => {
    const canary = makeTrade(77, 1, { strategyName: "canary-tick" });
    const smoke = makeTrade(78, 1, { strategyName: "smoke-test-strat" });
    const real = makeTrade(91, 1);
    const scores = computeResearchEdgeScores([canary, smoke, real]);
    const ids = scores.map((s) => s.strategyId);
    expect(ids).not.toContain(77);
    expect(ids).not.toContain(78);
  });
});

// ── INSUFFICIENT ──────────────────────────────────────────────────────────────

describe("computeResearchEdgeScores — INSUFFICIENT", () => {
  it("< 5 trades → INSUFFICIENT", () => {
    const scores = computeResearchEdgeScores(makeTrades(91, [1, 2, 3, 4]));
    expect(scores[0].status).toBe("INSUFFICIENT");
  });

  it("empty input → empty output", () => {
    expect(computeResearchEdgeScores([])).toEqual([]);
  });
});

// ── DISABLE ───────────────────────────────────────────────────────────────────

describe("computeResearchEdgeScores — DISABLE", () => {
  it("≥5 trades, negative expectancy, fee/gross > 100% → DISABLE", () => {
    // Net pnl deeply negative, large fee drag
    const trades = Array(6)
      .fill(0)
      .map(() => makeTrade(91, -2, { grossPnl: -0.5, fees: 3 })); // fee > gross → fee/gross > 100%
    const scores = computeResearchEdgeScores(trades);
    expect(scores[0].status).toBe("DISABLE");
    expect(scores[0].reason.toLowerCase()).toContain("fee");
  });
});

// ── PROMOTE ───────────────────────────────────────────────────────────────────

describe("computeResearchEdgeScores — PROMOTE", () => {
  it("≥12 trades, positive expectancy, fee/gross < 50%, PF ≥ 1.1 → PROMOTE", () => {
    // All winning trades with low fee ratio
    const trades = Array(14).fill(0).map(() =>
      makeTrade(91, 2, { grossPnl: 2.2, fees: 0.2 }),
    );
    const scores = computeResearchEdgeScores(trades);
    expect(scores[0].status).toBe("PROMOTE");
  });

  it("never promotes with < 12 trades even if all other criteria met", () => {
    const trades = Array(11).fill(0).map(() =>
      makeTrade(91, 2, { grossPnl: 2.2, fees: 0.2 }),
    );
    const scores = computeResearchEdgeScores(trades);
    expect(scores[0].status).not.toBe("PROMOTE");
  });
});

// ── KEEP ──────────────────────────────────────────────────────────────────────

describe("computeResearchEdgeScores — KEEP", () => {
  it("≥12 trades, positive expectancy but fee/gross ≥ 50% → KEEP not PROMOTE", () => {
    const trades = Array(12).fill(0).map(() =>
      makeTrade(91, 0.5, { grossPnl: 1.0, fees: 0.5 }), // fee/gross = 50%
    );
    const scores = computeResearchEdgeScores(trades);
    expect(scores[0].status).toBe("KEEP");
  });
});

// ── WATCH ─────────────────────────────────────────────────────────────────────

describe("computeResearchEdgeScores — WATCH", () => {
  it("5-11 trades, positive expectancy → WATCH (gathering data)", () => {
    const trades = makeTrades(91, Array(8).fill(1));
    const scores = computeResearchEdgeScores(trades);
    expect(scores[0].status).toBe("WATCH");
    expect(scores[0].reason).toMatch(/gather/i);
  });
});

// ── Metrics correctness ───────────────────────────────────────────────────────

describe("computeResearchEdgeScores — metrics", () => {
  it("computes winRate, expectancy, tpHits, slHits correctly", () => {
    const trades = [
      makeTrade(91, 3, { exitReason: "TP" }),
      makeTrade(91, 3, { exitReason: "TP" }),
      makeTrade(91, -1, { exitReason: "SL" }),
      makeTrade(91, -1, { exitReason: "SL" }),
      makeTrade(91, 2, { exitReason: "TP" }),
    ];
    const scores = computeResearchEdgeScores(trades);
    const s = scores[0];
    expect(s.trades).toBe(5);
    expect(s.winRate).toBeCloseTo(0.6);
    expect(s.expectancy).toBeCloseTo((3 + 3 - 1 - 1 + 2) / 5);
    expect(s.tpHits).toBe(3);
    expect(s.slHits).toBe(2);
  });

  it("computes regime breakdown when payload.regime is set", () => {
    const chopTrades = Array(4).fill(0).map(() => makeTrade(91, 1, { regime: "chop" }));
    const trendTrades = Array(4).fill(0).map(() => makeTrade(91, 2, { regime: "trendLow" }));
    const scores = computeResearchEdgeScores([...chopTrades, ...trendTrades]);
    const rb = scores[0].regimeBreakdown;
    expect(rb["chop"]).toBeDefined();
    expect(rb["chop"].trades).toBe(4);
    expect(rb["chop"].expectancy).toBeCloseTo(1);
    expect(rb["trendLow"].expectancy).toBeCloseTo(2);
  });

  it("does not mutate the input array", () => {
    const trades = makeTrades(91, [1, 2, 3]);
    const snapshot = JSON.stringify(trades);
    computeResearchEdgeScores(trades);
    expect(JSON.stringify(trades)).toBe(snapshot);
  });
});

// ── Sorting ───────────────────────────────────────────────────────────────────

describe("computeResearchEdgeScores — sorting", () => {
  it("PROMOTE comes before KEEP comes before WATCH comes before DISABLE", () => {
    const promote = Array(14).fill(0).map(() => makeTrade(91, 2, { grossPnl: 2.2, fees: 0.1 }));
    const disable = Array(6).fill(0).map(() => makeTrade(92, -2, { grossPnl: -0.1, fees: 3 }));
    const watch = makeTrades(93, Array(7).fill(0.5));
    const scores = computeResearchEdgeScores([...promote, ...disable, ...watch]);
    const statuses = scores.map((s) => s.status);
    expect(statuses.indexOf("PROMOTE")).toBeLessThan(statuses.indexOf("WATCH"));
    expect(statuses.indexOf("WATCH")).toBeLessThan(statuses.indexOf("DISABLE"));
  });
});
