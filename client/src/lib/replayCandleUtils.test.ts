/**
 * Tests for replay candle utilities — dedup, coverage, gap counting, and the
 * coverage guard logic used by replay-walkforward and the API route.
 *
 * All pure-function tests; no network calls, no filesystem.
 */
import { describe, it, expect } from "vitest";
import {
  deduplicateAndSortCandles,
  countCandleGaps,
  normalizeCandleTimeMs,
} from "@/lib/futuresKlinesFetch";
import {
  computeCoverageDays,
  hasSufficientCoverage,
} from "@/lib/futuresReplayFixtures";
import type { ReplayCandle } from "@/lib/futuresReplayFixtures";

// ─── Helpers ─────────────────────────────────────────────────────────────────

function makeCandle(timeMs: number, close = 65000): ReplayCandle {
  return { time: timeMs, open: close, high: close * 1.001, low: close * 0.999, close, volume: 1 };
}

/** Build a clean 1-minute candle series starting at `startMs`. */
function makeSeriesMs(count: number, startMs = 1_700_000_000_000): ReplayCandle[] {
  return Array.from({ length: count }, (_, i) => makeCandle(startMs + i * 60_000));
}

// ─── deduplicateAndSortCandles ────────────────────────────────────────────────

describe("deduplicateAndSortCandles", () => {
  it("returns empty array for empty input", () => {
    expect(deduplicateAndSortCandles([])).toEqual([]);
  });

  it("deduplicates candles with the same timestamp", () => {
    const dup = makeCandle(1000);
    const unique = makeCandle(2000);
    const result = deduplicateAndSortCandles([dup, unique, { ...dup, close: 99999 }]);
    expect(result).toHaveLength(2);
  });

  it("keeps the last occurrence when duplicates exist (Map.set overwrites)", () => {
    const first = makeCandle(1000, 65000);
    const second = makeCandle(1000, 70000); // same time, different close — last wins
    const [kept] = deduplicateAndSortCandles([first, second]);
    expect(kept!.close).toBe(70000);
  });

  it("sorts ascending by time", () => {
    const candles = [makeCandle(3000), makeCandle(1000), makeCandle(2000)];
    const result = deduplicateAndSortCandles(candles);
    expect(result.map((c) => c.time)).toEqual([1000, 2000, 3000]);
  });

  it("handles two pages stitched together (no duplicates expected)", () => {
    const page1 = makeSeriesMs(500, 1_700_000_000_000);
    const page2 = makeSeriesMs(500, 1_700_000_000_000 + 500 * 60_000);
    const merged = deduplicateAndSortCandles([...page1, ...page2]);
    expect(merged).toHaveLength(1000);
  });

  it("deduplicates overlapping pages correctly", () => {
    // Simulate two paged fetches with a 100-candle overlap
    const page1 = makeSeriesMs(500, 1_700_000_000_000);
    // page2 starts 400 candles in — 100-candle overlap
    const page2 = makeSeriesMs(500, 1_700_000_000_000 + 400 * 60_000);
    const merged = deduplicateAndSortCandles([...page1, ...page2]);
    // 500 + 500 - 100 overlap = 900 unique
    expect(merged).toHaveLength(900);
  });
});

// ─── countCandleGaps ─────────────────────────────────────────────────────────

describe("countCandleGaps", () => {
  it("returns 0 for empty or single-candle input", () => {
    expect(countCandleGaps([])).toBe(0);
    expect(countCandleGaps([makeCandle(1000)])).toBe(0);
  });

  it("returns 0 for a clean 1m series (60s gaps exactly)", () => {
    const candles = makeSeriesMs(100);
    expect(countCandleGaps(candles)).toBe(0);
  });

  it("counts a single >2min gap", () => {
    const candles = [
      makeCandle(0),
      makeCandle(60_000),       // +1m — ok
      makeCandle(300_000),      // +4m — gap!
      makeCandle(360_000),
    ];
    expect(countCandleGaps(candles)).toBe(1);
  });

  it("counts multiple gaps", () => {
    const candles = [
      makeCandle(0),
      makeCandle(600_000),  // gap 1
      makeCandle(660_000),
      makeCandle(1_500_000), // gap 2
    ];
    expect(countCandleGaps(candles)).toBe(2);
  });

  it("uses custom threshold", () => {
    const candles = [makeCandle(0), makeCandle(90_000), makeCandle(180_000)];
    // default threshold 120s → 90s gap is fine
    expect(countCandleGaps(candles, 120_000)).toBe(0);
    // threshold 60s → both 90s gaps count
    expect(countCandleGaps(candles, 60_000)).toBe(2);
  });
});

// ─── normalizeCandleTimeMs ────────────────────────────────────────────────────

describe("normalizeCandleTimeMs", () => {
  it("returns 0 for invalid input", () => {
    expect(normalizeCandleTimeMs(0)).toBe(0);
    expect(normalizeCandleTimeMs(NaN)).toBe(0);
  });

  it("converts Unix seconds to ms when t < 1e12", () => {
    const sec = 1_700_000_000;
    expect(normalizeCandleTimeMs(sec)).toBe(sec * 1000);
  });

  it("passes through already-ms timestamps", () => {
    const ms = 1_700_000_000_000;
    expect(normalizeCandleTimeMs(ms)).toBe(ms);
  });
});

// ─── computeCoverageDays ─────────────────────────────────────────────────────

describe("computeCoverageDays", () => {
  it("returns 0 for 0 candles", () => {
    expect(computeCoverageDays(0)).toBe(0);
  });

  it("500 candles ≈ 0.347 days", () => {
    const result = computeCoverageDays(500);
    expect(result).toBeCloseTo(500 / 1440, 4);
  });

  it("1440 candles = exactly 1 day", () => {
    expect(computeCoverageDays(1440)).toBe(1);
  });

  it("43200 candles = exactly 30 days", () => {
    expect(computeCoverageDays(43200)).toBe(30);
  });
});

// ─── hasSufficientCoverage ────────────────────────────────────────────────────

describe("hasSufficientCoverage", () => {
  it("returns false when 500 candles are passed as 30d coverage", () => {
    expect(hasSufficientCoverage(500, 30)).toBe(false);
  });

  it("returns true when coverage is exactly 80% of requested", () => {
    const requestedDays = 30;
    const needed = Math.ceil(requestedDays * 1440 * 0.8); // 34560
    expect(hasSufficientCoverage(needed, requestedDays)).toBe(true);
  });

  it("returns true for full 30-day coverage", () => {
    expect(hasSufficientCoverage(43200, 30)).toBe(true);
  });

  it("returns false for 79% coverage", () => {
    const requestedDays = 30;
    const shortfall = Math.floor(requestedDays * 1440 * 0.79);
    expect(hasSufficientCoverage(shortfall, requestedDays)).toBe(false);
  });

  it("returns true for 1-day with 1440 candles", () => {
    expect(hasSufficientCoverage(1440, 1)).toBe(true);
  });

  it("returns false for 1-day with only 500 candles", () => {
    // 500 / 1440 = 34.7% < 80%
    expect(hasSufficientCoverage(500, 1)).toBe(false);
  });
});

// ─── Coverage guard logic (mirrors replay-walkforward check) ─────────────────

describe("Coverage guard — replay-walkforward refuses insufficient data", () => {
  /**
   * Simulates the guard used in replay-walkforward.ts:
   *   if (!sufficient) → print error, exit
   */
  function guardCheck(candles: number, requestedDays: number): { ok: boolean; message: string } {
    const sufficient = hasSufficientCoverage(candles, requestedDays);
    if (!sufficient) {
      return {
        ok: false,
        message:
          `Insufficient replay data: ${candles} candles = ` +
          `${computeCoverageDays(candles).toFixed(1)}d coverage ` +
          `(need ≥80% of ${requestedDays}d). ` +
          `Run: npm run replay:fetch -- --days=${requestedDays}`,
      };
    }
    return { ok: true, message: "ok" };
  }

  it("blocks a 30d run with only 500 candles", () => {
    const result = guardCheck(500, 30);
    expect(result.ok).toBe(false);
    expect(result.message).toContain("Insufficient");
    expect(result.message).toContain("--days=30");
  });

  it("allows a 30d run with 43200 candles", () => {
    expect(guardCheck(43200, 30).ok).toBe(true);
  });

  it("allows a 30d run with 35000 candles (>80%)", () => {
    expect(guardCheck(35000, 30).ok).toBe(true);
  });

  it("blocks a 7d run with only 500 candles", () => {
    expect(guardCheck(500, 7).ok).toBe(false);
  });

  it("allows a 7d run with 10000 candles (>80% of 10080)", () => {
    expect(guardCheck(10000, 7).ok).toBe(true);
  });
});
