/**
 * Server-side market data fetch for mock-trading signal evaluation.
 * Mirrors the deleted paper desk worker Delta Exchange fetch.
 */

const DELTA_BASE =
  process.env.DELTA_API_BASE_URL?.replace(/\/+$/, "") ?? "https://api.india.delta.exchange";

const DEFAULT_SYMBOL = process.env.DELTA_BTC_FUTURES_SYMBOL?.trim() || "BTCUSD";
const FETCH_TIMEOUT_MS = 9_000;
const N_CANDLES = 400;

export type MockTradingBar = {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

function n(value: unknown): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

export function sanitizeMockTradingSymbol(raw: string | null | undefined): string {
  const s = raw?.trim().toUpperCase() ?? "";
  if (!s || !/^[A-Z0-9]{2,20}$/.test(s)) return DEFAULT_SYMBOL;
  return s;
}

export async function fetchMockTradingKlines(symbol = DEFAULT_SYMBOL): Promise<{
  bars: MockTradingBar[];
  markPrice: number;
  fundingRate: number;
}> {
  const endSec = Math.floor(Date.now() / 1000);
  const startSec = endSec - N_CANDLES * 60;
  const headers = { Accept: "application/json", "User-Agent": "RAIG-MockTrading/1.0" };
  const signal = AbortSignal.timeout(FETCH_TIMEOUT_MS);

  const [candlesRes, tickerRes] = await Promise.all([
    fetch(
      `${DELTA_BASE}/v2/history/candles?resolution=1m&symbol=${encodeURIComponent(symbol)}&start=${startSec}&end=${endSec}`,
      { headers, signal, cache: "no-store" },
    ),
    fetch(`${DELTA_BASE}/v2/tickers/${encodeURIComponent(symbol)}`, { headers, signal, cache: "no-store" }),
  ]);

  if (!candlesRes.ok) throw new Error(`Delta candles ${candlesRes.status}`);

  const cj = (await candlesRes.json()) as { success?: boolean; result?: unknown[] };
  if (!cj.success || !Array.isArray(cj.result)) throw new Error("Delta candles bad response");

  const bars: MockTradingBar[] = cj.result
    .map((r) => {
      const row = r as Record<string, unknown>;
      const close = n(row.close);
      if (close <= 0) return null;
      return {
        time: n(row.time),
        open: n(row.open) || close,
        high: n(row.high) || close,
        low: n(row.low) || close,
        close,
        volume: n(row.volume),
      };
    })
    .filter((b): b is MockTradingBar => b !== null)
    .sort((a, b) => a.time - b.time);

  let markPrice = bars.length > 0 ? bars[bars.length - 1].close : 0;
  let fundingRate = 0;

  if (tickerRes.ok) {
    const tj = (await tickerRes.json()) as { success?: boolean; result?: Record<string, unknown> };
    if (tj.success && tj.result) {
      markPrice = n(tj.result.mark_price) || n(tj.result.close) || markPrice;
      fundingRate = n(tj.result.funding_rate);
    }
  }

  return { bars, markPrice, fundingRate };
}
