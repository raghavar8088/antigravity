import { NextResponse } from "next/server";
import { DeltaClientError } from "@/server/delta/deltaErrors";
import { DeltaTestnetClient } from "@/server/delta/deltaClient";
import { appendDeltaAuditLog } from "@/server/delta/deltaTestnetAudit";
import { testnetCancelOrderBodySchema } from "@/server/delta/deltaTestnetSchemas";
import { guardTestnetApiRoute, guardTestnetOpsPanelEnabled } from "@/server/delta/testnetRouteGuards";

export const dynamic = "force-dynamic";

export async function POST(req: Request) {
  const panelGuard = guardTestnetOpsPanelEnabled();
  if (panelGuard) return panelGuard;

  const guard = await guardTestnetApiRoute();
  if (!guard.ok) return guard.response;

  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON" }, { status: 400 });
  }

  const parsed = testnetCancelOrderBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, error: "Validation failed", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  const orderId = String(parsed.data.orderId);

  try {
    const client = DeltaTestnetClient.fromEnv();
    const result = await client.cancelOrder(orderId);

    if (!result.ok) {
      await appendDeltaAuditLog({
        userId: guard.ctx.userId,
        action: "cancel_order",
        orderId,
        status: "error",
        payload: { error: result.error },
      });
      return NextResponse.json(
        { ok: false, error: result.error },
        { status: result.status >= 400 ? result.status : 502 },
      );
    }

    await appendDeltaAuditLog({
      userId: guard.ctx.userId,
      action: "cancel_order",
      orderId,
      status: "cancelled",
    });

    return NextResponse.json({
      ok: true,
      testnet: true,
      orderId,
      cancelled: true,
    });
  } catch (e) {
    const message = e instanceof DeltaClientError ? e.message : "Cancel order failed";
    return NextResponse.json({ ok: false, error: message }, { status: 503 });
  }
}
