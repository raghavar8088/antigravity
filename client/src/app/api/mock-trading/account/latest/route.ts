import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { getLatestMockAccountSnapshot } from "@/lib/trading/mockTradingMongo";
import { DEFAULT_MOCK_ACCOUNT_KEY, mockAccountKeySchema } from "@/lib/trading/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const url = new URL(req.url);
  const accountKey = mockAccountKeySchema.safeParse(url.searchParams.get("account_key") ?? DEFAULT_MOCK_ACCOUNT_KEY);
  if (!accountKey.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Invalid account_key", details: accountKey.error.flatten() },
      { status: 400 },
    );
  }

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" }, { status: 503 });
  }

  try {
    const latest = await getLatestMockAccountSnapshot(accountKey.data);
    return NextResponse.json({
      ok: true,
      snapshot: latest.account,
      config: latest.config,
      source: "mongo",
      storage: "mongo",
    });
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
