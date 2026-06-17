import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { getMockTrade, deleteClosedMockTrade } from "@/lib/trading/mockTradingMongo";
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

export async function DELETE(
  req: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  if (!id?.trim()) {
    return NextResponse.json({ ok: false, code: "VALIDATION_FAILED", error: "Trade id required" }, { status: 400 });
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  let accountKey = OWNER_ACCOUNT_KEY;
  try {
    const body = await req.json() as { accountKey?: string };
    if (body.accountKey?.trim()) accountKey = body.accountKey.trim();
  } catch {
    // accountKey optional — default to owner key
  }

  try {
    const result = await deleteClosedMockTrade(accountKey, id);
    if (!result.deleted) {
      return NextResponse.json(
        { ok: false, code: "NOT_FOUND", error: "Closed mock trade not found" },
        { status: 404 },
      );
    }
    return NextResponse.json({ ok: true, storage: "mongo", tradeId: id, deleted: true });
  } catch (err) {
    return NextResponse.json(
      {
        ok: false,
        code: "MONGO_WRITE_FAILED",
        error: "Mongo write failed",
        detail: err instanceof Error ? err.message : "unknown",
      },
      { status: 500 },
    );
  }
}
