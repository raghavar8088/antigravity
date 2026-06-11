import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";

export const dynamic = "force-dynamic";

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  const mongoOk = isMongoConfigured();
  const report = {
    overall: mongoOk ? "PASS" : "WARNING",
    execution_authority: "mock-trading",
    checks: [
      {
        subsystem: "mock_trading",
        status: mongoOk ? "PASS" : "WARNING",
        detail: mongoOk ? "Mongo configured for mock trading persistence" : "Mongo not configured",
      },
      {
        subsystem: "paper_desk",
        status: "PASS",
        detail: "Retired — all execution routed through mock trading",
      },
    ],
    server_time: new Date().toISOString(),
  };

  return NextResponse.json(report, { status: mongoOk ? 200 : 206 });
}
