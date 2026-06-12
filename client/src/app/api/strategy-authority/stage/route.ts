import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getStrategiesByStatus } from "@/lib/strategyAuthority/strategyAuthorityMongo";
import type { StrategyStatus } from "@/lib/strategyAuthority/types";

export const dynamic = "force-dynamic";

const VALID_STATUSES: StrategyStatus[] = [
  "GRADE_5", "GRADE_4", "GRADE_3", "GRADE_2", "GRADE_1", "MAIN_ENGINE",
];

export async function GET(req: Request): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }

  const url = new URL(req.url);
  const status = url.searchParams.get("status") as StrategyStatus | null;
  if (!status || !VALID_STATUSES.includes(status)) {
    return NextResponse.json(
      { ok: false, code: "INVALID_STATUS", error: "status must be one of GRADE_5..GRADE_1 or MAIN_ENGINE" },
      { status: 400 }
    );
  }

  try {
    const { strategies, summary } = await getStrategiesByStatus(status);
    return NextResponse.json({ ok: true, strategies, summary });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "STAGE_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
