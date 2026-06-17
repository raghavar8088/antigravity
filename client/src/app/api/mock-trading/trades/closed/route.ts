import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { deleteClosedMockTrades } from "@/lib/trading/mockTradingMongo";
import { mockClearClosedBodySchema } from "@/lib/trading/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";

function mongoNotConfigured() {
  return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" }, { status: 503 });
}

export async function DELETE(req: Request) {
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, code: "INVALID_JSON", error: "Invalid JSON" }, { status: 400 });
  }

  const parsed = mockClearClosedBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      {
        ok: false,
        code: "CONFIRMATION_REQUIRED",
        error: "Clear closed trades requires confirmation: CLEAR_CLOSED_TRADES",
        details: parsed.error.flatten(),
      },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  try {
    const result = await deleteClosedMockTrades(parsed.data.accountKey);
    return NextResponse.json({ ok: true, storage: "mongo", ...result });
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
