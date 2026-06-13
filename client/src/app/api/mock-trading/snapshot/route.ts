import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { buildMockTradingSnapshot } from "@/lib/trading/mockTradingSnapshotService";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

export async function GET() {
  const accountKey = OWNER_ACCOUNT_KEY;

  if (!isMongoConfigured()) {
    return NextResponse.json(
      { ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" },
      { status: 503 },
    );
  }

  try {
    const snapshot = await buildMockTradingSnapshot(accountKey);
    return NextResponse.json({
      ok: true,
      account_key: accountKey,
      execution_authority: "mock-trading",
      ...snapshot,
    });
  } catch (err) {
    return NextResponse.json(
      {
        ok: false,
        code: "MONGO_READ_FAILED",
        error: "Mock trading snapshot failed",
        detail: err instanceof Error ? err.message : "unknown",
      },
      { status: 500 },
    );
  }
}
