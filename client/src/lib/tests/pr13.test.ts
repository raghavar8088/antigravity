import { describe, it, expect } from "vitest";
import { scoreSignalQuality, rankSignalsByQuality } from "../analytics/futuresSignalQuality";
import { computeMTFConfluence, mtfSkipReason } from "../trading/futuresMTFConfluence";
import { computeAttribution } from "../analytics/futuresAttribution";

const now = Date.now();
const ago = (m: number) => new Date(now - m * 60_000).toISOString();

const goodInput = () => ({
  signalScore: 35,
  atrPct: 0.0012,
  spreadPct: 0.0002,
  volumeRatio: 1.6,
  regime: "trendHigh" as const,
  regimeFitsStrategy: true,
  ema20AboveEma50: true,
  priceAboveEma20: true,
  side: "LONG" as const,
  openPositionCount: 2,
  sameSideCount: 1,
  hoursIntoSession: 10,
  strategyWinRate: 0.55,
  strategyTrades: 30,
  cooldownRemainMs: 0,
});

const mkTFSnap = (tf: "1m" | "5m" | "15m" | "1h" | "4h" | "1d", bull: boolean) => ({
  tf,
  close: 100_000,
  ema20: bull ? 99_000 : 101_000,
  ema50: bull ? 98_000 : 102_000,
  rsi: bull ? 60 : 40,
  atr: 500,
  volumeRatio: 1.2,
  isAvailable: true,
});

const mkTrade = (
  o: Partial<{
    strategy_name: string;
    net_pnl: number;
    gross_pnl: number;
    fees: number;
    exit_reason: string;
    side: string;
    template_family: string;
    opened_at: string;
    closed_at: string;
  }> = {},
) => ({
  strategy_name: "MTF_Trend",
  net_pnl: -20,
  gross_pnl: -15,
  fees: 5,
  exit_reason: "SL",
  side: "SHORT",
  template_family: "mtf",
  opened_at: ago(30),
  closed_at: ago(10),
  ...o,
});

describe("scoreSignalQuality", () => {
  it("passes high-quality signal", () => {
    const result = scoreSignalQuality(goodInput());
    expect(result.pass).toBe(true);
    expect(result.total).toBeGreaterThanOrEqual(50);
  });

  it("fails low ATR signal", () => {
    const result = scoreSignalQuality({
      ...goodInput(),
      atrPct: 0.0001,
    });
    expect(result.deductions).toContain("ATR too low");
  });

  it("gives bonus for volume spike", () => {
    const withSpike = scoreSignalQuality({ ...goodInput(), volumeRatio: 2.0 });
    const withoutSpike = scoreSignalQuality({ ...goodInput(), volumeRatio: 0.9 });
    expect(withSpike.momentumScore).toBeGreaterThan(withoutSpike.momentumScore);
  });

  it("deducts for chop regime", () => {
    const chop = scoreSignalQuality({ ...goodInput(), regime: "chop" });
    const trend = scoreSignalQuality({ ...goodInput(), regime: "trendHigh" });
    expect(chop.regimeScore).toBeLessThan(trend.regimeScore);
  });

  it("deducts for counter-trend long in bear", () => {
    const result = scoreSignalQuality({
      ...goodInput(),
      side: "LONG",
      ema20AboveEma50: false,
      priceAboveEma20: false,
    });
    expect(result.deductions.some((d) => d.includes("Counter-trend"))).toBe(true);
  });

  it("deducts for active cooldown", () => {
    const withCd = scoreSignalQuality({ ...goodInput(), cooldownRemainMs: 60_000 });
    const withoutCd = scoreSignalQuality({ ...goodInput(), cooldownRemainMs: 0 });
    expect(withCd.sessionScore).toBeLessThan(withoutCd.sessionScore);
  });

  it("total is between 0 and 100", () => {
    const result = scoreSignalQuality(goodInput());
    expect(result.total).toBeGreaterThanOrEqual(0);
    expect(result.total).toBeLessThanOrEqual(100);
  });

  it("isHighQuality when total >= 75", () => {
    const result = scoreSignalQuality(goodInput());
    if (result.total >= 75) {
      expect(result.isHighQuality).toBe(true);
    } else {
      expect(result.isMarginal || result.isLowQuality).toBe(true);
    }
  });

  it("exactly one of high/marginal/low is true", () => {
    const result = scoreSignalQuality(goodInput());
    const flags = [result.isHighQuality, result.isMarginal, result.isLowQuality];
    expect(flags.filter(Boolean)).toHaveLength(1);
  });

  it("pass is false when quality < 50", () => {
    const result = scoreSignalQuality({
      ...goodInput(),
      atrPct: 0.0001,
      volumeRatio: 0.3,
      regime: "chop",
      regimeFitsStrategy: false,
      signalScore: 5,
      strategyWinRate: 0.1,
      cooldownRemainMs: 300_000,
      openPositionCount: 10,
      sameSideCount: 3,
    });
    expect(result.pass).toBe(false);
  });
});

describe("rankSignalsByQuality", () => {
  it("returns sorted by quality descending", () => {
    const inputs = [
      { ...goodInput(), strategyId: 1, signalScore: 10 },
      { ...goodInput(), strategyId: 2, signalScore: 40 },
      { ...goodInput(), strategyId: 3, signalScore: 25 },
    ];
    const ranked = rankSignalsByQuality(inputs);
    expect(ranked[0].strategyId).toBe(2);
    expect(ranked[ranked.length - 1].strategyId).toBe(1);
  });
});

describe("computeMTFConfluence", () => {
  it("returns BULLISH when all TFs bullish", () => {
    const snaps = (["1m", "5m", "15m", "1h", "4h", "1d"] as const).map((tf) =>
      mkTFSnap(tf, true),
    );
    const result = computeMTFConfluence(snaps, "LONG");
    expect(result.overallBias).toBe("BULLISH");
    expect(result.agrees).toBe(true);
  });

  it("returns BEARISH when all TFs bearish", () => {
    const snaps = (["1m", "5m", "15m", "1h", "4h", "1d"] as const).map((tf) =>
      mkTFSnap(tf, false),
    );
    const result = computeMTFConfluence(snaps, "SHORT");
    expect(result.overallBias).toBe("BEARISH");
    expect(result.agrees).toBe(true);
  });

  it("does not agree when bias conflicts with signal", () => {
    const snaps = (["1m", "5m", "15m", "1h", "4h", "1d"] as const).map((tf) =>
      mkTFSnap(tf, false),
    );
    const result = computeMTFConfluence(snaps, "LONG");
    expect(result.agrees).toBe(false);
  });

  it("handles no available data gracefully", () => {
    const snaps = (["1m", "5m"] as const).map((tf) => ({
      ...mkTFSnap(tf, true),
      isAvailable: false,
    }));
    const result = computeMTFConfluence(snaps, "LONG");
    expect(result.totalAvailable).toBe(0);
    expect(result.isConfluent).toBe(false);
  });

  it("confluenceScore is between 0 and 100", () => {
    const snaps = (["1m", "5m", "15m", "1h", "4h", "1d"] as const).map((tf) =>
      mkTFSnap(tf, true),
    );
    const result = computeMTFConfluence(snaps, "LONG");
    expect(result.confluenceScore).toBeGreaterThanOrEqual(0);
    expect(result.confluenceScore).toBeLessThanOrEqual(100);
  });

  it("isConfluent requires score >= 60 and agrees", () => {
    const snaps = (["1m", "5m", "15m", "1h", "4h", "1d"] as const).map((tf) =>
      mkTFSnap(tf, true),
    );
    const result = computeMTFConfluence(snaps, "LONG");
    if (result.isConfluent) {
      expect(result.confluenceScore).toBeGreaterThanOrEqual(60);
      expect(result.agrees).toBe(true);
    }
  });
});

describe("mtfSkipReason", () => {
  it("returns null when confluent and agrees", () => {
    const snaps = (["1m", "5m", "15m", "1h", "4h", "1d"] as const).map((tf) =>
      mkTFSnap(tf, true),
    );
    const result = computeMTFConfluence(snaps, "LONG");
    expect(mtfSkipReason(result, 55)).toBeNull();
  });

  it("returns conflict reason when bias disagrees", () => {
    const snaps = (["1m", "5m", "15m", "1h", "4h", "1d"] as const).map((tf) =>
      mkTFSnap(tf, false),
    );
    const result = computeMTFConfluence(snaps, "LONG");
    const reason = mtfSkipReason(result, 55);
    expect(reason).toContain("MTF_BIAS_CONFLICT");
  });
});

describe("computeAttribution", () => {
  it("excludes probe trades", () => {
    const trades = [
      mkTrade({ strategy_name: "PAPER_BOOTSTRAP_PROBE", net_pnl: 9999 }),
      mkTrade({ net_pnl: -20 }),
    ];
    const result = computeAttribution(trades as never);
    expect(result.totalAnalyzed).toBe(1);
  });

  it("bySide includes LONG and SHORT buckets", () => {
    const trades = [
      mkTrade({ side: "LONG", net_pnl: 10, exit_reason: "TP" }),
      mkTrade({ side: "SHORT", net_pnl: -20 }),
    ];
    const result = computeAttribution(trades as never);
    const sides = result.bySide.map((b) => b.label);
    expect(sides).toContain("LONG");
    expect(sides).toContain("SHORT");
  });

  it("byHoldDuration covers all 4 buckets", () => {
    const trades = [
      mkTrade({ opened_at: ago(2), closed_at: ago(1) }),
      mkTrade({ opened_at: ago(10), closed_at: ago(2) }),
      mkTrade({ opened_at: ago(30), closed_at: ago(5) }),
      mkTrade({ opened_at: ago(120), closed_at: ago(10) }),
    ];
    const result = computeAttribution(trades as never);
    const buckets = result.byHoldDuration.map((b) => b.label);
    expect(buckets).toContain("<5m");
    expect(buckets).toContain("5-15m");
    expect(buckets).toContain("15-60m");
    expect(buckets).toContain(">60m");
  });

  it("byExitReason groups correctly", () => {
    const trades = [
      mkTrade({ exit_reason: "SL" }),
      mkTrade({ exit_reason: "SL" }),
      mkTrade({ exit_reason: "TP", net_pnl: 15 }),
    ];
    const result = computeAttribution(trades as never);
    const sl = result.byExitReason.find((b) => b.label === "SL")!;
    const tp = result.byExitReason.find((b) => b.label === "TP")!;
    expect(sl.trades).toBe(2);
    expect(tp.trades).toBe(1);
  });

  it("bestHoldBucket has higher avgNetPnl than worstHoldBucket", () => {
    const trades = [
      mkTrade({ opened_at: ago(2), closed_at: ago(1), net_pnl: 15, exit_reason: "TP" }),
      mkTrade({ opened_at: ago(2), closed_at: ago(1), net_pnl: 10, exit_reason: "TP" }),
      mkTrade({ opened_at: ago(2), closed_at: ago(1), net_pnl: 12, exit_reason: "TP" }),
      mkTrade({ opened_at: ago(90), closed_at: ago(10), net_pnl: -30 }),
      mkTrade({ opened_at: ago(90), closed_at: ago(10), net_pnl: -25 }),
      mkTrade({ opened_at: ago(90), closed_at: ago(10), net_pnl: -28 }),
    ];
    const result = computeAttribution(trades as never);
    if (
      result.bestHoldBucket &&
      result.worstHoldBucket &&
      result.bestHoldBucket !== result.worstHoldBucket
    ) {
      const best = result.byHoldDuration.find((b) => b.label === result.bestHoldBucket)!;
      const worst = result.byHoldDuration.find((b) => b.label === result.worstHoldBucket)!;
      expect(best.avgNetPnl).toBeGreaterThan(worst.avgNetPnl);
    }
  });
});
