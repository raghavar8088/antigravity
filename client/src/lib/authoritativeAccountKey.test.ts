import { afterEach, describe, expect, it } from "vitest";
import {
  FRONTEND_OWNER_ACCOUNT_KEY,
  validateAccountKeyAlignment,
} from "./authoritativeAccountKey";

describe("authoritativeAccountKey", () => {
  const origOwner = process.env.OWNER_ACCOUNT_KEY;
  const origWorker = process.env.DESK_WORKER_ACCOUNT_KEY;

  afterEach(() => {
    if (origOwner === undefined) delete process.env.OWNER_ACCOUNT_KEY;
    else process.env.OWNER_ACCOUNT_KEY = origOwner;
    if (origWorker === undefined) delete process.env.DESK_WORKER_ACCOUNT_KEY;
    else process.env.DESK_WORKER_ACCOUNT_KEY = origWorker;
  });

  it("defaults to mock_trading_default", () => {
    delete process.env.OWNER_ACCOUNT_KEY;
    delete process.env.DESK_WORKER_ACCOUNT_KEY;
    const r = validateAccountKeyAlignment();
    expect(r.ok).toBe(true);
    expect(r.accountKey).toBe(FRONTEND_OWNER_ACCOUNT_KEY);
  });

  it("fails on anon worker key", () => {
    process.env.DESK_WORKER_ACCOUNT_KEY = "anon_e7da5e39";
    const r = validateAccountKeyAlignment();
    expect(r.ok).toBe(false);
    expect(r.errors[0]).toContain("anon_e7da5e39");
  });

  it("fails when OWNER_ACCOUNT_KEY mismatches frontend", () => {
    process.env.OWNER_ACCOUNT_KEY = "owner_admin";
    const r = validateAccountKeyAlignment();
    expect(r.ok).toBe(false);
  });
});
