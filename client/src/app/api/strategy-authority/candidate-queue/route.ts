import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { getCandidateQueue } from "@/lib/strategyAuthority/portfolioIntelligenceMongo";

export const dynamic = "force-dynamic";

export async function GET(): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  try {
    const queue = await getCandidateQueue();
    return NextResponse.json({ ok: true, queue });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "CANDIDATE_QUEUE_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
