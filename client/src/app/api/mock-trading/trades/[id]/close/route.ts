import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { closeMockTradeInMongo, upsertMockTrade } from "@/lib/mockTradingMongo";
import { mockTradeCloseBodySchema } from "@/lib/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";

export async function POST(
  req: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, code: "INVALID_JSON", error: "Invalid JSON" }, { status: 400 });
  }

  const parsed = mockTradeCloseBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Validation failed", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" }, { status: 503 });
  }

  try {
    if (parsed.data.trade) {
      if (parsed.data.trade.id !== id) {
        return NextResponse.json(
          { ok: false, code: "TRADE_ID_MISMATCH", error: "Route id must match trade.id" },
          { status: 400 },
        );
      }
      if (parsed.data.trade.status !== "CLOSED") {
        return NextResponse.json(
          { ok: false, code: "TRADE_NOT_CLOSED", error: "close endpoint requires a CLOSED trade payload" },
          { status: 400 },
        );
      }
      const result = await upsertMockTrade(
        parsed.data.accountKey,
        parsed.data.trade,
        parsed.data.config,
        "MOCK_TRADE_CLOSED",
      );
      return NextResponse.json({ ok: true, storage: "mongo", trade: parsed.data.trade, ...result });
    }

    if (parsed.data.price == null) {
      return NextResponse.json(
        { ok: false, code: "PRICE_REQUIRED", error: "price is required when no closed trade payload is supplied" },
        { status: 400 },
      );
    }
    const trade = await closeMockTradeInMongo({
      accountKey: parsed.data.accountKey,
      tradeId: id,
      price: parsed.data.price,
      closedAt: parsed.data.closedAt ?? Date.now(),
      config: parsed.data.config,
    });
    if (!trade) {
      return NextResponse.json({ ok: false, code: "NOT_FOUND", error: "Mock trade not found" }, { status: 404 });
    }
    return NextResponse.json({ ok: true, storage: "mongo", trade });
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
