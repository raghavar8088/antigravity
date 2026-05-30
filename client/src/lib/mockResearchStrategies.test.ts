import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import type { OHLCVCandle } from "@/lib/mockResearchIndicators";
import {
  ALL_RESEARCH_FAMILIES,
  RESEARCH_STRATEGIES,
  type ResearchFamily,
} from "@/lib/mockResearchStrategies";
import {
  DEFAULT_RESEARCH_RUNNER_CONFIG,
  evaluateMockResearchStrategies,
  type ResearchRunnerConfig,
} from "@/hooks/useMockResearchRunner";
import {
  buildMockTradeFromResearchSignal,
  DEFAULT_MOCK_TRADING_CONFIG,
} from "@/lib/mockTradingEngine";
import { BTC_RESEARCH_STRATEGIES, type BtcResearchFamily } from "@/lib/btcResearchStrategyRegistry";

const T0 = 1_700_000_000_000;

function downtrendCandles(count = 150): OHLCVCandle[] {
  let price = 70_000;
  const candles: OHLCVCandle[] = [];
  for (let i = 0; i < count; i++) {
    const open = price;
    const close = price - 120;
    candles.push({
      time: T0 + i * 60_000,
      open,
      high: open + 20,
      low: close - 20,
      close,
      volume: 1_000 + i,
    });
    price = close;
  }
  return candles;
}

function sweepCandles(): OHLCVCandle[] {
  const candles: OHLCVCandle[] = [];
  for (let i = 0; i < 30; i++) {
    candles.push({
      time: T0 + i * 60_000,
      open: 60_000,
      high: 60_050,
      low: 59_950,
      close: 60_000,
      volume: 1_000,
    });
  }
  candles.push({
    time: T0 + 31 * 60_000,
    open: 60_000,
    high: 60_020,
    low: 59_800,
    close: 59_980,
    volume: 1_500,
  });
  return candles;
}

function runnerConfig(families: ResearchFamily[], overrides: Partial<ResearchRunnerConfig> = {}): ResearchRunnerConfig {
  return {
    ...DEFAULT_RESEARCH_RUNNER_CONFIG,
    enabledFamilies: new Set(families),
    enabledBtcFamilies: new Set() as ResearchRunnerConfig["enabledBtcFamilies"],
    maxSignalsPerMinute: 500,
    minConfidence: 0,
    ...overrides,
  };
}

describe("RESEARCH_STRATEGIES registry", () => {
  it("contains exactly 500 enabled strategies", () => {
    expect(RESEARCH_STRATEGIES).toHaveLength(500);
    expect(RESEARCH_STRATEGIES.every((strategy) => strategy.enabled === true)).toBe(true);
  });

  it("uses unique strategy ids", () => {
    const ids = RESEARCH_STRATEGIES.map((strategy) => strategy.id);
    expect(new Set(ids).size).toBe(500);
  });

  it("covers every configured research family", () => {
    const actualFamilies = new Set(RESEARCH_STRATEGIES.map((strategy) => strategy.family));
    for (const family of ALL_RESEARCH_FAMILIES) {
      expect(actualFamilies.has(family)).toBe(true);
    }
  });
});

describe("evaluateMockResearchStrategies", () => {
  it("respects enabled family filters", () => {
    const result = evaluateMockResearchStrategies(
      downtrendCandles(),
      runnerConfig(["RsiMeanReversion"]),
      T0,
    );

    expect(result.evaluatedCount).toBeGreaterThan(0);
    expect(result.signals.length).toBeGreaterThan(0);
    expect(result.diagnostics.funnel.signalsGenerated).toBeGreaterThanOrEqual(result.signals.length);
    expect(result.diagnostics.funnel.confidencePassed).toBeGreaterThanOrEqual(result.signals.length);
    expect(result.signals.every((signal) => signal.family === "RsiMeanReversion")).toBe(true);
  });

  it("creates mock trades from BUY/SELL research signals", () => {
    const result = evaluateMockResearchStrategies(
      downtrendCandles(),
      runnerConfig(["RsiMeanReversion"]),
      T0,
    );
    const signal = result.signals.find((candidate) => candidate.side === "BUY" || candidate.side === "SELL");
    expect(signal).toBeDefined();

    const trade = buildMockTradeFromResearchSignal({
      signal: signal!,
      currentPrice: 60_000,
      config: { ...DEFAULT_MOCK_TRADING_CONFIG, minSignalScore: 0 },
      now: T0,
    });

    expect(trade).not.toBeNull();
    expect(trade?.researchPack).toBe(true);
    expect(trade?.strategyFamily).toBe(signal?.family);
    expect(trade?.confidenceScore).toBe(signal?.confidenceScore);
    expect(trade?.strategyParams).toEqual(signal?.params);
  });

  it("caps emitted signals by confidence", () => {
    const result = evaluateMockResearchStrategies(
      downtrendCandles(),
      runnerConfig(["RsiMeanReversion"], { maxSignalsPerMinute: 2 }),
      T0,
    );

    expect(result.signals.length).toBeLessThanOrEqual(2);
    expect(result.signals[0]?.confidence ?? 0).toBeGreaterThanOrEqual(result.signals[1]?.confidence ?? 0);
  });

  it("profit mode only emits approved top-ranked BTC research strategies", () => {
    const stopHuntLong = BTC_RESEARCH_STRATEGIES.find(
      (strategy) => strategy.family === "StopHuntSfp" && strategy.side === "LONG",
    );
    expect(stopHuntLong).toBeDefined();

    const baseConfig = runnerConfig([], {
      enabledBtcFamilies: new Set<BtcResearchFamily>(["StopHuntSfp"]),
      selectionMode: "PROFIT_MODE",
      minConfidence: 0,
    });

    const blocked = evaluateMockResearchStrategies(sweepCandles(), {
      ...baseConfig,
      approvedStrategyIds: new Set<number>(),
    }, T0);
    expect(blocked.signals).toHaveLength(0);
    expect(blocked.diagnostics.rejectionCounts.NO_APPROVED_STRATEGY).toBeGreaterThan(0);

    const approved = evaluateMockResearchStrategies(sweepCandles(), {
      ...baseConfig,
      approvedStrategyIds: new Set<number>([stopHuntLong!.id]),
    }, T0);
    expect(approved.signals.every((signal) => signal.strategyId === stopHuntLong!.id)).toBe(true);
    expect(approved.signals.length).toBeGreaterThan(0);
  });

  it("records low-confidence diagnostic rejections before signals reach the engine", () => {
    const result = evaluateMockResearchStrategies(
      downtrendCandles(),
      runnerConfig(["RsiMeanReversion"], { minConfidence: 101 }),
      T0,
    );

    expect(result.signals).toHaveLength(0);
    expect(result.diagnostics.funnel.signalsGenerated).toBeGreaterThan(0);
    expect(result.diagnostics.funnel.confidencePassed).toBe(0);
    expect(result.diagnostics.rejectionCounts.LOW_CONFIDENCE).toBeGreaterThan(0);
  });

  it("regime mode requires an approved historically successful regime strategy", () => {
    const stopHuntLong = BTC_RESEARCH_STRATEGIES.find(
      (strategy) => strategy.family === "StopHuntSfp" && strategy.side === "LONG",
    );
    expect(stopHuntLong).toBeDefined();

    const config = runnerConfig([], {
      enabledBtcFamilies: new Set<BtcResearchFamily>(["StopHuntSfp"]),
      selectionMode: "REGIME_MODE",
      minConfidence: 0,
      approvedStrategyIds: new Set<number>([stopHuntLong!.id]),
    });

    const result = evaluateMockResearchStrategies(sweepCandles(), config, "RANGING", T0);
    expect(result.signals.length).toBeGreaterThan(0);
    expect(result.signals.every((signal) => signal.regimeAtEntry === "RANGING")).toBe(true);
    expect(result.signals.every((signal) => signal.strategyId === stopHuntLong!.id)).toBe(true);
  });
});

describe("mock research isolation", () => {
  it("does not import real broker or order APIs from mock research files", () => {
    const files = [
      "src/lib/mockResearchStrategies.ts",
      "src/hooks/useMockResearchRunner.ts",
      "src/lib/mockCandleBuilder.ts",
      "src/hooks/useMockCandleBuilder.ts",
    ];
    const forbiddenImport = /from\s+["']@\/(?:lib|hooks|app)\/(?:paperOms|broker|delta|angel|orders?|liveOrder)[^"']*["']/i;
    const forbiddenCall = /\b(?:placeOrder|createOrder|submitOrder|sendOrder)\s*\(/i;

    for (const file of files) {
      const source = readFileSync(join(process.cwd(), file), "utf8");
      expect(source).not.toMatch(forbiddenImport);
      expect(source).not.toMatch(forbiddenCall);
    }
  });
});
