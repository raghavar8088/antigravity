import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getPipelineCounts, getTowerCounts } from "@/lib/strategyAuthority/strategyAuthorityMongo";

export const dynamic = "force-dynamic";

export async function GET(): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  try {
    const [counts, tower] = await Promise.all([getPipelineCounts(), getTowerCounts()]);
    return NextResponse.json({ ok: true, counts, tower });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "COUNTS_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
