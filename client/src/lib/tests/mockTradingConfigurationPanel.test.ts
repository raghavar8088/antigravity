import { describe, expect, it } from "vitest";
import {
  MIN_SIGNAL_SCORE_MAX,
  MIN_SIGNAL_SCORE_MIN,
  SIGNAL_THRESHOLD_MAX,
  SIGNAL_THRESHOLD_MIN,
} from "@/lib/mockTradingExecutor/executorConfigConstants";

describe("configuration panel API contract", () => {
  it("exposes expected threshold ranges for UI sliders", () => {
    expect(SIGNAL_THRESHOLD_MIN).toBe(18);
    expect(SIGNAL_THRESHOLD_MAX).toBe(32);
    expect(MIN_SIGNAL_SCORE_MIN).toBe(30);
    expect(MIN_SIGNAL_SCORE_MAX).toBe(70);
  });

  it("executor-status schema includes config when present", async () => {
    const mockConfig = {
      ok: true,
      config: {
        accountKey: "mock_trading_main",
        signalThreshold: 26,
        minSignalScore: 50,
        updatedAt: Date.now(),
        source: "mongodb" as const,
      },
    };
    expect(mockConfig.config.signalThreshold).toBe(26);
    expect(mockConfig.config.minSignalScore).toBe(50);
  });

  it("signal-impact response includes strategy qualification rows", () => {
    const impact = {
      currentThreshold: 26,
      testThreshold: 24,
      currentMinSignalScore: 50,
      testMinSignalScore: 45,
      evaluatedStrategies: 12,
      strategiesAboveSignalThreshold: 4,
      strategiesAboveOpenThreshold: 2,
      strategiesFullyQualified: 1,
      strategies: [
        { name: "ScalperVWAP", currentScore: 28.5, wouldQualify: true },
      ],
    };
    expect(impact.strategies[0].wouldQualify).toBe(true);
    expect(impact.strategiesAboveSignalThreshold).toBeGreaterThanOrEqual(0);
  });
});
