import { describe, expect, it } from "vitest";
import { collectAppSnapshot } from "../aiAppTracker/collectAppSnapshot";
import {
  computeReportSeverity,
  buildReportSummary,
  buildRecommendations,
} from "../aiAppTracker/summarizeTrackerReport";
import { buildTrackerReport } from "../aiAppTracker/writeTrackerReport";
import {
  STALE_WORKER_THRESHOLD_MS,
  BALANCE_DRIFT_WARN_USD,
} from "../aiAppTracker/trackerConstants";
import type { AiTrackerSnapshot } from "../aiAppTracker/types";
import type { EntryFunnelSnapshot } from "../deskEntryFunnelSnapshot";

// ── Helpers ──────────────────────────────────────────────────────────────────

function makeSnapshot(overrides: Partial<AiTrackerSnapshot> = {}): AiTrackerSnapshot {
  return {
    capturedAt: new Date().toISOString(),
    buildSha: "abc1234",
    accountKeySuffix: "…test",
    workerLastPollAgeSeconds: 10,
    workerOwner: "vps",
    funnelTickAt: Date.now(),
    funnelTickAgeSeconds: 5,
    dominantBlocker: "none",
    recommendation: "All good.",
    activeStrategies: 10,
    evaluatedStrategies: 10,
    signalPassed: 0,
    opened: 0,
    balance: 1000,
    balanceDriftUsd: 0,
    openPositionsCount: 0,
    closedTradesCount: 10,
    lastTradeAt: new Date().toISOString(),
    rotationSummary: null,
    envFlags: {
      profitMode: false,
      winnersOnly: false,
      workerEnabled: true,
      researchMode: false,
      entryCanary: false,
    },
    warnings: [],
    ...overrides,
  };
}

function makeInput(overrides: Parameters<typeof collectAppSnapshot>[0] extends infer T ? Partial<T> : never = {}) {
  return {
    accountKey: "test-account-key-1234",
    buildSha: "abc1234",
    balance: 1000,
    dayStartBalance: 1000,
    workerLastPollAt: Date.now() - 5000,
    workerOwner: "vps" as const,
    openPositionsCount: 0,
    closedTradesCount: 10,
    lastTradeAt: Date.now() - 60_000,
    funnelSnapshot: null,
    rotationReport: null,
    probeDominant: false,
    ...overrides,
  };
}

// ── collectAppSnapshot — no secrets ─────────────────────────────────────────

describe("collectAppSnapshot — security", () => {
  it("never stores the full account key", () => {
    const snap = collectAppSnapshot(makeInput({ accountKey: "MY-SUPER-SECRET-ACCOUNT-KEY" }));
    expect(snap.accountKeySuffix).not.toContain("MY-SUPER-SECRET");
    expect(snap.accountKeySuffix).not.toBe("MY-SUPER-SECRET-ACCOUNT-KEY");
  });

  it("stores only the last 4 chars of the account key", () => {
    // key = "abcdefghijklmn" (14 chars), last 4 = "klmn"
    const snap = collectAppSnapshot(makeInput({ accountKey: "abcdefghijklmn" }));
    expect(snap.accountKeySuffix).toBe("…klmn");
  });

  it("handles short keys gracefully", () => {
    const snap = collectAppSnapshot(makeInput({ accountKey: "ab" }));
    expect(snap.accountKeySuffix).toBe("****");
  });

  it("snapshot contains no MONGODB_URI or full env secrets", () => {
    const snap = collectAppSnapshot(makeInput());
    const json = JSON.stringify(snap);
    expect(json).not.toContain("mongodb+srv://");
    expect(json).not.toContain("mongodb://");
    expect(json).not.toContain("NEXT_PUBLIC_");
  });
});

// ── collectAppSnapshot — stale worker warning ─────────────────────────────

describe("collectAppSnapshot — stale worker", () => {
  it("emits stale_worker warning when heartbeat is old", () => {
    const staleMs = STALE_WORKER_THRESHOLD_MS + 10_000;
    const snap = collectAppSnapshot(
      makeInput({ workerLastPollAt: Date.now() - staleMs }),
    );
    expect(snap.warnings.some((w) => w.code === "stale_worker")).toBe(true);
  });

  it("does NOT emit stale_worker when heartbeat is fresh", () => {
    const snap = collectAppSnapshot(makeInput({ workerLastPollAt: Date.now() - 5000 }));
    expect(snap.warnings.some((w) => w.code === "stale_worker")).toBe(false);
  });

  it("does NOT emit stale_worker when workerLastPollAt is null", () => {
    const snap = collectAppSnapshot(makeInput({ workerLastPollAt: null }));
    expect(snap.warnings.some((w) => w.code === "stale_worker")).toBe(false);
  });
});

// ── collectAppSnapshot — balance drift ───────────────────────────────────

describe("collectAppSnapshot — balance drift", () => {
  it("emits balance_drift when drift exceeds threshold", () => {
    const drift = BALANCE_DRIFT_WARN_USD + 10;
    const snap = collectAppSnapshot(
      makeInput({ balance: 1000 + drift, dayStartBalance: 1000 }),
    );
    expect(snap.warnings.some((w) => w.code === "balance_drift")).toBe(true);
  });

  it("emits balance_drift for negative drift too", () => {
    const drift = BALANCE_DRIFT_WARN_USD + 10;
    const snap = collectAppSnapshot(
      makeInput({ balance: 1000 - drift, dayStartBalance: 1000 }),
    );
    expect(snap.warnings.some((w) => w.code === "balance_drift")).toBe(true);
  });

  it("does NOT emit balance_drift when within threshold", () => {
    const snap = collectAppSnapshot(
      makeInput({ balance: 1010, dayStartBalance: 1000 }),
    );
    expect(snap.warnings.some((w) => w.code === "balance_drift")).toBe(false);
  });

  it("correctly computes balanceDriftUsd", () => {
    const snap = collectAppSnapshot(
      makeInput({ balance: 1075, dayStartBalance: 1000 }),
    );
    expect(snap.balanceDriftUsd).toBeCloseTo(75);
  });
});

// ── collectAppSnapshot — no_trades_30min ────────────────────────────────

describe("collectAppSnapshot — no_trades_30min", () => {
  it("emits no_trades_30min when last trade was >30min ago", () => {
    const snap = collectAppSnapshot(
      makeInput({ lastTradeAt: Date.now() - 35 * 60_000 }),
    );
    expect(snap.warnings.some((w) => w.code === "no_trades_30min")).toBe(true);
  });

  it("does NOT emit no_trades_30min when recent trade", () => {
    const snap = collectAppSnapshot(makeInput({ lastTradeAt: Date.now() - 5 * 60_000 }));
    expect(snap.warnings.some((w) => w.code === "no_trades_30min")).toBe(false);
  });
});

// ── collectAppSnapshot — probe_dominant ─────────────────────────────────

describe("collectAppSnapshot — probe_dominant", () => {
  it("emits probe_dominant when probeDominant=true", () => {
    const snap = collectAppSnapshot(makeInput({ probeDominant: true }));
    expect(snap.warnings.some((w) => w.code === "probe_dominant")).toBe(true);
  });

  it("does NOT emit probe_dominant when probeDominant=false", () => {
    const snap = collectAppSnapshot(makeInput({ probeDominant: false }));
    expect(snap.warnings.some((w) => w.code === "probe_dominant")).toBe(false);
  });
});

// ── collectAppSnapshot — funnel snapshot pass-through ───────────────────

describe("collectAppSnapshot — funnel snapshot", () => {
  it("uses dominantBlocker from funnel snapshot when present", () => {
    const funnel: EntryFunnelSnapshot = {
      tickAt: Date.now(),
      workerMode: "worker",
      workerFresh: true,
      symbol: "BTCUSD",
      markPrice: 60000,
      bars: 200,
      activeStrategies: 5,
      evaluatedStrategies: 5,
      signalPassed: 0,
      confirmPassed: 0,
      candidateCount: 0,
      openAttempts: 0,
      opened: 0,
      blockerCounts: {
        noData: 0, noStrategies: 0, signal: 7, confirm: 0,
        regime: 0, atrFees: 0, rotation: 0, suspended: 0,
        spread: 0, session: 0, category: 0, sameSide: 0,
        margin: 0, maxOpen: 0, cooldown: 0,
      },
      dominantBlocker: "signal",
      recommendation: "No strategy signal passed threshold.",
    };
    const snap = collectAppSnapshot(makeInput({ funnelSnapshot: funnel }));
    expect(snap.dominantBlocker).toBe("signal");
    expect(snap.activeStrategies).toBe(5);
  });

  it("falls back to 'unknown' when no funnel snapshot", () => {
    const snap = collectAppSnapshot(makeInput({ funnelSnapshot: null }));
    expect(snap.dominantBlocker).toBe("unknown");
  });
});

// ── computeReportSeverity ────────────────────────────────────────────────

describe("computeReportSeverity", () => {
  it("returns info when no warnings", () => {
    expect(computeReportSeverity(makeSnapshot())).toBe("info");
  });

  it("returns warning with a single warning", () => {
    const snap = makeSnapshot({
      warnings: [{ code: "stale_worker", message: "stale" }],
    });
    expect(computeReportSeverity(snap)).toBe("warning");
  });

  it("returns danger when stale_worker AND no_trades_30min together", () => {
    const snap = makeSnapshot({
      warnings: [
        { code: "stale_worker", message: "stale" },
        { code: "no_trades_30min", message: "no trades" },
      ],
    });
    expect(computeReportSeverity(snap)).toBe("danger");
  });

  it("returns danger when all strategies suspended", () => {
    const snap = makeSnapshot({
      warnings: [{ code: "no_strategy_candidates", message: "all suspended" }],
    });
    expect(computeReportSeverity(snap)).toBe("danger");
  });

  it("returns danger when balance drift > $100", () => {
    const snap = makeSnapshot({ balanceDriftUsd: -150, warnings: [] });
    expect(computeReportSeverity(snap)).toBe("danger");
  });
});

// ── buildRecommendations — no threshold-lowering suggestions ────────────

describe("buildRecommendations — no threshold lowering", () => {
  const warningCodes = [
    "stale_worker",
    "stale_deployment",
    "no_trades_30min",
    "balance_drift",
    "probe_dominant",
    "high_fee_gross",
    "no_strategy_candidates",
    "no_open_positions_long",
    "data_gap",
  ] as const;

  for (const code of warningCodes) {
    it(`recommendation for '${code}' does not suggest lowering threshold`, () => {
      const snap = makeSnapshot({
        warnings: [{ code, message: "test" }],
      });
      const recs = buildRecommendations(snap);
      const joined = recs.join(" ").toLowerCase();
      expect(joined).not.toContain("lower threshold");
      expect(joined).not.toContain("lower the threshold");
      expect(joined).not.toContain("reduce threshold");
      expect(joined).not.toContain("decrease threshold");
    });
  }
});

// ── buildTrackerReport — schema validation ───────────────────────────────

describe("buildTrackerReport — schema", () => {
  it("produces all required fields", () => {
    const snap = makeSnapshot();
    const report = buildTrackerReport(snap);
    expect(report.report_id).toBeTruthy();
    expect(report.created_at).toBeTruthy();
    expect(report.module).toBe("btc_future_trading");
    expect(["info", "warning", "danger"]).toContain(report.severity);
    expect(typeof report.summary).toBe("string");
    expect(report.summary.length).toBeGreaterThan(0);
    expect(Array.isArray(report.recommendations)).toBe(true);
    expect(report.snapshot).toBe(snap);
  });

  it("report_id is a valid UUID", () => {
    const report = buildTrackerReport(makeSnapshot());
    expect(report.report_id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
    );
  });

  it("app_build_sha matches snapshot buildSha", () => {
    const snap = makeSnapshot({ buildSha: "deadbeef" } as Partial<AiTrackerSnapshot>);
    const report = buildTrackerReport(snap);
    expect(report.app_build_sha).toBe("deadbeef");
  });

  it("account_key_suffix never contains full key", () => {
    const snap = makeSnapshot({ accountKeySuffix: "…abcd" });
    const report = buildTrackerReport(snap);
    expect(report.account_key_suffix).toBe("…abcd");
    const json = JSON.stringify(report);
    expect(json).not.toContain("SECRET");
  });
});

// ── Mind map JSON — valid schema ─────────────────────────────────────────

describe("ai-application-mindmap.json — schema", () => {
  it("has required top-level fields", async () => {
    const { readFileSync } = await import("fs");
    const { join, resolve } = await import("path");
    const path = resolve(__dirname, "../../../docs/ai-application-mindmap.json");
    const raw = readFileSync(path, "utf-8");
    const doc = JSON.parse(raw) as Record<string, unknown>;
    expect(doc.version).toBeTruthy();
    expect(doc.updatedAt).toBeTruthy();
    expect(doc.appName).toBeTruthy();
    expect(Array.isArray(doc.modules)).toBe(true);
    expect(Array.isArray(doc.dataStores)).toBe(true);
    expect(Array.isArray(doc.criticalFlows)).toBe(true);
    expect(Array.isArray(doc.constraints)).toBe(true);
    expect(Array.isArray(doc.debugChecklist)).toBe(true);
  });

  it("every module has required fields", async () => {
    const { readFileSync } = await import("fs");
    const { resolve } = await import("path");
    const path = resolve(__dirname, "../../../docs/ai-application-mindmap.json");
    const doc = JSON.parse(readFileSync(path, "utf-8")) as {
      modules: Array<Record<string, unknown>>;
    };
    for (const mod of doc.modules) {
      expect(typeof mod.name).toBe("string");
      expect(typeof mod.purpose).toBe("string");
      expect(Array.isArray(mod.keyFiles)).toBe(true);
      expect(["browser", "server", "worker", "shared"]).toContain(mod.runtime);
      expect(Array.isArray(mod.risks)).toBe(true);
    }
  });

  it("includes hard constraints array with at least 8 entries", async () => {
    const { readFileSync } = await import("fs");
    const { resolve } = await import("path");
    const path = resolve(__dirname, "../../../docs/ai-application-mindmap.json");
    const doc = JSON.parse(readFileSync(path, "utf-8")) as { constraints: unknown[] };
    expect(doc.constraints.length).toBeGreaterThanOrEqual(8);
  });

  it("constraints do not mention lowering the threshold", async () => {
    const { readFileSync } = await import("fs");
    const { resolve } = await import("path");
    const path = resolve(__dirname, "../../../docs/ai-application-mindmap.json");
    const doc = JSON.parse(readFileSync(path, "utf-8")) as { constraints: string[] };
    const joined = doc.constraints.join(" ").toLowerCase();
    expect(joined).not.toContain("lower threshold");
    expect(joined).not.toContain("reduce threshold");
  });
});

// ── buildReportSummary — content ─────────────────────────────────────────

describe("buildReportSummary", () => {
  it("info summary mentions 'healthy'", () => {
    const snap = makeSnapshot();
    const summary = buildReportSummary(snap, "info");
    expect(summary.toLowerCase()).toContain("healthy");
  });

  it("danger summary mentions 'DANGER'", () => {
    const snap = makeSnapshot({
      warnings: [
        { code: "stale_worker", message: "stale" },
        { code: "no_trades_30min", message: "no trades" },
      ],
    });
    const summary = buildReportSummary(snap, "danger");
    expect(summary.toUpperCase()).toContain("DANGER");
  });

  it("summary is always a non-empty string", () => {
    for (const sev of ["info", "warning", "danger"] as const) {
      const summary = buildReportSummary(makeSnapshot(), sev);
      expect(typeof summary).toBe("string");
      expect(summary.length).toBeGreaterThan(0);
    }
  });
});
