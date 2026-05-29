import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getMockTrade, upsertMockTrade } from "@/lib/mockTradingMongo";
import {
  DEFAULT_MOCK_ACCOUNT_KEY,
  mockAccountKeySchema,
  mockTradePatchBodySchema,
} from "@/lib/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";

function mongoNotConfigured() {
  return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" }, { status: 503 });
}

export async function GET(
  req: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  const url = new URL(req.url);
  const account = mockAccountKeySchema.safeParse(url.searchParams.get("account_key") ?? DEFAULT_MOCK_ACCOUNT_KEY);
  if (!account.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Invalid account_key", details: account.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  const trade = await getMockTrade(account.data, id);
  if (!trade) {
    return NextResponse.json({ ok: false, code: "NOT_FOUND", error: "Mock trade not found" }, { status: 404 });
  }
  return NextResponse.json({ ok: true, trade, source: "mongo", storage: "mongo" });
}

export async function PATCH(
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

  const parsed = mockTradePatchBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Validation failed", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  if (parsed.data.trade.id !== id) {
    return NextResponse.json(
      { ok: false, code: "TRADE_ID_MISMATCH", error: "Route id must match trade.id" },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  try {
    const event = parsed.data.trade.status === "CLOSED" ? "MOCK_TRADE_CLOSED" : "MOCK_TRADE_UPDATED";
    const result = await upsertMockTrade(parsed.data.accountKey, parsed.data.trade, parsed.data.config, event);
    return NextResponse.json({ ok: true, storage: "mongo", tradeId: id, ...result });
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
