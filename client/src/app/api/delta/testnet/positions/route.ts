import { NextResponse } from "next/server";
import { DeltaClientError } from "@/server/delta/deltaErrors";
import { DeltaTestnetClient } from "@/server/delta/deltaClient";
import { guardTestnetApiRoute, guardTestnetOpsPanelEnabled } from "@/server/delta/testnetRouteGuards";

export const dynamic = "force-dynamic";

/** Read-only margined positions + open orders (panel helper). */
export async function GET() {
  const panelGuard = guardTestnetOpsPanelEnabled();
  if (panelGuard) return panelGuard;

  const guard = await guardTestnetApiRoute();
  if (!guard.ok) return guard.response;

  try {
    const client = DeltaTestnetClient.fromEnv();
    const [positions, openOrders] = await Promise.all([
      client.getPositions(),
      client.getOpenOrders(),
    ]);

    if (!positions.ok) {
      return NextResponse.json(
        { ok: false, error: positions.error },
        { status: positions.status >= 400 ? positions.status : 502 },
      );
    }

    const orders = openOrders.ok ? openOrders.data : [];

    return NextResponse.json({
      ok: true,
      testnet: true,
      positions: positions.data,
      openOrders: orders,
      openOrdersError: openOrders.ok ? undefined : openOrders.error,
    });
  } catch (e) {
    const message = e instanceof DeltaClientError ? e.message : "Failed to load positions";
    return NextResponse.json({ ok: false, error: message }, { status: 503 });
  }
}
