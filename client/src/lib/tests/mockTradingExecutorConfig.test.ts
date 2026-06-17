import { describe, expect, it } from "vitest";
import {
  clampMinSignalScore,
  clampSignalThreshold,
  validateExecutorThresholdInput,
  SIGNAL_THRESHOLD_MIN,
  SIGNAL_THRESHOLD_MAX,
  MIN_SIGNAL_SCORE_MIN,
  MIN_SIGNAL_SCORE_MAX,
} from "@/lib/mockTradingExecutor/executorConfigConstants";

describe("executor threshold config", () => {
  it("clamps signal threshold to 18–32", () => {
    expect(clampSignalThreshold(10)).toBe(SIGNAL_THRESHOLD_MIN);
    expect(clampSignalThreshold(40)).toBe(SIGNAL_THRESHOLD_MAX);
    expect(clampSignalThreshold(26)).toBe(26);
  });

  it("clamps minSignalScore to 30–70", () => {
    expect(clampMinSignalScore(10)).toBe(MIN_SIGNAL_SCORE_MIN);
    expect(clampMinSignalScore(90)).toBe(MIN_SIGNAL_SCORE_MAX);
    expect(clampMinSignalScore(50)).toBe(50);
  });

  it("validates input ranges", () => {
    expect(validateExecutorThresholdInput(26, 50)).toBeNull();
    expect(validateExecutorThresholdInput(10, 50)).toContain("signalThreshold");
    expect(validateExecutorThresholdInput(26, 20)).toContain("minSignalScore");
    expect(validateExecutorThresholdInput(40, 80)).toContain("signalThreshold");
  });
});
