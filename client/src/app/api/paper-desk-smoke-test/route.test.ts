import { afterEach, describe, expect, it, vi } from "vitest";
import { buildPaperDeskSmokeTrade, paperDeskSmokeTestEnabled } from "./route";
import { isProbeOrBootstrapTrade } from "@/lib/futuresSessionMetrics";

describe("paper desk smoke test helpers", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("is disabled by default", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_SMOKE_TEST", "");
    expect(paperDeskSmokeTestEnabled()).toBe(false);
  });

  it("requires explicit env enablement", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_SMOKE_TEST", "1");
    expect(paperDeskSmokeTestEnabled()).toBe(true);
  });

  it("creates a smoke/probe trade excluded from production metrics", () => {
    const trade = buildPaperDeskSmokeTrade("acct", "2026-01-01T00:00:00.000Z");
    expect(trade.strategy_name).toBe("PAPER_EXECUTION_SMOKE_TEST");
    expect(trade.net_pnl).toBe(0);
    expect(isProbeOrBootstrapTrade({ strategy_name: trade.strategy_name })).toBe(true);
  });
});
