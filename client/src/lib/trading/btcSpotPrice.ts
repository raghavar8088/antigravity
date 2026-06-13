export type BtcSpotPrice = {
  price: number;
  change24h: number;
  high24h: number;
  low24h: number;
  volume24h: number;
  source: "binance" | "coinbase";
};

async function fetchBinanceBtcSpot(): Promise<BtcSpotPrice | null> {
  try {
    const res = await fetch("https://api.binance.com/api/v3/ticker/24hr?symbol=BTCUSDT", {
      signal: AbortSignal.timeout(4_000),
    });
    if (!res.ok) return null;
    const d = await res.json() as Record<string, string>;
    const price = parseFloat(d.lastPrice);
    if (!Number.isFinite(price) || price <= 0) return null;
    return {
      price,
      change24h: parseFloat(d.priceChangePercent),
      high24h: parseFloat(d.highPrice),
      low24h: parseFloat(d.lowPrice),
      volume24h: parseFloat(d.volume),
      source: "binance",
    };
  } catch {
    return null;
  }
}

async function fetchCoinbaseBtcSpot(): Promise<BtcSpotPrice | null> {
  try {
    const [spotRes, statsRes] = await Promise.allSettled([
      fetch("https://api.coinbase.com/v2/prices/BTC-USD/spot", { signal: AbortSignal.timeout(4_000) }),
      fetch("https://api.exchange.coinbase.com/products/BTC-USD/stats", { signal: AbortSignal.timeout(4_000) }),
    ]);

    let price = 0;
    let high24h = 0;
    let low24h = 0;
    let volume24h = 0;
    let change24h = 0;

    if (spotRes.status === "fulfilled" && spotRes.value.ok) {
      const d = await spotRes.value.json() as { data: { amount: string } };
      price = parseFloat(d.data.amount);
    }

    if (statsRes.status === "fulfilled" && statsRes.value.ok) {
      const s = await statsRes.value.json() as {
        high: string;
        low: string;
        volume: string;
        open: string;
        last: string;
      };
      high24h = parseFloat(s.high);
      low24h = parseFloat(s.low);
      volume24h = parseFloat(s.volume);
      const open = parseFloat(s.open);
      const last = parseFloat(s.last);
      if (price === 0 && Number.isFinite(last) && last > 0) price = last;
      if (Number.isFinite(open) && open > 0 && Number.isFinite(price) && price > 0) {
        change24h = ((price - open) / open) * 100;
      }
    }

    if (!Number.isFinite(price) || price <= 0) return null;
    return { price, change24h, high24h, low24h, volume24h, source: "coinbase" };
  } catch {
    return null;
  }
}

/** Fetch live BTC spot from public exchange APIs (server-safe, no auth cookie required). */
export async function fetchBtcSpotPrice(): Promise<BtcSpotPrice | null> {
  const binance = await fetchBinanceBtcSpot();
  if (binance) return binance;
  return fetchCoinbaseBtcSpot();
}
