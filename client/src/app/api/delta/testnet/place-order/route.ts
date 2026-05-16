import { NextResponse } from "next/server";
import { DeltaClientError } from "@/server/delta/deltaErrors";
import { DeltaTestnetClient } from "@/server/delta/deltaClient";
import { appendDeltaAuditLog } from "@/server/delta/deltaTestnetAudit";
import {
  checkTestnetPlaceOrderRateLimit,
  recordTestnetPlaceOrder,
} from "@/server/delta/deltaTestnetRateLimit";
import { testnetPlaceOrderBodySchema } from "@/server/delta/deltaTestnetSchemas";
import { resolveTestnetPerpProductId } from "@/server/delta/resolveTestnetProduct";
import { guardTestnetApiRoute, guardTestnetOpsPanelEnabled } from "@/server/delta/testnetRouteGuards";

export const dynamic = "force-dynamic";

export async function POST(req: Request) {
  const panelGuard = guardTestnetOpsPanelEnabled();
  if (panelGuard) return panelGuard;

  const guard = await guardTestnetApiRoute();
  if (!guard.ok) return guard.response;

  const rate = checkTestnetPlaceOrderRateLimit(guard.ctx.userId);
  if (!rate.allowed) {
    return NextResponse.json(
      {
        ok: false,
        error: "Rate limit: max 10 testnet place-order requests per hour",
        retryAfterSec: rate.retryAfterSec,
      },
      { status: 429, headers: { "Retry-After": String(rate.retryAfterSec) } },
    );
  }

  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON" }, { status: 400 });
  }

  const parsed = testnetPlaceOrderBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, error: "Validation failed", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  const { symbol, side, size, type, price } = parsed.data;

  try {
    const productId = await resolveTestnetPerpProductId(symbol);
    if (!productId) {
      return NextResponse.json(
        { ok: false, error: `Unknown testnet perpetual symbol: ${symbol}` },
        { status: 404 },
      );
    }

    const client = DeltaTestnetClient.fromEnv();
    const result = await client.placeOrder({
      productId,
      size,
      side,
      orderType: type === "market" ? "market_order" : "limit_order",
      limitPrice: type === "limit" ? price : undefined,
    });

    if (!result.ok) {
      await appendDeltaAuditLog({
        userId: guard.ctx.userId,
        action: "place_order",
        symbol,
        side,
        size,
        status: "error",
        payload: { error: result.error, status: result.status },
      });
      return NextResponse.json(
        { ok: false, error: result.error },
        { status: result.status >= 400 ? result.status : 502 },
      );
    }

    recordTestnetPlaceOrder(guard.ctx.userId);
    await appendDeltaAuditLog({
      userId: guard.ctx.userId,
      action: "place_order",
      symbol,
      side,
      size,
      orderId: result.data.orderId,
      status: result.data.state,
      payload: { type, price: price ?? null, productId },
    });

    return NextResponse.json({
      ok: true,
      testnet: true,
      orderId: result.data.orderId,
      status: result.data.state,
      symbol: result.data.symbol || symbol,
      averageFillPrice: result.data.averageFillPrice,
      rateLimitRemaining: rate.remaining - 1,
    });
  } catch (e) {
    const message = e instanceof DeltaClientError ? e.message : "Place order failed";
    return NextResponse.json({ ok: false, error: message }, { status: 503 });
  }
}
