import { createHmac } from "crypto";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { deltaSign } from "@/lib/broker/deltaSign";
import { buildDeltaSignedHeaders, DeltaTestnetClient } from "./deltaClient";
import { isDeltaTestnetExecutionEnabled } from "./deltaConfig";

describe("deltaSign / buildDeltaSignedHeaders", () => {
  it("matches HMAC-SHA256(method + timestamp + path + body)", () => {
    const method = "GET";
    const path = "/v2/wallet/balances";
    const body = "";
    const ts = "1710000000";
    const secret = "test-secret";

    const expected = createHmac("sha256", secret)
      .update(method + ts + path + body)
      .digest("hex");

    expect(deltaSign(method, path, body, ts, secret)).toBe(expected);

    const headers = buildDeltaSignedHeaders(method, path, body, "key-id", secret, ts);
    expect(headers.signature).toBe(expected);
    expect(headers["api-key"]).toBe("key-id");
    expect(headers.timestamp).toBe(ts);
  });

  it("includes body in signature for POST /v2/orders", () => {
    const body = JSON.stringify({ product_id: 1, size: 1, side: "buy", order_type: "market_order" });
    const ts = "1710000001";
    const secret = "s";
    const sig = deltaSign("POST", "/v2/orders", body, ts, secret);
    const headers = buildDeltaSignedHeaders("POST", "/v2/orders", body, "k", secret, ts);
    expect(headers.signature).toBe(sig);
  });
});

describe("DeltaTestnetClient (mocked HTTP)", () => {
  const mockHttp = vi.fn();

  beforeEach(() => {
    vi.stubEnv("DELTA_TESTNET", "1");
    vi.stubEnv("DELTA_API_KEY", "test-key");
    vi.stubEnv("DELTA_API_SECRET", "test-secret");
    mockHttp.mockReset();
  });

  it("refuses when DELTA_TESTNET is not enabled", () => {
    vi.stubEnv("DELTA_TESTNET", "0");
    expect(isDeltaTestnetExecutionEnabled()).toBe(false);
    expect(() => DeltaTestnetClient.fromEnv(mockHttp)).toThrow(/DELTA_TESTNET/);
  });

  it("getBalances sends signed GET to testnet path", async () => {
    mockHttp.mockResolvedValueOnce({
      ok: true,
      status: 200,
      data: {
        result: [
          {
            asset_symbol: "USDT",
            balance: "100",
            available_balance: "90",
            blocked_margin: "10",
            unrealised_cashflow: "0",
          },
        ],
      },
    });

    const client = DeltaTestnetClient.fromEnv(mockHttp);
    const result = await client.getBalances();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.data[0]?.asset).toBe("USDT");
      expect(result.data[0]?.availableBalance).toBe(90);
    }

    expect(mockHttp).toHaveBeenCalledTimes(1);
    const [url, method, headers] = mockHttp.mock.calls[0] as [string, string, Record<string, string>];
    expect(url).toContain("testnet-api.delta.exchange/v2/wallet/balances");
    expect(method).toBe("GET");
    expect(headers["api-key"]).toBe("test-key");
    expect(headers.signature).toBe(
      deltaSign("GET", "/v2/wallet/balances", "", headers.timestamp, "test-secret"),
    );
  });

  it("placeOrder POSTs JSON body with matching signature", async () => {
    mockHttp.mockResolvedValueOnce({
      ok: true,
      status: 200,
      data: { result: { id: 42, symbol: "BTCUSD", state: "open" } },
    });

    const client = DeltaTestnetClient.fromEnv(mockHttp);
    const result = await client.placeOrder({
      productId: 27,
      size: 1,
      side: "buy",
      orderType: "market_order",
    });

    expect(result.ok).toBe(true);
    const [, method, headers, body] = mockHttp.mock.calls[0] as [
      string,
      string,
      Record<string, string>,
      string,
    ];
    expect(method).toBe("POST");
    expect(body).toContain('"product_id":27');
    expect(headers.signature).toBe(deltaSign("POST", "/v2/orders", body, headers.timestamp, "test-secret"));
  });

  it("cancelOrder uses DELETE /v2/orders/:id", async () => {
    mockHttp.mockResolvedValueOnce({ ok: true, status: 200, data: { success: true } });

    const client = DeltaTestnetClient.fromEnv(mockHttp);
    const result = await client.cancelOrder(99);

    expect(result.ok).toBe(true);
    const [url, method] = mockHttp.mock.calls[0] as [string, string];
    expect(url).toContain("/v2/orders/99");
    expect(method).toBe("DELETE");
  });
});
