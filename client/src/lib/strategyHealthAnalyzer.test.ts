/**
 * Tests for strategyHealthAnalyzer.
 * Pure-function tests — no I/O, no MongoDB.
 */
import { describe, it, expect } from "vitest";
import {
  analyzeStrategyHealth,
  summarizeRosterHealth,
  type AnalyzerTradeInput,
} from "./strategyHealthAnalyzer";

// ── helpers ───────────────────────────────────────────────────────────────────

function trade(
  strategyId: number,
  netPnl: number,
  closedAtIso: string,
  name = `strat-${strategyId}`,
): AnalyzerTradeInput {
  return {
    strategy_id: strategyId,
    strategy_name: name,
    net_pnl: netPnl,
    closed_at: closedAtIso,
  };
}

function tradesForStrategy(
  strategyId: number,
  pnls: number[],
  baseDate = "2026-05-01",
  name?: string,
): AnalyzerTradeInput[] {
  return pnls.map((pnl, i) => {
    const d = new Date(`${baseDate}T${String(i).padStart(2, "0")}:00:00Z`);
    return trade(strategyId, pnl, d.toISOString(), name);
  });
}

// ── Empty + edge cases ────────────────────────────────────────────────────────

describe("analyzeStrategyHealth — basic", () => {
  it("empty input → empty array", () => {
    expect(analyzeStrategyHealth([])).toEqual([]);
  });

  it("one trade → 'thin' classification", () => {
    const reports = analyzeStrategyHealth([trade(91, 5.0, "2026-05-01T00:00:00Z")]);
    expect(reports).toHaveLength(1);
    expect(reports[0].classification).toBe("thin");
    expect(reports[0].trades).toBe(1);
  });

  it("ignores trades with non-finite strategy_id", () => {
    const bad = { ...trade(0, 1, "2026-05-01T00:00:00Z"), strategy_id: NaN };
    const reports = analyzeStrategyHealth([bad, trade(91, 1, "2026-05-01T00:00:00Z")]);
    expect(reports.map((r) => r.strategyId)).toEqual([91]);
  });
});

// ── Winner classification ─────────────────────────────────────────────────────

describe("analyzeStrategyHealth — winners", () => {
  it("20 trades, 60% win rate → winner", () => {
    const pnls = [...Array(12).fill(2), ...Array(8).fill(-1)]; // 12 wins, 8 losses
    const reports = analyzeStrategyHealth(tradesForStrategy(91, pnls));
    expect(reports[0].classification).toBe("winner");
    expect(reports[0].wins).toBe(12);
    expect(reports[0].losses).toBe(8);
    expect(reports[0].winRate).toBeCloseTo(0.6);
  });

  it("winner with low volatility — keeps winner classification", () => {
    const pnls = Array(20).fill(0).map((_, i) => (i < 14 ? 2 : -1)); // steady
    const reports = analyzeStrategyHealth(tradesForStrategy(91, pnls));
    expect(reports[0].classification).toBe("winner");
  });
});

// ── Loser classification ──────────────────────────────────────────────────────

describe("analyzeStrategyHealth — losers", () => {
  it("20 trades, 20% win rate → loser", () => {
    const pnls = [...Array(4).fill(1), ...Array(16).fill(-2)];
    const reports = analyzeStrategyHealth(tradesForStrategy(91, pnls));
    expect(reports[0].classification).toBe("loser");
    expect(reports[0].winRate).toBeCloseTo(0.2);
    expect(reports[0].recommendation.toLowerCase()).toContain("disable");
  });

  it("recommendation never suggests lowering thresholds for losers", () => {
    const pnls = Array(20).fill(-1);
    const reports = analyzeStrategyHealth(tradesForStrategy(91, pnls));
    const rec = reports[0].recommendation.toLowerCase();
    expect(rec).not.toContain("lower threshold");
    expect(rec).not.toContain("reduce threshold");
    expect(rec).not.toContain("decrease threshold");
  });
});

// ── Volatile classification ───────────────────────────────────────────────────

describe("analyzeStrategyHealth — volatile", () => {
  it("high win rate but very high σ/μ → volatile", () => {
    // 60% win rate, but huge swings: +100, -50, +100, -50, ...
    const pnls = Array(20).fill(0).map((_, i) => (i % 2 === 0 ? 100 : -50));
    // winRate 50% — would NOT classify as winner. Use a winner-leaning pattern instead.
    const winnerVol = [
      100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100,
      -200, -200, -200, -200, -200, -200, -200, -200, // 12/20 wins, big swings
    ];
    const reports = analyzeStrategyHealth(tradesForStrategy(91, winnerVol));
    expect(reports[0].classification).toBe("volatile");
  });
});

// ── Thin classification ───────────────────────────────────────────────────────

describe("analyzeStrategyHealth — thin", () => {
  it("19 trades → thin regardless of winRate", () => {
    const pnls = Array(19).fill(5); // 100% wins but only 19 trades
    const reports = analyzeStrategyHealth(tradesForStrategy(91, pnls));
    expect(reports[0].classification).toBe("thin");
  });

  it("custom minTradesForVerdict overrides default", () => {
    const pnls = [...Array(7).fill(2), ...Array(3).fill(-1)]; // 70% win, 10 trades
    const reports = analyzeStrategyHealth(tradesForStrategy(91, pnls), {
      minTradesForVerdict: 5,
    });
    expect(reports[0].classification).toBe("winner");
  });
});

// ── Window slicing ────────────────────────────────────────────────────────────

describe("analyzeStrategyHealth — windowSize", () => {
  it("uses only the most recent N trades per strategy", () => {
    // 30 trades total: first 10 are losses, last 20 are wins
    const losses = tradesForStrategy(91, Array(10).fill(-5), "2026-04-01", "strat");
    const wins = tradesForStrategy(91, Array(20).fill(5), "2026-05-01", "strat");
    const reports = analyzeStrategyHealth([...losses, ...wins], { windowSize: 20 });
    expect(reports[0].trades).toBe(20);
    expect(reports[0].wins).toBe(20);
    expect(reports[0].losses).toBe(0);
    expect(reports[0].classification).toBe("winner");
  });

  it("smaller windowSize narrows the view", () => {
    const pnls = [...Array(15).fill(-1), ...Array(5).fill(5)];
    const reports = analyzeStrategyHealth(tradesForStrategy(91, pnls), {
      windowSize: 5,
      minTradesForVerdict: 5,
    });
    expect(reports[0].trades).toBe(5);
    expect(reports[0].wins).toBe(5);
    expect(reports[0].classification).toBe("winner");
  });
});

// ── Multi-strategy ────────────────────────────────────────────────────────────

describe("analyzeStrategyHealth — multiple strategies", () => {
  it("groups by strategy_id and sorts by total net PnL descending", () => {
    const a = tradesForStrategy(91, Array(20).fill(2), "2026-05-01"); // +40
    const b = tradesForStrategy(92, Array(20).fill(-1), "2026-05-01"); // -20
    const c = tradesForStrategy(95, Array(20).fill(0.5), "2026-05-01"); // +10
    const reports = analyzeStrategyHealth([...a, ...b, ...c]);
    expect(reports.map((r) => r.strategyId)).toEqual([91, 95, 92]);
    expect(reports[0].totalNetPnlUsd).toBe(40);
    expect(reports[1].totalNetPnlUsd).toBe(10);
    expect(reports[2].totalNetPnlUsd).toBe(-20);
  });

  it("each strategy gets its own window", () => {
    const a = tradesForStrategy(91, Array(25).fill(1));
    const b = tradesForStrategy(92, Array(5).fill(1));
    const reports = analyzeStrategyHealth([...a, ...b]);
    const r91 = reports.find((r) => r.strategyId === 91)!;
    const r92 = reports.find((r) => r.strategyId === 92)!;
    expect(r91.trades).toBe(20);
    expect(r92.trades).toBe(5);
    expect(r92.classification).toBe("thin");
  });
});

// ── summarizeRosterHealth ─────────────────────────────────────────────────────

describe("summarizeRosterHealth", () => {
  it("buckets strategy IDs by classification", () => {
    const reports = analyzeStrategyHealth([
      ...tradesForStrategy(91, Array(20).fill(5)), // winner
      ...tradesForStrategy(92, Array(20).fill(-5)), // loser
      ...tradesForStrategy(95, Array(5).fill(5)), // thin
    ]);
    const summary = summarizeRosterHealth(reports);
    expect(summary.total).toBe(3);
    expect(summary.winners).toEqual([91]);
    expect(summary.losers).toEqual([92]);
    expect(summary.thin).toEqual([95]);
  });

  it("empty input → zero buckets", () => {
    const summary = summarizeRosterHealth([]);
    expect(summary).toEqual({ total: 0, winners: [], losers: [], volatile: [], thin: [] });
  });
});

// ── Hard invariants ───────────────────────────────────────────────────────────

describe("analyzeStrategyHealth — hard invariants", () => {
  it("recommendation never tells operator to lower thresholds", () => {
    const cases = [
      analyzeStrategyHealth(tradesForStrategy(91, Array(20).fill(5))),
      analyzeStrategyHealth(tradesForStrategy(92, Array(20).fill(-5))),
      analyzeStrategyHealth(tradesForStrategy(95, Array(5).fill(0))),
    ];
    for (const reports of cases) {
      for (const r of reports) {
        const rec = r.recommendation.toLowerCase();
        expect(rec).not.toContain("lower threshold");
        expect(rec).not.toContain("reduce threshold");
        expect(rec).not.toContain("decrease threshold");
        expect(rec).not.toContain("bypass gate");
      }
    }
  });

  it("does not delete or mutate the input array", () => {
    const input: AnalyzerTradeInput[] = tradesForStrategy(91, [1, 2, 3]);
    const snapshot = JSON.stringify(input);
    analyzeStrategyHealth(input);
    expect(JSON.stringify(input)).toBe(snapshot);
  });
});
