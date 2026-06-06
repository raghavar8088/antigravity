import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getMockAnalyticsSummary, getLatestMockAccountSnapshot } from "@/lib/mockTradingMongo";
import { mockTradingConfigSchema } from "@/lib/mockTradingPersistenceTypes";
import { DEFAULT_MOCK_TRADING_CONFIG } from "@/lib/mockTradingEngine";
import { OWNER_ACCOUNT_KEY } from "@/lib/ownerAuth";

export const dynamic = "force-dynamic";

export async function GET() {
  const accountKey = OWNER_ACCOUNT_KEY;

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" }, { status: 503 });
  }

  try {
    const latest = await getLatestMockAccountSnapshot(accountKey);
    const config = mockTradingConfigSchema.safeParse(latest.config).success
      ? latest.config ?? DEFAULT_MOCK_TRADING_CONFIG
      : DEFAULT_MOCK_TRADING_CONFIG;
    const analytics = await getMockAnalyticsSummary(accountKey, config);
    return NextResponse.json({ ok: true, analytics, source: "mongo", storage: "mongo" });
  } catch (err) {
    return NextResponse.json(
      {
        ok: false,
        code: "MONGO_READ_FAILED",
        error: "Mongo read failed",
        detail: err instanceof Error ? err.message : "unknown",
      },
      { status: 500 },
    );
  }
}
