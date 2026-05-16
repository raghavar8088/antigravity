import { describe, expect, it } from "vitest";
import {
  assertCloudAccountMatchesSession,
  requireAuthenticatedPaperUser,
  resolveCloudPaperTradesAccountKey,
} from "./paperTradesAuth";

describe("resolveCloudPaperTradesAccountKey", () => {
  it("uses supabase user id when logged in", () => {
    expect(
      resolveCloudPaperTradesAccountKey({
        supabaseUserId: "uuid-abc",
        storageNamespace: "btc_future_trading_20",
      }),
    ).toBe("uuid-abc");
  });

  it("returns null when logged out (local namespace not used for cloud)", () => {
    expect(
      resolveCloudPaperTradesAccountKey({
        supabaseUserId: null,
        storageNamespace: "btc_future_trading_20",
      }),
    ).toBeNull();
  });
});

describe("requireAuthenticatedPaperUser", () => {
  it("accepts non-empty user id", () => {
    expect(requireAuthenticatedPaperUser("user-1")).toEqual({ ok: true, userId: "user-1" });
  });

  it("rejects missing user", () => {
    const r = requireAuthenticatedPaperUser(undefined);
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.status).toBe(401);
  });
});

describe("assertCloudAccountMatchesSession", () => {
  it("allows omitted account_key", () => {
    expect(assertCloudAccountMatchesSession("u1", undefined)).toEqual({ ok: true, userId: "u1" });
  });

  it("rejects mismatched account_key", () => {
    const r = assertCloudAccountMatchesSession("u1", "other");
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.status).toBe(401);
  });
});
