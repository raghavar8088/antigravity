/**
 * /api/nifty/stream — SSE live price feed for NIFTY 50
 *
 * Data source: Yahoo Finance v8 chart API (^NSEI)
 * - No authentication required
 * - Uses the same v8/chart endpoint as candles (v7/quote returns 401)
 * - regularMarketPrice from chart meta gives the live price
 * - Polls every 3 seconds, dual-mirror fallback
 * - Works on Vercel (serverless, maxDuration 60s — client auto-reconnects)
 */

export const maxDuration = 60;

const CHART_URL1 =
  "https://query1.finance.yahoo.com/v8/finance/chart/%5ENSEI?interval=1m&range=1d&includePrePost=false";
const CHART_URL2 =
  "https://query2.finance.yahoo.com/v8/finance/chart/%5ENSEI?interval=1m&range=1d&includePrePost=false";

const HEADERS = {
  "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
  "Accept": "application/json",
};

type ChartMeta = {
  regularMarketPrice?: number;
  regularMarketOpen?: number;
  regularMarketDayHigh?: number;
  regularMarketDayLow?: number;
  chartPreviousClose?: number;
  previousClose?: number;
};

type ChartResponse = {
  chart?: {
    result?: Array<{ meta?: ChartMeta }>;
    error?: unknown;
  };
};

function n(v: unknown): number {
  const num = Number(v);
  return Number.isFinite(num) ? num : 0;
}

async function fetchNiftyQuote(): Promise<ChartMeta | null> {
  for (const url of [CHART_URL1, CHART_URL2]) {
    try {
      const res = await fetch(url, { headers: HEADERS, cache: "no-store" });
      if (!res.ok) continue;
      const data = await res.json() as ChartResponse;
      const meta = data?.chart?.result?.[0]?.meta;
      if (meta?.regularMarketPrice) return meta;
    } catch {
      // try next mirror
    }
  }
  return null;
}

export async function GET() {
  const encoder = new TextEncoder();

  const stream = new ReadableStream({
    async start(controller) {
      let active = true;

      try {
        while (active) {
          try {
            const meta = await fetchNiftyQuote();

            if (meta) {
              const price = n(meta.regularMarketPrice);
              const open  = n(meta.regularMarketOpen);
              const high  = n(meta.regularMarketDayHigh);
              const low   = n(meta.regularMarketDayLow);
              const close = n(meta.chartPreviousClose ?? meta.previousClose);

              controller.enqueue(encoder.encode(
                `data: ${JSON.stringify({
                  price,
                  open,
                  high,
                  low,
                  close,
                  change: price - close,
                  percentChange: close > 0 ? ((price - close) / close) * 100 : 0,
                  source: "yahoo-v8",
                  fetched_at: new Date().toISOString(),
                })}\n\n`,
              ));
            } else {
              controller.enqueue(encoder.encode(
                `data: {"error":"Yahoo Finance unavailable"}\n\n`,
              ));
            }
          } catch {
            controller.enqueue(encoder.encode(
              `data: {"error":"stream fetch failed"}\n\n`,
            ));
          }

          // Poll every 3 seconds
          await new Promise<void>((r) => setTimeout(r, 3000));
        }
      } finally {
        active = false;
        try { controller.close(); } catch { /* already closed */ }
      }
    },
  });

  return new Response(stream, {
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      "Connection": "keep-alive",
    },
  });
}
