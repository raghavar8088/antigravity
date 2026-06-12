import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getRegimeIntelligence, getCurrentMarketRegime } from "@/lib/strategyAuthority/portfolioIntelligenceMongo";

export const dynamic = "force-dynamic";

export async function GET(req: Request): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  const { searchParams } = new URL(req.url);
  const limit = parseInt(searchParams.get("limit") ?? "200", 10);

  try {
    const [metrics, currentRegime] = await Promise.all([
      getRegimeIntelligence(Math.min(305, limit)),
      getCurrentMarketRegime(),
    ]);
    return NextResponse.json({ ok: true, metrics, currentRegime, count: metrics.length });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "REGIME_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
