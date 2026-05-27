import { describe, expect, it } from "vitest";
import {
  isWorkerHeartbeatFresh,
  workerHeartbeatAgeSeconds,
  WORKER_LEASE_TTL_MS,
} from "../paperDeskWorker/workerLease";
import { normalizeBaseUrl } from "../paperDeskWorker/workerHelpers";
import {
  resolveWorkerStrategyRoster,
  workerPaperAllowsRelaxedConfirm,
  workerPaperEffectiveSignalThreshold,
} from "../paperDeskWorker/runPaperDeskPollTick";
import { FUTURES_STRAT_DEFS } from "../futuresStrategies";

describe("isWorkerHeartbeatFresh", () => {
  it("returns false for null", () => {
    expect(isWorkerHeartbeatFresh(null)).toBe(false);
  });

  it("returns false for undefined", () => {
    expect(isWorkerHeartbeatFresh(undefined)).toBe(false);
  });

  it("returns true for a recent timestamp (1 second ago)", () => {
    const oneSecAgo = Date.now() - 1_000;
    expect(isWorkerHeartbeatFresh(oneSecAgo)).toBe(true);
  });

  it("returns true just inside the TTL boundary", () => {
    const justInside = Date.now() - (WORKER_LEASE_TTL_MS - 1_000);
    expect(isWorkerHeartbeatFresh(justInside)).toBe(true);
  });

  it("returns false just outside the TTL boundary", () => {
    const justOutside = Date.now() - (WORKER_LEASE_TTL_MS + 1_000);
    expect(isWorkerHeartbeatFresh(justOutside)).toBe(false);
  });

  it("returns false for a timestamp 10 minutes old", () => {
    const tenMinAgo = Date.now() - 10 * 60 * 1_000;
    expect(isWorkerHeartbeatFresh(tenMinAgo)).toBe(false);
  });

  it("respects a custom ttlMs override", () => {
    const fiveSecAgo = Date.now() - 5_000;
    expect(isWorkerHeartbeatFresh(fiveSecAgo, 10_000)).toBe(true);
    expect(isWorkerHeartbeatFresh(fiveSecAgo, 3_000)).toBe(false);
  });
});

describe("workerHeartbeatAgeSeconds", () => {
  it("returns null for null", () => {
    expect(workerHeartbeatAgeSeconds(null)).toBeNull();
  });

  it("returns null for undefined", () => {
    expect(workerHeartbeatAgeSeconds(undefined)).toBeNull();
  });

  it("returns approximate age in whole seconds", () => {
    const thirtySecAgo = Date.now() - 30_000;
    const age = workerHeartbeatAgeSeconds(thirtySecAgo);
    expect(age).toBeGreaterThanOrEqual(29);
    expect(age).toBeLessThanOrEqual(31);
  });

  it("returns 0 for a timestamp less than 1 second old", () => {
    const justNow = Date.now() - 200;
    expect(workerHeartbeatAgeSeconds(justNow)).toBe(0);
  });

  it("floors fractional seconds (does not round up)", () => {
    const ninePointNineSecAgo = Date.now() - 9_900;
    expect(workerHeartbeatAgeSeconds(ninePointNineSecAgo)).toBe(9);
  });
});

describe("cron skip guard contract (isWorkerHeartbeatFresh)", () => {
  it("cron should skip when worker polled within the last 30 seconds", () => {
    const recentPoll = Date.now() - 30_000;
    // Simulate the cron route decision: skip if fresh
    const shouldSkip = isWorkerHeartbeatFresh(recentPoll);
    expect(shouldSkip).toBe(true);
  });

  it("cron should run when worker_last_poll_at is null (worker never started)", () => {
    const shouldSkip = isWorkerHeartbeatFresh(null);
    expect(shouldSkip).toBe(false);
  });

  it("cron should run when worker polled more than TTL seconds ago", () => {
    const stalePoll = Date.now() - (WORKER_LEASE_TTL_MS + 10_000);
    const shouldSkip = isWorkerHeartbeatFresh(stalePoll);
    expect(shouldSkip).toBe(false);
  });

  it("cron should skip when worker polled exactly at TTL boundary (1 s inside)", () => {
    const justInsideTtl = Date.now() - (WORKER_LEASE_TTL_MS - 1_000);
    const shouldSkip = isWorkerHeartbeatFresh(justInsideTtl);
    expect(shouldSkip).toBe(true);
  });
});

describe("WORKER_LEASE_TTL_MS constant", () => {
  it("is 45 seconds", () => {
    expect(WORKER_LEASE_TTL_MS).toBe(45_000);
  });
});

// ── normalizeBaseUrl ──────────────────────────────────────────────────────────

describe("normalizeBaseUrl", () => {
  it("falls back to localhost when undefined", () => {
    expect(normalizeBaseUrl(undefined)).toBe("http://localhost:3000");
  });

  it("falls back to localhost when empty string", () => {
    expect(normalizeBaseUrl("")).toBe("http://localhost:3000");
  });

  it("falls back to localhost when whitespace-only", () => {
    expect(normalizeBaseUrl("   ")).toBe("http://localhost:3000");
  });

  it("prepends https:// when no protocol is present", () => {
    expect(normalizeBaseUrl("app.vercel.app")).toBe("https://app.vercel.app");
  });

  it("does NOT double-prefix when https:// already present", () => {
    expect(normalizeBaseUrl("https://app.vercel.app")).toBe("https://app.vercel.app");
  });

  it("does NOT double-prefix when http:// already present", () => {
    expect(normalizeBaseUrl("http://localhost:3000")).toBe("http://localhost:3000");
  });

  it("strips trailing slashes", () => {
    expect(normalizeBaseUrl("https://app.vercel.app///")).toBe("https://app.vercel.app");
  });

  it("strips trailing slashes from bare host too", () => {
    expect(normalizeBaseUrl("app.vercel.app/")).toBe("https://app.vercel.app");
  });

  it("preserves path segments after the host", () => {
    expect(normalizeBaseUrl("https://app.vercel.app/api")).toBe("https://app.vercel.app/api");
  });
});

// ── health endpoint: freshness gate ──────────────────────────────────────────
//
// The health endpoint uses `isWorkerHeartbeatFresh(state.worker_last_poll_at)`
// — the same function as the cron route — so these tests cover its contract
// from the health perspective.

describe("health endpoint freshness gate (via isWorkerHeartbeatFresh)", () => {
  it("reports stale when worker_last_poll_at is null (worker never started)", () => {
    expect(isWorkerHeartbeatFresh(null)).toBe(false);
  });

  it("reports fresh when worker updated within the last second", () => {
    expect(isWorkerHeartbeatFresh(Date.now() - 500)).toBe(true);
  });

  it("reports fresh at 44 seconds old (inside 45-s TTL)", () => {
    expect(isWorkerHeartbeatFresh(Date.now() - 44_000)).toBe(true);
  });

  it("reports stale at 46 seconds old (outside 45-s TTL)", () => {
    expect(isWorkerHeartbeatFresh(Date.now() - 46_000)).toBe(false);
  });

  it("reports stale when worker_last_poll_at is 0 (epoch)", () => {
    // 0 is more than 45 s ago, so stale
    expect(isWorkerHeartbeatFresh(0)).toBe(false);
  });
});

describe("worker paper signal seed gates", () => {
  const mountedAt = 1_000_000;

  it("does not lower worker signal threshold during cold-start seed window", () => {
    expect(
      workerPaperEffectiveSignalThreshold({
        baseThreshold: 26,
        nowMs: mountedAt + 6 * 60_000,
        mountedAtMs: mountedAt,
        stratTradeCount: 0,
        productionTradeCount: 0,
        profitModeEnabled: true,
      }),
    ).toBe(26);
  });

  it("returns to strict worker threshold after seed production closes exist", () => {
    expect(
      workerPaperEffectiveSignalThreshold({
        baseThreshold: 26,
        nowMs: mountedAt + 11 * 60_000,
        mountedAtMs: mountedAt,
        stratTradeCount: 0,
        productionTradeCount: 10,
        profitModeEnabled: true,
      }),
    ).toBe(26);
  });

  it("does not relax confirmation for seed threshold drops", () => {
    expect(
      workerPaperAllowsRelaxedConfirm({
        baseThreshold: 26,
        effectiveThreshold: 18,
        productionTradeCount: 0,
        profitModeEnabled: true,
      }),
    ).toBe(false);
    expect(
      workerPaperAllowsRelaxedConfirm({
        baseThreshold: 26,
        effectiveThreshold: 26,
        productionTradeCount: 0,
        profitModeEnabled: true,
      }),
    ).toBe(false);
  });
});

describe("worker roster fallback", () => {
  it("falls back to CORE when explicit IDs are invalid and fallback is enabled", () => {
    const r = resolveWorkerStrategyRoster({
      allStrategies: FUTURES_STRAT_DEFS,
      strategyIds: [999001, 999002],
      explicit: true,
      fallbackMode: "core",
    });
    expect(r.invalidIds).toEqual([999001, 999002]);
    expect(r.fallbackApplied).toBe(true);
    expect(r.active.length).toBeGreaterThan(0);
  });

  it("reports no strategies when explicit IDs are invalid and fallback is disabled", () => {
    const r = resolveWorkerStrategyRoster({
      allStrategies: FUTURES_STRAT_DEFS,
      strategyIds: [999001],
      explicit: true,
      fallbackMode: "off",
    });
    expect(r.invalidIds).toEqual([999001]);
    expect(r.fallbackApplied).toBe(false);
    expect(r.active).toEqual([]);
  });
});
