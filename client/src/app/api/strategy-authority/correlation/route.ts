import { NextResponse } from "next/server";
import { isMongoConfigured, getDb } from "@/lib/mongoTradesClient";
import { getCorrelationMatrix, STRATEGY_CORRELATIONS_COLLECTION } from "@/lib/strategyAuthority/correlationEngine";

export const dynamic = "force-dynamic";

export async function GET(req: Request): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  const { searchParams } = new URL(req.url);
  const limitParam = parseInt(searchParams.get("limit") ?? "40", 10);
  const limit = Math.min(60, Math.max(5, limitParam));

  try {
    const db = await getDb();

    // Get top strategies by diversification score to build focused matrix
    const topStrategies = await db.collection(STRATEGY_CORRELATIONS_COLLECTION)
      .aggregate([
        { $group: { _id: "$strategy_id_a", name: { $first: "$strategy_name_a" } } },
        { $limit: limit },
      ])
      .toArray();

    const strategyIds = topStrategies.map((s) => s._id as string);
    const { rows, strategies } = await getCorrelationMatrix(db, strategyIds);

    return NextResponse.json({
      ok: true,
      pairs: rows.length,
      strategies,
      matrix: rows,
    });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "CORRELATION_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
