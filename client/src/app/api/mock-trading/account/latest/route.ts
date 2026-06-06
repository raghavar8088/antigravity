import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getLatestMockAccountSnapshot } from "@/lib/mockTradingMongo";
import { OWNER_ACCOUNT_KEY } from "@/lib/ownerAuth";

export const dynamic = "force-dynamic";

export async function GET() {
  const accountKey = OWNER_ACCOUNT_KEY;

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" }, { status: 503 });
  }

  try {
    const latest = await getLatestMockAccountSnapshot(accountKey);
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
