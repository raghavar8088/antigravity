import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getMockAnalyticsSummary, getLatestMockAccountSnapshot } from "@/lib/mockTradingMongo";
import {
  DEFAULT_MOCK_ACCOUNT_KEY,
  mockAccountKeySchema,
  mockTradingConfigSchema,
} from "@/lib/mockTradingPersistenceTypes";
import { DEFAULT_MOCK_TRADING_CONFIG } from "@/lib/mockTradingEngine";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const url = new URL(req.url);
  const account = mockAccountKeySchema.safeParse(url.searchParams.get("account_key") ?? DEFAULT_MOCK_ACCOUNT_KEY);
  if (!account.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Invalid account_key", details: account.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" }, { status: 503 });
  }

  try {
    const latest = await getLatestMockAccountSnapshot(account.data);
    const config = mockTradingConfigSchema.safeParse(latest.config).success
      ? latest.config ?? DEFAULT_MOCK_TRADING_CONFIG
      : DEFAULT_MOCK_TRADING_CONFIG;
    const analytics = await getMockAnalyticsSummary(account.data, config);
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
