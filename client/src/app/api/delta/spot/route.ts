import { NextResponse } from "next/server";
import { blockedDirectExecutionRoute } from "@/lib/risk/blockedExecutionRoute";

export const dynamic = "force-dynamic";

export async function GET() {
  return NextResponse.json(
    { ok: false, code: "READ_ONLY", error: "Spot probe moved to read-only — direct order placement disabled" },
    { status: 410 },
  );
}

/** Direct Delta spot order placement is retired — use POST /api/execution/request */
export async function POST() {
  return blockedDirectExecutionRoute("/api/delta/spot");
}
