import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextResponse } from "next/server";

vi.mock("@/lib/paperTradesApiAuth", () => ({
  getAuthenticatedPaperApiUser: vi.fn(),
}));

vi.mock("@/lib/shadowTradeIntentMapper", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/shadowTradeIntentMapper")>();
  return {
    ...actual,
    isDeskShadowIntentsEnabled: vi.fn(() => true),
  };
});

vi.mock("@/server/delta/shadowWouldPlaceTestnet", () => ({
  computeWouldPlaceTestnetShadow: vi.fn(() => false),
}));

const mockUpsert = vi.fn(async () => ({ error: null }));
const mockSelect = vi.fn();

vi.mock("@/lib/supabase/server", () => ({
  createServiceSupabase: () => ({
    from: (table: string) => {
      if (table !== "shadow_trade_intents") throw new Error(`unexpected table ${table}`);
      return {
        upsert: mockUpsert,
        select: () => ({
          eq: () => ({
            order: () => ({
              limit: mockSelect,
            }),
          }),
        }),
      };
    },
  }),
}));

import { getAuthenticatedPaperApiUser } from "@/lib/paperTradesApiAuth";
import { POST, GET } from "./route";

describe("shadow-trade-intents API", () => {
  beforeEach(() => {
    vi.mocked(getAuthenticatedPaperApiUser).mockResolvedValue({
      ok: true,
      ctx: { userId: "user-1" },
    });
    mockUpsert.mockClear();
    mockSelect.mockResolvedValue({ data: [], error: null });
  });

  it("POST returns 401 without session", async () => {
    vi.mocked(getAuthenticatedPaperApiUser).mockResolvedValueOnce({
      ok: false,
      response: NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 }),
    });

    const res = await POST(
      new Request("http://localhost/api/shadow-trade-intents", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          intentKind: "close",
          symbol: "BTCUSD",
          side: "LONG",
          notional: 100,
          entryPrice: 1,
          exitPrice: 2,
          exitReason: "TP",
          strategyId: 1,
        }),
      }),
    );
    expect(res.status).toBe(401);
  });

  it("POST validates close body requires exit fields", async () => {
    const res = await POST(
      new Request("http://localhost/api/shadow-trade-intents", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          intentKind: "close",
          symbol: "BTCUSD",
          side: "LONG",
          notional: 100,
          entryPrice: 1,
          strategyId: 1,
        }),
      }),
    );
    expect(res.status).toBe(400);
  });

  it("POST inserts close intent", async () => {
    const res = await POST(
      new Request("http://localhost/api/shadow-trade-intents", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          intentKind: "close",
          symbol: "BTCUSD",
          side: "LONG",
          notional: 100,
          entryPrice: 80_000,
          exitPrice: 81_000,
          exitReason: "TP",
          strategyId: 91,
          strategyName: "Trend",
        }),
      }),
    );
    const json = (await res.json()) as { ok: boolean; wouldPlaceTestnet?: boolean };
    expect(res.status).toBe(200);
    expect(json.ok).toBe(true);
    expect(mockUpsert).toHaveBeenCalled();
  });

  it("GET lists intents for user", async () => {
    mockSelect.mockResolvedValueOnce({
      data: [
        {
          id: "a",
          created_at: "2026-05-16T12:00:00.000Z",
          user_id: "user-1",
          client_intent_id: "550e8400-e29b-41d4-a716-446655440000",
          intent_kind: "open",
          symbol: "BTCUSD",
          side: "LONG",
          notional: 50,
          entry_price: 90_000,
          exit_price: null,
          exit_reason: null,
          strategy_id: 92,
          strategy_name: "X",
          would_place_testnet: false,
        },
      ],
      error: null,
    });

    const res = await GET(new Request("http://localhost/api/shadow-trade-intents?limit=20"));
    const json = (await res.json()) as { ok: boolean; intents?: { intentKind: string }[] };
    expect(json.ok).toBe(true);
    expect(json.intents?.[0]?.intentKind).toBe("open");
  });
});
