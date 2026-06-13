import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { evaluateAndUpdateGrades } from "@/lib/strategyAuthority/strategyAuthorityMongo";

export const dynamic = "force-dynamic";

export async function POST(): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  try {
    const result = await evaluateAndUpdateGrades();
    return NextResponse.json({
      ok: true,
      message: `Evaluation complete. Promoted: ${result.promoted}, Demoted: ${result.demoted}, Retired: ${result.retired}`,
      ...result,
    });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "EVALUATE_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
