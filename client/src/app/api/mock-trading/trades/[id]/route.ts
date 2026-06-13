import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { getMockTrade, upsertMockTrade } from "@/lib/trading/mockTradingMongo";
import { mockTradePatchBodySchema } from "@/lib/trading/mockTradingPersistenceTypes";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

function mongoNotConfigured() {
  return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" }, { status: 503 });
}

export async function GET(
  _req: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const accountKey = OWNER_ACCOUNT_KEY;

  const { id } = await params;
  if (!isMongoConfigured()) return mongoNotConfigured();

  const trade = await getMockTrade(accountKey, id);
  if (!trade) {
    return NextResponse.json({ ok: false, code: "NOT_FOUND", error: "Mock trade not found" }, { status: 404 });
  }
  return NextResponse.json({ ok: true, trade, source: "mongo", storage: "mongo" });
}

// PATCH is disabled — position mark-to-market is now owned by the Go engine.
// Positions are tracked in paper_positions and read via /api/paper-desk/positions.
export async function PATCH() {
  return NextResponse.json(
    { ok: false, code: "DEPRECATED", error: "Browser trade updates are disabled. The Go engine owns position mark-to-market. Read positions from /api/paper-desk/positions." },
    { status: 410 },
  );
}
