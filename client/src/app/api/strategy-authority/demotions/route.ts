import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getRecentDemotions } from "@/lib/strategyAuthority/strategyAuthorityMongo";

export const dynamic = "force-dynamic";

export async function GET(req: Request): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  try {
    const url = new URL(req.url);
    const limit = Math.min(parseInt(url.searchParams.get("limit") ?? "50"), 200);
    const demotions = await getRecentDemotions(limit);
    return NextResponse.json({ ok: true, demotions });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "DEMOTIONS_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
