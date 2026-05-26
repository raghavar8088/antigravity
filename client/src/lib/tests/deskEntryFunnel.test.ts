import { describe, expect, it } from "vitest";
import {
  computeDominantBlocker,
  buildFunnelSnapshot,
  funnelRecommendation,
  emptyBlockerCounts,
  funnelSnapshotFromBrowserDebug,
  type EntryFunnelBlockerCounts,
} from "../deskEntryFunnelSnapshot";
import { isProbeOrBootstrapTrade } from "../futuresSessionMetrics";

// ── emptyBlockerCounts ────────────────────────────────────────────────────────

describe("emptyBlockerCounts", () => {
  it("returns all zeros", () => {
    const c = emptyBlockerCounts();
    for (const v of Object.values(c)) expect(v).toBe(0);
  });
});

// ── computeDominantBlocker ────────────────────────────────────────────────────

describe("computeDominantBlocker", () => {
  it("returns 'none' when a trade opened", () => {
    const c = emptyBlockerCounts();
    c.signal = 10;
    expect(computeDominantBlocker(c, 1, 5)).toBe("none");
  });

  it("returns 'noData' before everything else when noData > 0", () => {
    const c = emptyBlockerCounts();
    c.noData = 1;
    c.signal = 99;
    expect(computeDominantBlocker(c, 0, 5)).toBe("noData");
  });

  it("returns 'noStrategies' when activeStrategies === 0", () => {
    const c = emptyBlockerCounts();
    expect(computeDominantBlocker(c, 0, 0)).toBe("noStrategies");
  });

  it("returns 'noStrategies' when noStrategies count > 0", () => {
    const c = emptyBlockerCounts();
    c.noStrategies = 1;
    expect(computeDominantBlocker(c, 0, 5)).toBe("noStrategies");
  });

  it("returns 'maxOpen' when only maxOpen is hit and no signal/confirm ran", () => {
    const c = emptyBlockerCounts();
    c.maxOpen = 3;
    expect(computeDominantBlocker(c, 0, 5)).toBe("maxOpen");
  });

  it("does NOT return maxOpen when signal also ran (pipeline wins)", () => {
    const c = emptyBlockerCounts();
    c.maxOpen = 3;
    c.signal = 5;
    expect(computeDominantBlocker(c, 0, 5)).toBe("signal");
  });

  it("returns 'session' when session is the only gate hit", () => {
    const c = emptyBlockerCounts();
    c.session = 4;
    expect(computeDominantBlocker(c, 0, 5)).toBe("session");
  });

  it("picks highest-count pipeline gate", () => {
    const c = emptyBlockerCounts();
    c.signal = 2;
    c.regime = 8;
    c.confirm = 3;
    expect(computeDominantBlocker(c, 0, 5)).toBe("regime");
  });

  it("returns 'none' when no blockers are set and nothing opened", () => {
    const c = emptyBlockerCounts();
    expect(computeDominantBlocker(c, 0, 5)).toBe("none");
  });
});

// ── funnelRecommendation ──────────────────────────────────────────────────────

describe("funnelRecommendation", () => {
  it("returns a non-empty string for every known key", () => {
    const keys = [
      "noData", "noStrategies", "signal", "confirm", "regime", "atrFees",
      "rotation", "suspended", "spread", "session", "category", "sameSide",
      "margin", "maxOpen", "cooldown", "none",
    ] as const;
    for (const key of keys) {
      const rec = funnelRecommendation(key);
      expect(typeof rec).toBe("string");
      expect(rec.length).toBeGreaterThan(0);
    }
  });

  it("never suggests lowering the threshold", () => {
    const keys = [
      "noData", "noStrategies", "signal", "confirm", "regime", "atrFees",
      "rotation", "suspended", "spread", "session", "category", "sameSide",
      "margin", "maxOpen", "cooldown", "none",
    ] as const;
    for (const key of keys) {
      const rec = funnelRecommendation(key).toLowerCase();
      expect(rec).not.toContain("lower threshold");
      expect(rec).not.toContain("lower the threshold");
      expect(rec).not.toContain("reduce threshold");
    }
  });
});

// ── buildFunnelSnapshot ───────────────────────────────────────────────────────

describe("buildFunnelSnapshot", () => {
  it("derives dominantBlocker and recommendation from counts", () => {
    const counts = emptyBlockerCounts();
    counts.signal = 7;
    const snap = buildFunnelSnapshot({
      tickAt: Date.now(),
      workerMode: "worker",
      workerFresh: true,
      symbol: "BTCUSD",
      markPrice: 60000,
      bars: 200,
      activeStrategies: 10,
      evaluatedStrategies: 10,
      signalPassed: 0,
      confirmPassed: 0,
      candidateCount: 0,
      openAttempts: 0,
      opened: 0,
      blockerCounts: counts,
    });
    expect(snap.dominantBlocker).toBe("signal");
    expect(snap.recommendation).toContain("signal");
  });

  it("sets dominantBlocker to 'none' when a trade opened", () => {
    const snap = buildFunnelSnapshot({
      tickAt: Date.now(),
      workerMode: "browser",
      workerFresh: false,
      symbol: "BTCUSD",
      markPrice: 60000,
      bars: 200,
      activeStrategies: 5,
      evaluatedStrategies: 5,
      signalPassed: 1,
      confirmPassed: 1,
      candidateCount: 1,
      openAttempts: 1,
      opened: 1,
      blockerCounts: { ...emptyBlockerCounts(), signal: 4 },
    });
    expect(snap.dominantBlocker).toBe("none");
  });
});

// ── funnelSnapshotFromBrowserDebug ────────────────────────────────────────────

describe("funnelSnapshotFromBrowserDebug", () => {
  const baseDebug = {
    pollAt: Date.now(),
    pauseEntries: false,
    drawdownLocked: false,
    intradayDdLocked: false,
    hasMarketData: true,
    activeStratCount: 10,
    evalPairs: 10,
    failSignal: 0,
    failConfirm: 0,
    failOpenRegime: 0,
    failMinMove: 0,
    failSpread: 0,
    failSession: 0,
    failCategoryCap: 0,
    failSameDirCap: 0,
    failMargin: 0,
    candidatesBuilt: 0,
    openAttempts: 0,
    openedThisPoll: 0,
    failDisabled: 0,
    failCooldown: 0,
    failOccupied: 0,
    dataHealthStatus: "ok",
    dominantBlocker: "NONE",
  };

  const opts = { workerFresh: false, symbol: "BTCUSD", markPrice: 60000, bars: 200 };

  it("maps noData when hasMarketData is false", () => {
    const snap = funnelSnapshotFromBrowserDebug({ ...baseDebug, hasMarketData: false }, opts);
    expect(snap.blockerCounts.noData).toBe(1);
    expect(snap.dominantBlocker).toBe("noData");
  });

  it("maps noStrategies when activeStratCount is 0", () => {
    const snap = funnelSnapshotFromBrowserDebug({ ...baseDebug, activeStratCount: 0 }, opts);
    expect(snap.blockerCounts.noStrategies).toBe(1);
    expect(snap.dominantBlocker).toBe("noStrategies");
  });

  it("maps failSignal to signal blocker", () => {
    const snap = funnelSnapshotFromBrowserDebug({ ...baseDebug, failSignal: 5 }, opts);
    expect(snap.blockerCounts.signal).toBe(5);
    expect(snap.dominantBlocker).toBe("signal");
  });

  it("maps pauseEntries to maxOpen blocker", () => {
    const snap = funnelSnapshotFromBrowserDebug({ ...baseDebug, pauseEntries: true }, opts);
    expect(snap.blockerCounts.maxOpen).toBeGreaterThan(0);
  });

  it("sets workerMode to 'browser'", () => {
    const snap = funnelSnapshotFromBrowserDebug(baseDebug, opts);
    expect(snap.workerMode).toBe("browser");
  });

  it("passes workerFresh through to snapshot", () => {
    const snap = funnelSnapshotFromBrowserDebug(baseDebug, { ...opts, workerFresh: true });
    expect(snap.workerFresh).toBe(true);
  });

  it("sets dominantBlocker to none when a trade opened", () => {
    const snap = funnelSnapshotFromBrowserDebug(
      { ...baseDebug, openedThisPoll: 1, failSignal: 99 },
      opts,
    );
    expect(snap.dominantBlocker).toBe("none");
  });
});

// ── isProbeOrBootstrapTrade — CANARY exclusion ────────────────────────────────

describe("isProbeOrBootstrapTrade — canary exclusion", () => {
  it("returns true for PAPER_ENTRY_CANARY (strategyName)", () => {
    expect(isProbeOrBootstrapTrade({ strategyName: "PAPER_ENTRY_CANARY" })).toBe(true);
  });

  it("returns true for PAPER_ENTRY_CANARY (strategy_name snake_case)", () => {
    expect(isProbeOrBootstrapTrade({ strategy_name: "PAPER_ENTRY_CANARY" })).toBe(true);
  });

  it("returns true for lowercase canary name", () => {
    expect(isProbeOrBootstrapTrade({ strategyName: "paper_entry_canary" })).toBe(true);
  });

  it("returns true for PAPER_BOOTSTRAP_PROBE (original behaviour preserved)", () => {
    expect(isProbeOrBootstrapTrade({ strategyName: "PAPER_BOOTSTRAP_PROBE" })).toBe(true);
  });

  it("returns true for DEV_FORCE_PROBE_OPEN (original behaviour preserved)", () => {
    expect(isProbeOrBootstrapTrade({ strategyName: "DEV_FORCE_PROBE_OPEN" })).toBe(true);
  });

  it("returns false for a real production strategy name", () => {
    expect(isProbeOrBootstrapTrade({ strategyName: "PRM_LONG_MOMENTUM_91" })).toBe(false);
  });

  it("returns false for empty string", () => {
    expect(isProbeOrBootstrapTrade({ strategyName: "" })).toBe(false);
  });

  it("returns false for null strategy name", () => {
    expect(isProbeOrBootstrapTrade({ strategyName: null })).toBe(false);
  });
});

// ── Canary default-off ────────────────────────────────────────────────────────

describe("canary — disabled by default", () => {
  it("NEXT_PUBLIC_DESK_ENTRY_CANARY is not set to '1' in test env", () => {
    // The canary must be OFF by default — operator must explicitly opt in.
    expect(process.env.NEXT_PUBLIC_DESK_ENTRY_CANARY).not.toBe("1");
  });
});

// ── Recommendations never suggest lowering threshold ─────────────────────────

describe("recommendations never suggest lowering threshold", () => {
  const allKeys = [
    "noData", "noStrategies", "signal", "confirm", "regime", "atrFees",
    "rotation", "suspended", "spread", "session", "category", "sameSide",
    "margin", "maxOpen", "cooldown", "none",
  ] as const;

  for (const key of allKeys) {
    it(`recommendation for '${key}' does not suggest lowering threshold`, () => {
      const rec = funnelRecommendation(key).toLowerCase();
      expect(rec).not.toContain("lower threshold");
      expect(rec).not.toContain("lower the threshold");
      expect(rec).not.toContain("reduce threshold");
      expect(rec).not.toContain("decrease threshold");
    });
  }
});
