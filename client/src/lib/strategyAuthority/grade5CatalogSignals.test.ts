import { describe, expect, it } from "vitest";
import { createTraceRow } from "@/lib/strategySignalTrace";
import { isExecutableTraceRow } from "@/lib/mockTradingEngine";
import {
  catalogStrategyNumericId,
  fanOutGrade5CatalogSignals,
} from "@/lib/strategyAuthority/grade5CatalogSignals";

describe("grade5CatalogSignals", () => {
  it("maps desk candidates to catalog strategies in the same family bucket", () => {
    const baseRows = [
      createTraceRow({
        traceId: "desk-1",
        tickAt: 1_700_000_000_000,
        mode: "browser",
        symbol: "BTCUSD",
        strategyId: 91,
        strategyName: "Trend_Continuation_Long",
        category: "Trend",
        side: "LONG",
        status: "CANDIDATE",
        gate: "OPENED",
        reason: "candidate",
        signalScore: 72,
        requiredThreshold: 50,
        confirmPassed: true,
        regime: "trend",
        regimeAllowed: true,
        feeHurdlePassed: true,
      }),
    ];

    const rows = fanOutGrade5CatalogSignals({
      catalog: [
        {
          id: "EMACrossScalper_8_21",
          name: "EMA Cross Scalper (8/21)",
          family: "Trend",
          category: "Trend",
          timeframe: "1m",
        },
        {
          id: "RSIOversold30Scalp",
          name: "RSI Oversold (30)",
          family: "Mean Reversion",
          category: "Mean Reversion",
          timeframe: "1m",
        },
      ],
      baseRows,
      tickAt: 1_700_000_000_000,
      symbol: "BTCUSD",
      regime: "trend",
    });

    expect(rows.length).toBe(2);
    expect(rows.every((row) => isExecutableTraceRow(row))).toBe(true);
    expect(rows.find((row) => row.ispapStrategyId === "EMACrossScalper_8_21")?.side).toBe("LONG");
    expect(rows.find((row) => row.ispapStrategyId === "RSIOversold30Scalp")?.side).toBe("LONG");
    expect(rows[0]?.pipelineStage).toBe("GRADE_5");
  });

  it("returns empty rows when no executable desk candidates exist", () => {
    const rows = fanOutGrade5CatalogSignals({
      catalog: [
        {
          id: "EMACrossScalper_8_21",
          name: "EMA Cross Scalper (8/21)",
          family: "Trend",
          category: "Trend",
          timeframe: "1m",
        },
      ],
      baseRows: [
        createTraceRow({
          traceId: "desk-reject",
          tickAt: 1,
          mode: "browser",
          symbol: "BTCUSD",
          strategyId: 91,
          strategyName: "Trend_Continuation_Long",
          category: "Trend",
          side: "LONG",
          status: "REJECTED",
          gate: "CONFIRM",
          reason: "failed confirm",
          signalScore: 72,
          requiredThreshold: 50,
          confirmPassed: false,
          regime: "trend",
          regimeAllowed: true,
        }),
      ],
      tickAt: 1,
      symbol: "BTCUSD",
      regime: "trend",
    });

    expect(rows).toEqual([]);
  });

  it("uses stable numeric ids for catalog slugs", () => {
    const a = catalogStrategyNumericId("EMACrossScalper_8_21");
    const b = catalogStrategyNumericId("EMACrossScalper_8_21");
    const c = catalogStrategyNumericId("RSIOversold30Scalp");
    expect(a).toBe(b);
    expect(a).toBeGreaterThanOrEqual(900_000);
    expect(c).not.toBe(a);
  });
});
