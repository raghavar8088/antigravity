/**
 * FEAT 1 — SSE Live Position Feed
 * Streams live unrealized PnL for all open mock trades every 2 seconds.
 */
import { getDb } from "@/lib/broker/mongoTradesClient";
import { MOCK_TRADES_COLLECTION } from "@/lib/trading/mockTradingMongo";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

async function fetchBTCPrice(): Promise<number> {
  try {
    const res = await fetch(
      "https://api.binance.com/api/v3/ticker/price?symbol=BTCUSDT",
      { next: { revalidate: 0 } },
    );
    if (!res.ok) return 0;
    const data = (await res.json()) as { price?: string };
    return parseFloat(data.price ?? "0") || 0;
  } catch {
    return 0;
  }
}

export async function GET(req: Request) {
  const encoder = new TextEncoder();
  const accountKey = OWNER_ACCOUNT_KEY;

  const stream = new ReadableStream({
    async start(controller) {
      const abort = req.signal;

      const sendEvent = (data: unknown) => {
        try {
          controller.enqueue(encoder.encode(`data: ${JSON.stringify(data)}\n\n`));
        } catch {
          // stream closed
        }
      };

      const tick = async () => {
        try {
          const [db, price] = await Promise.all([getDb(), fetchBTCPrice()]);
          const col = db.collection(MOCK_TRADES_COLLECTION);
          const openTrades = await col
            .find({ account_key: accountKey, status: "OPEN" })
            .limit(100)
            .toArray();

          const positions = openTrades.map((t) => {
            const notional = (t.notional as number) ?? 0;
            const entryPrice = (t.entry_price as number) ?? 0;
            const side = (t.side as string) ?? "BUY";
            let unrealizedPnl = 0;
            if (entryPrice > 0 && price > 0 && notional > 0) {
              const pct =
                side === "BUY"
                  ? (price - entryPrice) / entryPrice
                  : (entryPrice - price) / entryPrice;
              unrealizedPnl = pct * notional;
            }
            return {
              trade_id: t.trade_id,
              strategy_name: t.strategy_name,
              side,
              entry_price: entryPrice,
              current_price: price,
              notional,
              unrealized_pnl: unrealizedPnl,
            };
          });

          sendEvent({ type: "positions", positions, price, timestamp: Date.now() });
        } catch (err) {
          sendEvent({ type: "error", message: err instanceof Error ? err.message : "unknown" });
        }
      };

      // Send initial data immediately then every 2 seconds.
      await tick();
      const interval = setInterval(tick, 2_000);

      abort.addEventListener("abort", () => {
        clearInterval(interval);
        try { controller.close(); } catch { /* already closed */ }
      });
    },
  });

  return new Response(stream, {
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    },
  });
}
