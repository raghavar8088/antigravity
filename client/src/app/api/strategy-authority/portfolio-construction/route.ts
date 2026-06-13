import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { getPortfolioConstruction } from "@/lib/strategyAuthority/portfolioIntelligenceMongo";

export const dynamic = "force-dynamic";

export async function GET(): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  try {
    const construction = await getPortfolioConstruction();
    if (!construction) {
      return NextResponse.json({ ok: false, code: "NOT_COMPUTED", error: "Run portfolio intelligence compute first" }, { status: 404 });
    }
    return NextResponse.json({ ok: true, construction });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "CONSTRUCTION_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
