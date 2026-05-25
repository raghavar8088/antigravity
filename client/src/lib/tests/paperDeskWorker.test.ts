import { describe, expect, it } from "vitest";
import {
  isWorkerHeartbeatFresh,
  workerHeartbeatAgeSeconds,
  WORKER_LEASE_TTL_MS,
} from "../paperDeskWorker/workerLease";

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
