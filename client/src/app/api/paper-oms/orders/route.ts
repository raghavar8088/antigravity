/**
 * GET /api/paper-oms/orders
 *
 * Query params:
 *   limit       number   (default 100, max 500)
 *   status      string   filter by PaperOmsOrderStatus
 *   strategy_id number   filter by strategyId
 *   gate        string   filter by rejection gate
 *
 * Returns newest-first OMS orders for the configured desk account.
 * Never returns secrets or full MONGODB_URI.
 */

import { NextResponse } from "next/server";
import { listPaperOmsOrders } from "@/lib/paperOmsMongo";
import type { PaperOmsOrderStatus } from "@/lib/paperOms";

export const dynamic = "force-dynamic";

const VALID_STATUSES = new Set<string>([
  "NEW", "RISK_CHECKED", "REJECTED", "ACCEPTED",
  "SIMULATED_FILL", "POSITION_OPENED", "POSITION_CLOSED", "CANCELLED",
]);

export async function GET(req: Request): Promise<NextResponse> {
  try {
    const url = new URL(req.url);
    const limit = Math.min(500, Math.max(1, Number(url.searchParams.get("limit") ?? "100") || 100));
    const rawStatus = url.searchParams.get("status") ?? "";
    const strategyId = url.searchParams.get("strategy_id");
    const gate = url.searchParams.get("gate") ?? undefined;

    const status = VALID_STATUSES.has(rawStatus) ? (rawStatus as PaperOmsOrderStatus) : undefined;
    const stratId = strategyId ? Number(strategyId) : undefined;

    const orders = await listPaperOmsOrders({
      limit,
      status,
      strategyId: stratId,
      gate,
    });

    return NextResponse.json({ ok: true, count: orders.length, orders });
  } catch (err) {
    return NextResponse.json(
      { ok: false, error: err instanceof Error ? err.message : "unknown error" },
      { status: 500 },
    );
  }
}
