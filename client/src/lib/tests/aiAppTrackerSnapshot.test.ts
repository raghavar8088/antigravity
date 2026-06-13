/**
 * Tests for buildSnapshotFromRaw — pure function, no MongoDB.
 * Covers: security, stale worker, missing account key, balance drift, funnel fields.
 */

import { describe, it, expect } from "vitest";
import { buildSnapshotFromRaw, type SnapshotRawInput } from "../aiAppTracker/collectAppSnapshot";
import {
  STALE_WORKER_THRESHOLD_MS,
  BALANCE_DRIFT_WARN_USD,
  STALE_FUNNEL_THRESHOLD_MS,
} from "../aiAppTracker/trackerConstants";
import type { EntryFunnelSnapshot } from "../trading/deskEntryFunnelSnapshot";

// ── helpers ───────────────────────────────────────────────────────────────────

function baseInput(overrides: Partial<SnapshotRawInput> = {}): SnapshotRawInput {
  return {
    accountKey: "test-account-key-1234",
    nowMs: Date.now(),
    buildSha: "abc1234",
    workerEnabled: true,
    workerLastPollAt: Date.now() - 5_000,
    workerOwner: "vps",
    funnelSnapshot: null,
    balance: 1000,
    dayStartBalance: 1000,
    openPositionsCount: 0,
    pauseEntries: false,
    clearedAt: null,
    hasPaperState: true,
    ...overrides,
  };
}

// ── Security — account key suffix only ───────────────────────────────────────

describe("buildSnapshotFromRaw — security", () => {
  it("stores only last 4 chars of account key", () => {
    const snap = buildSnapshotFromRaw(baseInput({ accountKey: "abcdefghijklmn" }));
    expect(snap.accountKeySuffix).toBe("…klmn");
  });

  it("never stores the full account key", () => {
    const snap = buildSnapshotFromRaw(baseInput({ accountKey: "MY-SUPER-SECRET-KEY" }));
    const json = JSON.stringify(snap);
    expect(json).not.toContain("MY-SUPER-SECRET");
    expect(snap.accountKeySuffix).not.toBe("MY-SUPER-SECRET-KEY");
  });

  it("returns **** suffix for short keys", () => {
    const snap = buildSnapshotFromRaw(baseInput({ accountKey: "ab" }));
    expect(snap.accountKeySuffix).toBe("****");
  });

  it("snapshot JSON contains no MONGODB_URI or connection string fragments", () => {
    const snap = buildSnapshotFromRaw(baseInput());
    const json = JSON.stringify(snap);
    expect(json).not.toContain("mongodb+srv://");
    expect(json).not.toContain("mongodb://");
    expect(json).not.toContain("NEXT_PUBLIC_");
  });
});

// ── Missing account key ───────────────────────────────────────────────────────

describe("buildSnapshotFromRaw — missing account key", () => {
  it("emits warning when accountKey is null", () => {
    const snap = buildSnapshotFromRaw(baseInput({ accountKey: null }));
    expect(snap.warnings.some((w) => w.includes("DESK_WORKER_ACCOUNT_KEY"))).toBe(true);
  });

  it("sets accountKeySuffix to null when accountKey is null", () => {
    const snap = buildSnapshotFromRaw(baseInput({ accountKey: null }));
    expect(snap.accountKeySuffix).toBeNull();
  });

  it("emits 'no paper_state' warning when hasPaperState is false with a key", () => {
    const snap = buildSnapshotFromRaw(baseInput({ hasPaperState: false }));
    expect(snap.warnings.some((w) => w.includes("No paper_state"))).toBe(true);
  });
});

// ── Stale worker warning ──────────────────────────────────────────────────────

describe("buildSnapshotFromRaw — stale worker", () => {
  it("sets worker.stale=true and emits warning when heartbeat is old", () => {
    const now = Date.now();
    const snap = buildSnapshotFromRaw(
      baseInput({
        nowMs: now,
        workerLastPollAt: now - STALE_WORKER_THRESHOLD_MS - 10_000,
      }),
    );
    expect(snap.worker.stale).toBe(true);
    expect(snap.warnings.some((w) => w.includes("Worker heartbeat is stale"))).toBe(true);
  });

  it("sets worker.stale=false when heartbeat is fresh", () => {
    const now = Date.now();
    const snap = buildSnapshotFromRaw(
      baseInput({ nowMs: now, workerLastPollAt: now - 5_000 }),
    );
    expect(snap.worker.stale).toBe(false);
    expect(snap.warnings.some((w) => w.includes("Worker heartbeat is stale"))).toBe(false);
  });

  it("does not emit stale warning when workerLastPollAt is null", () => {
    const snap = buildSnapshotFromRaw(baseInput({ workerLastPollAt: null }));
    expect(snap.worker.stale).toBe(false);
    expect(snap.warnings.some((w) => w.includes("stale"))).toBe(false);
  });

  it("populates worker.ageSeconds correctly", () => {
    const now = Date.now();
    const snap = buildSnapshotFromRaw(
      baseInput({ nowMs: now, workerLastPollAt: now - 30_000 }),
    );
    expect(snap.worker.ageSeconds).toBe(30);
  });
});

// ── Balance drift ─────────────────────────────────────────────────────────────

describe("buildSnapshotFromRaw — balance drift", () => {
  it("emits warning when positive drift exceeds threshold", () => {
    const snap = buildSnapshotFromRaw(
      baseInput({ balance: 1000 + BALANCE_DRIFT_WARN_USD + 10, dayStartBalance: 1000 }),
    );
    expect(snap.warnings.some((w) => w.includes("Balance drifted"))).toBe(true);
    expect(snap.paperState.balanceDriftUsd).toBeGreaterThan(0);
  });

  it("emits warning when negative drift exceeds threshold", () => {
    const snap = buildSnapshotFromRaw(
      baseInput({ balance: 1000 - BALANCE_DRIFT_WARN_USD - 10, dayStartBalance: 1000 }),
    );
    expect(snap.warnings.some((w) => w.includes("Balance drifted"))).toBe(true);
  });

  it("does NOT emit drift warning within threshold", () => {
    const snap = buildSnapshotFromRaw(
      baseInput({ balance: 1010, dayStartBalance: 1000 }),
    );
    expect(snap.warnings.some((w) => w.includes("Balance drifted"))).toBe(false);
  });

  it("computes balanceDriftUsd correctly", () => {
    const snap = buildSnapshotFromRaw(
      baseInput({ balance: 1075, dayStartBalance: 1000 }),
    );
    expect(snap.paperState.balanceDriftUsd).toBeCloseTo(75);
  });
});

// ── Pause entries ─────────────────────────────────────────────────────────────

describe("buildSnapshotFromRaw — pause entries", () => {
  it("emits warning when pauseEntries=true", () => {
    const snap = buildSnapshotFromRaw(baseInput({ pauseEntries: true }));
    expect(snap.warnings.some((w) => w.includes("Entries are paused"))).toBe(true);
  });

  it("does NOT emit pause warning when pauseEntries=false", () => {
    const snap = buildSnapshotFromRaw(baseInput({ pauseEntries: false }));
    expect(snap.warnings.some((w) => w.includes("Entries are paused"))).toBe(false);
  });

  it("sets paperState.pauseEntries=true in snapshot", () => {
    const snap = buildSnapshotFromRaw(baseInput({ pauseEntries: true }));
    expect(snap.paperState.pauseEntries).toBe(true);
  });
});

// ── Funnel snapshot fields ────────────────────────────────────────────────────

describe("buildSnapshotFromRaw — funnel snapshot", () => {
  it("populates entryFunnel fields from funnel snapshot", () => {
    const funnel: EntryFunnelSnapshot = {
      tickAt: Date.now() - 2_000,
      workerMode: "worker",
      workerFresh: true,
      symbol: "BTCUSD",
      markPrice: 60000,
      bars: 200,
      activeStrategies: 8,
      evaluatedStrategies: 8,
      signalPassed: 0,
      confirmPassed: 0,
      candidateCount: 0,
      openAttempts: 0,
      opened: 0,
      blockerCounts: {
        noData: 0, noStrategies: 0, signal: 8, confirm: 0,
        regime: 0, atrFees: 0, rotation: 0, suspended: 0,
        spread: 0, session: 0, category: 0, sameSide: 0,
        margin: 0, maxOpen: 0, cooldown: 0,
      },
      dominantBlocker: "signal",
      recommendation: "No strategy signal passed threshold.",
    };
    const snap = buildSnapshotFromRaw(baseInput({ funnelSnapshot: funnel }));
    expect(snap.entryFunnel.dominantBlocker).toBe("signal");
    expect(snap.entryFunnel.activeStrategies).toBe(8);
    expect(snap.entryFunnel.candidates).toBe(0);
  });

  it("populates null entryFunnel fields when no funnel snapshot", () => {
    const snap = buildSnapshotFromRaw(baseInput({ funnelSnapshot: null }));
    expect(snap.entryFunnel.dominantBlocker).toBeNull();
    expect(snap.entryFunnel.activeStrategies).toBeNull();
  });

  it("emits stale funnel warning when snapshot is old", () => {
    const now = Date.now();
    const funnel: EntryFunnelSnapshot = {
      tickAt: now - STALE_FUNNEL_THRESHOLD_MS - 10_000,
      workerMode: "worker",
      workerFresh: false,
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
        noData: 0, noStrategies: 0, signal: 5, confirm: 0,
        regime: 0, atrFees: 0, rotation: 0, suspended: 0,
        spread: 0, session: 0, category: 0, sameSide: 0,
        margin: 0, maxOpen: 0, cooldown: 0,
      },
      dominantBlocker: "signal",
      recommendation: "",
    };
    const snap = buildSnapshotFromRaw(baseInput({ nowMs: now, funnelSnapshot: funnel }));
    expect(snap.warnings.some((w) => w.includes("funnel snapshot is stale"))).toBe(true);
  });
});

// ── Module field ──────────────────────────────────────────────────────────────

describe("buildSnapshotFromRaw — schema", () => {
  it("always sets module to btc_future_trading", () => {
    const snap = buildSnapshotFromRaw(baseInput());
    expect(snap.module).toBe("btc_future_trading");
  });

  it("createdAt is a valid ISO string", () => {
    const snap = buildSnapshotFromRaw(baseInput());
    expect(() => new Date(snap.createdAt)).not.toThrow();
    expect(new Date(snap.createdAt).getFullYear()).toBeGreaterThan(2020);
  });
});
