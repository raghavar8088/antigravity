/**
 * Tests for summarizeTrackerSnapshot — pure function, no MongoDB.
 * Covers: severity rules, recommendation content, threshold-lowering guard.
 */

import { describe, it, expect } from "vitest";
import { summarizeTrackerSnapshot } from "../aiAppTracker/summarizeTrackerReport";
import { buildTrackerReport } from "../aiAppTracker/writeTrackerReport";
import type { AiAppTrackerSnapshot } from "../aiAppTracker/types";

// ── helpers ───────────────────────────────────────────────────────────────────

function makeSnap(overrides: Partial<AiAppTrackerSnapshot> = {}): AiAppTrackerSnapshot {
  const base: AiAppTrackerSnapshot = {
    createdAt: new Date().toISOString(),
    appBuildSha: "abc1234",
    module: "btc_future_trading",
    accountKeySuffix: "…test",
    worker: { enabled: true, stale: false, lastPollAt: Date.now() - 5_000, ageSeconds: 5, owner: "vps" },
    entryFunnel: {
      ageSeconds: 3,
      dominantBlocker: "none",
      recommendation: "All good.",
      activeStrategies: 10,
      evaluatedStrategies: 10,
      candidates: 0,
      opened: 0,
    },
    paperState: {
      balance: 1000,
      openPositions: 0,
      workerLastPollAt: Date.now() - 5_000,
      balanceDriftUsd: 0,
      pauseEntries: false,
      clearedAt: null,
    },
    env: { profitMode: false, winnersOnly: false, researchMode: false, workerEnabled: true },
    warnings: [],
  };
  return { ...base, ...overrides };
}

// ── Severity rules ────────────────────────────────────────────────────────────

describe("summarizeTrackerSnapshot — severity", () => {
  it("returns info when no warnings and worker is fresh", () => {
    const { severity } = summarizeTrackerSnapshot(makeSnap());
    expect(severity).toBe("info");
  });

  it("returns warning for a single generic warning", () => {
    const snap = makeSnap({ warnings: ["Something minor is off."] });
    const { severity } = summarizeTrackerSnapshot(snap);
    expect(severity).toBe("warning");
  });

  it("returns danger when worker is stale and no open positions", () => {
    const snap = makeSnap({
      worker: { enabled: true, stale: true, lastPollAt: Date.now() - 200_000, ageSeconds: 200, owner: null },
      paperState: {
        balance: 1000, openPositions: 0,
        workerLastPollAt: Date.now() - 200_000,
        balanceDriftUsd: 0, pauseEntries: false, clearedAt: null,
      },
      warnings: ["Worker heartbeat is stale (200s old)."],
    });
    const { severity } = summarizeTrackerSnapshot(snap);
    expect(severity).toBe("danger");
  });

  it("returns danger when balance drift > $100", () => {
    const snap = makeSnap({
      paperState: {
        balance: 850, openPositions: 0,
        workerLastPollAt: Date.now() - 5_000,
        balanceDriftUsd: -150, pauseEntries: false, clearedAt: null,
      },
      warnings: ["Balance drifted -$150.00 from day start."],
    });
    const { severity } = summarizeTrackerSnapshot(snap);
    expect(severity).toBe("danger");
  });

  it("returns danger when no account key suffix", () => {
    const snap = makeSnap({
      accountKeySuffix: null,
      warnings: ["DESK_WORKER_ACCOUNT_KEY is not set."],
    });
    const { severity } = summarizeTrackerSnapshot(snap);
    expect(severity).toBe("danger");
  });
});

// ── Summary text ──────────────────────────────────────────────────────────────

describe("summarizeTrackerSnapshot — summary text", () => {
  it("info summary mentions 'healthy'", () => {
    const { summary } = summarizeTrackerSnapshot(makeSnap());
    expect(summary.toLowerCase()).toContain("healthy");
  });

  it("danger summary contains DANGER", () => {
    const snap = makeSnap({
      worker: { enabled: true, stale: true, lastPollAt: null, ageSeconds: null, owner: null },
      paperState: { balance: 1000, openPositions: 0, workerLastPollAt: null, balanceDriftUsd: 0, pauseEntries: false, clearedAt: null },
      warnings: ["Worker heartbeat is stale."],
    });
    const { summary } = summarizeTrackerSnapshot(snap);
    expect(summary.toUpperCase()).toContain("DANGER");
  });

  it("summary is always a non-empty string", () => {
    for (const overrides of [{}, { warnings: ["x"] }, { accountKeySuffix: null as null }]) {
      const { summary } = summarizeTrackerSnapshot(makeSnap(overrides));
      expect(typeof summary).toBe("string");
      expect(summary.length).toBeGreaterThan(0);
    }
  });
});

// ── Recommendations ───────────────────────────────────────────────────────────

describe("summarizeTrackerSnapshot — recommendations", () => {
  it("recommends restarting pm2 worker for stale worker warning", () => {
    const snap = makeSnap({
      worker: { enabled: true, stale: true, lastPollAt: null, ageSeconds: 200, owner: null },
      paperState: { balance: 1000, openPositions: 0, workerLastPollAt: null, balanceDriftUsd: 0, pauseEntries: false, clearedAt: null },
      warnings: ["Worker heartbeat is stale (200s old). Restart: pm2 restart btc-ft-paper-worker on VPS."],
    });
    const { recommendations } = summarizeTrackerSnapshot(snap);
    const joined = recommendations.join(" ").toLowerCase();
    expect(joined).toContain("pm2");
  });

  it("recommends computeSessionEquityFromProduction for balance drift", () => {
    const snap = makeSnap({
      warnings: ["Balance drifted +$75.00 from day start. Run computeSessionEquityFromProduction() to verify."],
    });
    const { recommendations } = summarizeTrackerSnapshot(snap);
    const joined = recommendations.join(" ").toLowerCase();
    expect(joined).toContain("computesessionequityfromprod");
  });

  it("info state: clean recommendation to monitor desk", () => {
    const { recommendations } = summarizeTrackerSnapshot(makeSnap());
    expect(recommendations.some((r) => r.toLowerCase().includes("healthy"))).toBe(true);
  });

  it("always appends ai:summary pointer", () => {
    for (const snap of [makeSnap(), makeSnap({ warnings: ["Worker heartbeat is stale (200s)."] })]) {
      const { recommendations } = summarizeTrackerSnapshot(snap);
      expect(recommendations.some((r) => r.includes("ai:summary"))).toBe(true);
    }
  });
});

// ── Hard invariant: never recommend lowering the signal threshold ─────────────

describe("summarizeTrackerSnapshot — no threshold-lowering", () => {
  const warningSamples = [
    "Worker heartbeat is stale (200s old). Restart: pm2 restart btc-ft-paper-worker on VPS.",
    "Balance drifted -$75.00 from day start. Run computeSessionEquityFromProduction() to verify.",
    "Entry funnel snapshot is stale.",
    "Entries are paused (pauseEntries=true in paper_state).",
    "No entry candidates or trades opened last tick. Dominant blocker: signal.",
    "DESK_WORKER_ACCOUNT_KEY is not set — cannot read desk state.",
    "No paper_state found in MongoDB for this account.",
    "MongoDB is not configured (MONGODB_URI missing or invalid).",
  ];

  for (const warning of warningSamples) {
    it(`warning "${warning.slice(0, 50)}…" does not suggest lowering threshold`, () => {
      const snap = makeSnap({ warnings: [warning] });
      const { recommendations } = summarizeTrackerSnapshot(snap);
      const joined = recommendations.join(" ").toLowerCase();
      expect(joined).not.toContain("lower threshold");
      expect(joined).not.toContain("lower the threshold");
      expect(joined).not.toContain("reduce threshold");
      expect(joined).not.toContain("decrease threshold");
      expect(joined).not.toContain("signal threshold to");
    });
  }
});

// ── buildTrackerReport — schema ───────────────────────────────────────────────

describe("buildTrackerReport — schema", () => {
  it("produces all required fields", () => {
    const snap = makeSnap();
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

  it("report_id is a valid UUID v4", () => {
    const report = buildTrackerReport(makeSnap());
    expect(report.report_id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
    );
  });

  it("account_key_suffix never contains the full key string", () => {
    const snap = makeSnap({ accountKeySuffix: "…abcd" });
    const report = buildTrackerReport(snap);
    expect(report.account_key_suffix).toBe("…abcd");
    expect(JSON.stringify(report)).not.toContain("SECRET");
  });
});
