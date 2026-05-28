/**
 * Walk-forward validation for paper trade history.
 *
 * Pure function — no I/O. Splits a chronological trade list into rolling
 * in-sample (train) / out-of-sample (test) windows and computes the
 * Walk-Forward Efficiency (WFE) for each window and in aggregate.
 *
 * A strategy must demonstrate positive out-of-sample performance —
 * not only positive in-sample — to be considered validated.
 *
 * WFE definition (per window):
 *   WFE = testExpectancy / |trainExpectancy|
 *   WFE > 0.50 → window passes
 *   WFE < 0.30 → window fails conclusively
 *
 * Aggregate pass: ≥50% of windows pass AND aggregate WFE ≥ 0.50.
 */

// ─── Public types ─────────────────────────────────────────────────────────────

export interface WalkForwardWindow {
  trainStart: string;
  trainEnd: string;
  testStart: string;
  testEnd: string;
  trainTrades: number;
  testTrades: number;
  trainExpectancy: number;
  testExpectancy: number;
  walkForwardEfficiency: number;
  pass: boolean;
}

export interface WalkForwardResult {
  windows: WalkForwardWindow[];
  aggregateWFE: number;
  aggregatePass: boolean;
  reason: string;
  status: "PASS" | "FAIL" | "COLLECT_DATA";
}

export interface WalkForwardTrade {
  closedAt: string;
  netPnl: number;
}

export interface WalkForwardOptions {
  /** Training window length in days (default 21). */
  trainDays?: number;
  /** Test window length in days (default 7). */
  testDays?: number;
  /** Minimum trades in the training window (default 10). */
  minTrainTrades?: number;
  /** Minimum trades in the test window (default 3). */
  minTestTrades?: number;
  /** WFE at or above which a window passes (default 0.50). */
  wfePassThreshold?: number;
  /** WFE below which a window conclusively fails (default 0.30). */
  wfeFailThreshold?: number;
}

// ─── Constants ────────────────────────────────────────────────────────────────

const MS_PER_DAY = 86_400_000;
const DEFAULT_TRAIN_DAYS = 21;
const DEFAULT_TEST_DAYS = 7;
const DEFAULT_MIN_TRAIN = 10;
const DEFAULT_MIN_TEST = 3;
const DEFAULT_WFE_PASS = 0.5;
const DEFAULT_WFE_FAIL = 0.3;

// ─── Helpers ──────────────────────────────────────────────────────────────────

function windowExpectancy(trades: WalkForwardTrade[]): number {
  if (trades.length === 0) return 0;
  return trades.reduce((s, t) => s + t.netPnl, 0) / trades.length;
}

/**
 * Walk-Forward Efficiency for one window.
 * Clipped to [-2, 2] to prevent division artefacts from near-zero train expectancy.
 */
function computeWFE(trainExp: number, testExp: number): number {
  const absTrainExp = Math.abs(trainExp);
  if (absTrainExp < 0.001) return testExp > 0 ? 1 : 0;
  return Math.max(-2, Math.min(2, testExp / absTrainExp));
}

// ─── Main export ──────────────────────────────────────────────────────────────

/**
 * Run walk-forward validation on a set of trades.
 * Returns COLLECT_DATA when there is insufficient history to build any window.
 */
export function runWalkForwardValidation(
  trades: WalkForwardTrade[],
  options: WalkForwardOptions = {},
): WalkForwardResult {
  const {
    trainDays = DEFAULT_TRAIN_DAYS,
    testDays = DEFAULT_TEST_DAYS,
    minTrainTrades = DEFAULT_MIN_TRAIN,
    minTestTrades = DEFAULT_MIN_TEST,
    wfePassThreshold = DEFAULT_WFE_PASS,
    wfeFailThreshold = DEFAULT_WFE_FAIL,
  } = options;

  const minTotal = minTrainTrades + minTestTrades;
  if (trades.length < minTotal) {
    return {
      windows: [],
      aggregateWFE: 0,
      aggregatePass: false,
      reason: `Insufficient data: ${trades.length} trades, need ≥${minTotal} for walk-forward.`,
      status: "COLLECT_DATA",
    };
  }

  // Sort chronologically
  const sorted = [...trades].sort(
    (a, b) => new Date(a.closedAt).getTime() - new Date(b.closedAt).getTime(),
  );

  const startMs = new Date(sorted[0].closedAt).getTime();
  const endMs = new Date(sorted[sorted.length - 1].closedAt).getTime();
  const spanDays = (endMs - startMs) / MS_PER_DAY;
  const requiredDays = trainDays + testDays;

  if (spanDays < requiredDays) {
    return {
      windows: [],
      aggregateWFE: 0,
      aggregatePass: false,
      reason: `Trade history spans ${Math.floor(spanDays)}d — need ≥${requiredDays}d of history.`,
      status: "COLLECT_DATA",
    };
  }

  // Build rolling windows, step = testDays
  const windows: WalkForwardWindow[] = [];
  let windowStartMs = startMs;

  while (windowStartMs + requiredDays * MS_PER_DAY <= endMs) {
    const trainEndMs = windowStartMs + trainDays * MS_PER_DAY;
    const testEndMs = trainEndMs + testDays * MS_PER_DAY;

    const trainTrades = sorted.filter((t) => {
      const ms = new Date(t.closedAt).getTime();
      return ms >= windowStartMs && ms < trainEndMs;
    });
    const testTrades = sorted.filter((t) => {
      const ms = new Date(t.closedAt).getTime();
      return ms >= trainEndMs && ms < testEndMs;
    });

    if (trainTrades.length >= minTrainTrades && testTrades.length >= minTestTrades) {
      const trainExp = windowExpectancy(trainTrades);
      const testExp = windowExpectancy(testTrades);
      const wfe = computeWFE(trainExp, testExp);

      windows.push({
        trainStart: new Date(windowStartMs).toISOString(),
        trainEnd: new Date(trainEndMs).toISOString(),
        testStart: new Date(trainEndMs).toISOString(),
        testEnd: new Date(testEndMs).toISOString(),
        trainTrades: trainTrades.length,
        testTrades: testTrades.length,
        trainExpectancy: trainExp,
        testExpectancy: testExp,
        walkForwardEfficiency: wfe,
        pass: wfe >= wfePassThreshold,
      });
    }

    windowStartMs += testDays * MS_PER_DAY;
  }

  if (windows.length === 0) {
    return {
      windows: [],
      aggregateWFE: 0,
      aggregatePass: false,
      reason: "No complete windows formed — too few trades per window segment.",
      status: "COLLECT_DATA",
    };
  }

  const passCount = windows.filter((w) => w.pass).length;
  const aggWFE = windows.reduce((s, w) => s + w.walkForwardEfficiency, 0) / windows.length;
  const passFraction = passCount / windows.length;
  const aggregatePass = aggWFE >= wfePassThreshold && passFraction >= wfePassThreshold;

  let status: WalkForwardResult["status"];
  let reason: string;

  if (aggregatePass) {
    status = "PASS";
    reason = `${passCount}/${windows.length} windows pass. Aggregate WFE ${(aggWFE * 100).toFixed(0)}% ≥ ${(wfePassThreshold * 100).toFixed(0)}%. Out-of-sample performance replicates in-sample.`;
  } else if (aggWFE < wfeFailThreshold) {
    status = "FAIL";
    reason = `Aggregate WFE ${(aggWFE * 100).toFixed(0)}% < ${(wfeFailThreshold * 100).toFixed(0)}% fail threshold. Out-of-sample performance does not replicate in-sample.`;
  } else {
    status = "FAIL";
    reason = `Aggregate WFE ${(aggWFE * 100).toFixed(0)}% — inconclusive (between ${(wfeFailThreshold * 100).toFixed(0)}% fail and ${(wfePassThreshold * 100).toFixed(0)}% pass). Gather more out-of-sample data.`;
  }

  return { windows, aggregateWFE: aggWFE, aggregatePass, reason, status };
}
