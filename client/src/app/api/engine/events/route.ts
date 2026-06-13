import { getAuthenticatedApiSession } from "@/lib/broker/getAuthenticatedApiSession";
import { buildPlatformEvents } from "@/lib/trading/platformEvents";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function GET(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  const encoder = new TextEncoder();
  let closed = false;
  req.signal.addEventListener("abort", () => { closed = true; });

  const stream = new ReadableStream({
    async start(controller) {
      while (!closed) {
        try {
          const events = await buildPlatformEvents(accountKey);
          controller.enqueue(
            encoder.encode(`data: ${JSON.stringify({ ok: true, events, server_time: new Date().toISOString() })}\n\n`),
          );
        } catch (e) {
          controller.enqueue(
            encoder.encode(`data: ${JSON.stringify({ ok: false, error: e instanceof Error ? e.message : "failed" })}\n\n`),
          );
        }
        await new Promise((r) => setTimeout(r, 3000));
      }
      controller.close();
    },
  });

  return new Response(stream, {
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache, no-transform",
      Connection: "keep-alive",
    },
  });
}
