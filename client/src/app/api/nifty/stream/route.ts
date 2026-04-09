import { getAngelJWT, invalidateAngelJWT, isAngelConfigured, commonHeaders, BASE_URL } from "@/lib/angelAuth";

export const maxDuration = 60;

function n(v: unknown): number {
  const num = Number(v);
  return Number.isFinite(num) ? num : 0;
}

export async function GET() {
  if (!isAngelConfigured()) {
    const encoder = new TextEncoder();
    const body = encoder.encode(`data: {"error":"Angel One not configured"}\n\n`);
    return new Response(body, {
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        "Connection": "keep-alive",
      },
    });
  }

  const encoder = new TextEncoder();
  const stream = new ReadableStream({
    async start(controller) {
      let active = true;

      // Handle client disconnect
      const cleanup = () => { active = false; };

      try {
        while (active) {
          try {
            const jwt = await getAngelJWT();

            const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/order/v1/getLtpData`, {
              method: "POST",
              headers: commonHeaders(jwt),
              body: JSON.stringify({ exchange: "NSE", tradingsymbol: "NIFTY 50", symboltoken: "99926000" }),
              next: { revalidate: 0 },
            });

            if (res.status === 401) {
              invalidateAngelJWT();
              controller.enqueue(encoder.encode(`data: {"error":"auth expired, re-logging in"}\n\n`));
            } else if (res.ok) {
              const payload = await res.json() as {
                status?: boolean;
                data?: {
                  ltp?: unknown;
                  open?: unknown;
                  high?: unknown;
                  low?: unknown;
                  close?: unknown;
                };
              };

              if (payload.status) {
                const d = payload.data ?? {};
                const price = n(d.ltp);
                const close = n(d.close);
                const change = close > 0 ? price - close : 0;
                const percentChange = close > 0 ? (change / close) * 100 : 0;
                const data = JSON.stringify({
                  price,
                  open: n(d.open),
                  high: n(d.high),
                  low: n(d.low),
                  close,
                  change,
                  percentChange,
                  fetched_at: new Date().toISOString(),
                });
                controller.enqueue(encoder.encode(`data: ${data}\n\n`));
              } else {
                controller.enqueue(encoder.encode(`data: {"error":"LTP status false"}\n\n`));
              }
            } else {
              controller.enqueue(encoder.encode(`data: {"error":"fetch failed ${res.status}"}\n\n`));
            }
          } catch {
            controller.enqueue(encoder.encode(`data: {"error":"fetch failed"}\n\n`));
          }

          await new Promise<void>((r) => setTimeout(r, 2000));
        }
      } finally {
        cleanup();
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
