import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { getAllocationSummary } from "@/lib/strategyAuthority/portfolioIntelligenceMongo";

export const dynamic = "force-dynamic";

export async function GET(): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  try {
    const summary = await getAllocationSummary();
    return NextResponse.json({ ok: true, allocation: summary });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "ALLOCATION_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
