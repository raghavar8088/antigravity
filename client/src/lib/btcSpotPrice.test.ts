import { describe, expect, it, vi } from "vitest";
import { fetchBtcSpotPrice } from "./btcSpotPrice";

describe("fetchBtcSpotPrice", () => {
  it("returns Binance spot when the primary feed succeeds", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) => {
        if (url.includes("binance.com")) {
          return new Response(
            JSON.stringify({
              lastPrice: "65000.5",
              priceChangePercent: "1.25",
              highPrice: "66000",
              lowPrice: "64000",
              volume: "12345",
            }),
            { status: 200 },
          );
        }
        throw new Error("unexpected fetch");
      }),
    );

    const spot = await fetchBtcSpotPrice();
    expect(spot).toMatchObject({
      price: 65000.5,
      change24h: 1.25,
      source: "binance",
    });
  });
});
