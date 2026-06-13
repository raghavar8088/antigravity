import { describe, expect, it } from "vitest";
import { recommendOneTune } from "../trading/futuresParameterTuner";
import type { PaperTradeDbRow } from "../portfolio/paperTradesTypes";

const BASE: PaperTradeDbRow = {
  id: "t1",
  created_at: "2024-06-01T00:00:00Z",
  account_key: "acc",
  client_trade_id: "c1",
  opened_at: "2024-06-01T00:00:00Z",
  closed_at: "2024-06-01T00:10:00Z",
  symbol: "BTCUSD",
  strategy_id: 91,
  strategy_name: "Trend_Continuation_Long",
  side: "LONG",
  entry_price: 50000,
  exit_price: 49900,
  contracts: 1,
  notional: 100,
  margin_used: 4,
  gross_pnl: -15,
  fees: 5,
  funding_costs: 0,
  net_pnl: -20,
  exit_reason: "SL",
  payload: null,
};

function mkRow(
  overrides: Partial<PaperTradeDbRow> & { closed_at?: string },
  idx: number,
): PaperTradeDbRow {
  const closed = overrides.closed_at ?? `2024-06-01T${String(10 + idx).padStart(2, "0")}:00:00Z`;
  return {
    ...BASE,
    id: `t${idx}`,
    client_trade_id: `c${idx}`,
    closed_at: closed,
    ...overrides,
  };
}

function many(
  n: number,
  overrides: Partial<PaperTradeDbRow> = {},
): PaperTradeDbRow[] {
  return Array.from({ length: n }, (_, i) => mkRow(overrides, i));
}

describe("recommendOneTune", () => {
  it("returns NO_CHANGE when fewer than 10 production trades", () => {
    const r = recommendOneTune(many(5), 28, 1.5, 0.5, 2);
    expect(r.target).toBe("NO_CHANGE");
    expect(r.tradesAnalyzed).toBe(5);
  });

  it("excludes probe trades from analysis", () => {
    const trades = [
      ...many(9),
      mkRow({ strategy_name: "PAPER_BOOTSTRAP_PROBE", net_pnl: 999, gross_pnl: 1000, fees: 1 }, 99),
    ];
    const r = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(r.tradesAnalyzed).toBe(9);
    expect(r.target).toBe("NO_CHANGE");
  });

  it("Rule 2: high fee ratio recommends SIGNAL_THRESHOLD +4", () => {
    const trades = many(12, {
      gross_pnl: 2,
      fees: 5,
      net_pnl: -3,
      exit_reason: "TIME",
    });
    const r = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(r.target).toBe("SIGNAL_THRESHOLD");
    expect(r.suggestedValue).toBe(32);
    expect(r.delta).toBe(4);
  });

  it("Rule 3a: high SL + short hold recommends threshold raise", () => {
    const trades = many(12, {
      exit_reason: "SL",
      net_pnl: -15,
      gross_pnl: -10,
      fees: 5,
      opened_at: "2024-06-01T00:00:00Z",
      closed_at: "2024-06-01T00:02:00Z",
    });
    const r = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(r.target).toBe("SIGNAL_THRESHOLD");
    expect(r.suggestedValue).toBe(31);
  });

  it("Rule 3b: high SL + longer hold recommends SL widen", () => {
    const trades = many(12, {
      exit_reason: "SL",
      net_pnl: -15,
      gross_pnl: -10,
      fees: 5,
      opened_at: "2024-06-01T00:00:00Z",
      closed_at: "2024-06-01T00:15:00Z",
    });
    const r = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(r.target).toBe("SL_PCT");
    expect(r.suggestedValue).toBeGreaterThan(0.5);
  });

  it("Rule 4: near-zero TP rate recommends TP tighten", () => {
    const trades = many(20, {
      exit_reason: "TIME",
      net_pnl: 1,
      gross_pnl: 2,
      fees: 1,
      opened_at: "2024-06-01T00:00:00Z",
      closed_at: "2024-06-01T00:25:00Z",
    });
    const r = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(r.target).toBe("TP_PCT");
    expect(r.suggestedValue).toBeLessThan(1.5);
  });

  it("Rule 5: clustered SL recommends SAME_SIDE_CAP → 1", () => {
    const losses = many(17, {
      exit_reason: "SL",
      net_pnl: -5,
      gross_pnl: -3,
      fees: 2,
    });
    const wins = many(3, {
      exit_reason: "TP",
      net_pnl: 80,
      gross_pnl: 82,
      fees: 2,
    });
    const r = recommendOneTune([...losses, ...wins], 28, 1.5, 0.5, 2);
    expect(r.target).toBe("SAME_SIDE_CAP");
    expect(r.suggestedValue).toBe(1);
  });

  it("Rule 6: healthy metrics return NO_CHANGE", () => {
    const trades = many(15, {
      exit_reason: "TP",
      net_pnl: 12,
      gross_pnl: 14,
      fees: 2,
    });
    const r = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(r.target).toBe("NO_CHANGE");
    expect(r.confidence).toBe("HIGH");
  });
});
