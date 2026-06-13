import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { getFamilyIntelligence } from "@/lib/strategyAuthority/strategyAuthorityMongo";

export const dynamic = "force-dynamic";

export async function GET(): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  try {
    const families = await getFamilyIntelligence();
    return NextResponse.json({ ok: true, families, count: families.length });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "FAMILIES_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
