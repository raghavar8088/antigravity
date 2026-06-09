import { NextResponse } from "next/server";
import { blockedDirectExecutionRoute } from "@/lib/blockedExecutionRoute";

export const dynamic = "force-dynamic";

/** Direct Delta mirror execution is retired — use POST /api/execution/request */
export async function POST() {
  return blockedDirectExecutionRoute("/api/delta/mirror");
}
