/**
 * /api/nifty/stream — SSE live price feed for NIFTY 50
 *
 * Data source: Yahoo Finance v7 quote API (^NSEI)
 * - No authentication required
 * - No API key required
 * - Polls every 3 seconds
 * - Works on Vercel (serverless, maxDuration 60s — client auto-reconnects)
 */

export const maxDuration = 60;

const YAHOO_QUOTE_URL =
  "https://query1.finance.yahoo.com/v7/finance/quote?symbols=%5ENSEI&fields=regularMarketPrice,regularMarketOpen,regularMarketDayHigh,regularMarketDayLow,regularMarketPreviousClose,regularMarketChange,regularMarketChangePercent";

// Fallback mirror — Yahoo sometimes throttles query1
const YAHOO_QUOTE_URL2 =
  "https://query2.finance.yahoo.com/v7/finance/quote?symbols=%5ENSEI&fields=regularMarketPrice,regularMarketOpen,regularMarketDayHigh,regularMarketDayLow,regularMarketPreviousClose,regularMarketChange,regularMarketChangePercent";

type YahooQuoteResult = {
  regularMarketPrice?: number;
  regularMarketOpen?: number;
  regularMarketDayHigh?: number;
  regularMarketDayLow?: number;
  regularMarketPreviousClose?: number;
  regularMarketChange?: number;
  regularMarketChangePercent?: number;
};

type YahooQuoteResponse = {
  quoteResponse?: {
    result?: YahooQuoteResult[];
    error?: unknown;
  };
};

function n(v: unknown): number {
  const num = Number(v);
  return Number.isFinite(num) ? num : 0;
}

async function fetchNiftyQuote(): Promise<YahooQuoteResult | null> {
  const headers = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
    "Accept": "application/json",
  };

  for (const url of [YAHOO_QUOTE_URL, YAHOO_QUOTE_URL2]) {
    try {
      const res = await fetch(url, { headers, cache: "no-store" });
      if (!res.ok) continue;
      const data = await res.json() as YahooQuoteResponse;
      const result = data?.quoteResponse?.result?.[0];
      if (result?.regularMarketPrice) return result;
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
            const quote = await fetchNiftyQuote();

            if (quote) {
              const price = n(quote.regularMarketPrice);
              const open  = n(quote.regularMarketOpen);
              const high  = n(quote.regularMarketDayHigh);
              const low   = n(quote.regularMarketDayLow);
              const close = n(quote.regularMarketPreviousClose);
              const change = n(quote.regularMarketChange);
              const percentChange = n(quote.regularMarketChangePercent);

              controller.enqueue(encoder.encode(
                `data: ${JSON.stringify({
                  price,
                  open,
                  high,
                  low,
                  close,
                  change,
                  percentChange,
                  source: "yahoo",
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

          // Poll every 3 seconds — Yahoo rate limit is generous at this pace
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
