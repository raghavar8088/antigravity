import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getPipelineState } from "@/lib/strategyAuthority/strategyAuthorityMongo";

export const dynamic = "force-dynamic";

export async function GET(): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  try {
    const pipeline = await getPipelineState();
    return NextResponse.json({ ok: true, pipeline });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "PIPELINE_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
