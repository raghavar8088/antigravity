/**
 * Tests for aiAppTracker/aiAppTrackerMongo helpers.
 * MongoDB is mocked — no real Atlas connection required.
 */

import { describe, it, expect, vi, beforeEach } from "vitest";
import type { AiAppTrackerReport, AiAppTrackerSnapshot } from "../aiAppTracker/types";

// ── Mock MongoDB getDb ────────────────────────────────────────────────────────

// In-memory store shared across the mock collection calls
let store: AiAppTrackerReport[] = [];

const mockCollection = {
  createIndex: vi.fn().mockResolvedValue(undefined),
  insertOne: vi.fn().mockImplementation(async (doc: AiAppTrackerReport) => {
    // Simulate unique report_id constraint
    if (store.find((r) => r.report_id === doc.report_id)) {
      throw Object.assign(new Error("E11000 duplicate key"), { code: 11000 });
    }
    store.push({ ...doc });
    return { insertedId: doc.report_id };
  }),
  find: vi.fn().mockImplementation((_filter: Record<string, unknown>) => ({
    sort: (_s: unknown) => ({
      limit: (n: number) => ({
        toArray: async () =>
          [...store]
            .sort((a, b) => b.created_at.localeCompare(a.created_at))
            .slice(0, n)
            .map(({ ...rest }) => rest),
        next: async () => {
          const sorted = [...store].sort((a, b) => b.created_at.localeCompare(a.created_at));
          return sorted[0] ?? null;
        },
      }),
    }),
  })),
};

vi.mock("../mongoTradesClient", () => ({
  getDb: vi.fn().mockResolvedValue({
    collection: vi.fn().mockReturnValue(mockCollection),
  }),
}));

// Import AFTER mocks are set up
const { insertAiTrackerReport, listAiTrackerReports, getLatestAiTrackerReport } =
  await import("../aiAppTracker/aiAppTrackerMongo");

// ── helpers ───────────────────────────────────────────────────────────────────

function makeReport(overrides: Partial<AiAppTrackerReport> = {}): AiAppTrackerReport {
  const snap: AiAppTrackerSnapshot = {
    createdAt: new Date().toISOString(),
    appBuildSha: "abc1234",
    module: "btc_future_trading",
    accountKeySuffix: "…test",
    worker: { enabled: true, stale: false, lastPollAt: Date.now(), ageSeconds: 5, owner: "vps" },
    entryFunnel: {
      ageSeconds: 3, dominantBlocker: "none", recommendation: "All good.",
      activeStrategies: 10, evaluatedStrategies: 10, candidates: 0, opened: 0,
    },
    paperState: {
      balance: 1000, openPositions: 0, workerLastPollAt: Date.now(),
      balanceDriftUsd: 0, pauseEntries: false, clearedAt: null,
    },
    env: { profitMode: false, winnersOnly: false, researchMode: false, workerEnabled: true },
    warnings: [],
  };
  return {
    report_id: `test-${Math.random().toString(36).slice(2)}`,
    created_at: new Date().toISOString(),
    app_build_sha: "abc1234",
    account_key_suffix: "…test",
    module: "btc_future_trading",
    severity: "info",
    summary: "Desk healthy.",
    snapshot: snap,
    recommendations: ["Monitor the Entry Funnel Card."],
    ...overrides,
  };
}

beforeEach(() => {
  store = [];
  vi.clearAllMocks();
  // Re-apply mock implementations after clearAllMocks
  mockCollection.createIndex.mockResolvedValue(undefined);
  mockCollection.insertOne.mockImplementation(async (doc: AiAppTrackerReport) => {
    if (store.find((r) => r.report_id === doc.report_id)) {
      throw Object.assign(new Error("E11000 duplicate key"), { code: 11000 });
    }
    store.push({ ...doc });
    return { insertedId: doc.report_id };
  });
  mockCollection.find.mockImplementation((_filter: Record<string, unknown>) => ({
    sort: (_s: unknown) => ({
      limit: (n: number) => ({
        toArray: async () =>
          [...store]
            .sort((a, b) => b.created_at.localeCompare(a.created_at))
            .slice(0, n)
            .map(({ ...rest }) => rest),
        next: async () => {
          const sorted = [...store].sort((a, b) => b.created_at.localeCompare(a.created_at));
          return sorted[0] ?? null;
        },
      }),
    }),
  }));
});

// ── insertAiTrackerReport ─────────────────────────────────────────────────────

describe("insertAiTrackerReport", () => {
  it("inserts a report without throwing", async () => {
    const report = makeReport();
    await expect(insertAiTrackerReport(report)).resolves.toBeUndefined();
    expect(store).toHaveLength(1);
    expect(store[0].report_id).toBe(report.report_id);
  });

  it("is non-fatal when MongoDB throws — does not propagate error", async () => {
    mockCollection.insertOne.mockRejectedValueOnce(new Error("Mongo down"));
    await expect(insertAiTrackerReport(makeReport())).resolves.toBeUndefined();
  });

  it("never stores the full account key in the collection", async () => {
    const report = makeReport({ account_key_suffix: "…abcd" });
    await insertAiTrackerReport(report);
    expect(JSON.stringify(store[0])).not.toContain("SECRET");
    expect(store[0].account_key_suffix).toBe("…abcd");
  });
});

// ── listAiTrackerReports ──────────────────────────────────────────────────────

describe("listAiTrackerReports", () => {
  it("returns reports newest-first", async () => {
    const older = makeReport({ created_at: "2026-01-01T00:00:00.000Z" });
    const newer = makeReport({ created_at: "2026-06-01T00:00:00.000Z" });
    store.push(older, newer);

    const reports = await listAiTrackerReports({ limit: 10, module: "btc_future_trading" });
    expect(reports[0].created_at).toBe("2026-06-01T00:00:00.000Z");
    expect(reports[1].created_at).toBe("2026-01-01T00:00:00.000Z");
  });

  it("respects limit parameter", async () => {
    for (let i = 0; i < 5; i++) store.push(makeReport());
    const reports = await listAiTrackerReports({ limit: 3 });
    expect(reports.length).toBeLessThanOrEqual(3);
  });

  it("returns empty array on Mongo error — non-fatal", async () => {
    mockCollection.find.mockImplementationOnce(() => { throw new Error("Mongo down"); });
    await expect(listAiTrackerReports({ limit: 10 })).resolves.toEqual([]);
  });
});

// ── getLatestAiTrackerReport ──────────────────────────────────────────────────

describe("getLatestAiTrackerReport", () => {
  it("returns null when no reports exist", async () => {
    const result = await getLatestAiTrackerReport();
    expect(result).toBeNull();
  });

  it("returns the most recently created report", async () => {
    const older = makeReport({ created_at: "2026-01-01T00:00:00.000Z" });
    const newer = makeReport({ created_at: "2026-06-01T00:00:00.000Z" });
    store.push(older, newer);

    const latest = await getLatestAiTrackerReport();
    expect(latest?.created_at).toBe("2026-06-01T00:00:00.000Z");
  });

  it("returns null on Mongo error — non-fatal", async () => {
    mockCollection.find.mockImplementationOnce(() => { throw new Error("Mongo down"); });
    await expect(getLatestAiTrackerReport()).resolves.toBeNull();
  });
});
