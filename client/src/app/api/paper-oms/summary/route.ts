/**
 * GET /api/paper-oms/summary
 *
 * Query params:
 *   window_minutes  number  (default 120)
 *
 * Returns aggregated OMS metrics for the given window:
 *   countsByStatus, topRejectGates, latestOrders, fillRate, rejectRate
 */

import { NextResponse } from "next/server";
import { listPaperOmsOrders } from "@/lib/paperOmsMongo";
import { summarizeOmsOrders } from "@/lib/paperOms";

export const dynamic = "force-dynamic";

export async function GET(req: Request): Promise<NextResponse> {
  try {
    const url = new URL(req.url);
    const windowMinutes = Math.min(
      1440,
      Math.max(1, Number(url.searchParams.get("window_minutes") ?? "120") || 120),
    );
    const sinceMs = Date.now() - windowMinutes * 60 * 1000;

    const orders = await listPaperOmsOrders({ limit: 500, sinceMs });
    const summary = summarizeOmsOrders(orders);

    // Top rejected gates (sorted by count desc)
    const gateCounts = new Map<string, number>();
    for (const order of orders) {
      if (order.status === "REJECTED" && order.gate) {
        gateCounts.set(order.gate, (gateCounts.get(order.gate) ?? 0) + 1);
      }
    }
    const topRejectGates = [...gateCounts.entries()]
      .sort((a, b) => b[1] - a[1])
      .map(([gate, count]) => ({ gate, count }));

    const latestOrders = orders.slice(0, 20);

    return NextResponse.json({
      ok: true,
      windowMinutes,
      generatedAt: new Date().toISOString(),
      total: summary.total,
      countsByStatus: summary.countsByStatus,
      topRejectGates,
      latestOrders,
      fillRate: summary.fillRate,
      rejectRate: summary.rejectRate,
    });
  } catch (err) {
    return NextResponse.json(
      { ok: false, error: err instanceof Error ? err.message : "unknown error" },
      { status: 500 },
    );
  }
}
