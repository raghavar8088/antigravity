import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { listMockTrades, upsertMockTrade } from "@/lib/mockTradingMongo";
import { mockTradeListQuerySchema, mockTradeWriteBodySchema } from "@/lib/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";

function mongoNotConfigured() {
  return NextResponse.json(
    {
      ok: false,
      code: "MONGO_NOT_CONFIGURED",
      error: "MongoDB not configured",
      hint: "Set MONGODB_URI and MONGODB_DB_NAME in the server environment",
    },
    { status: 503 },
  );
}

export async function GET(req: Request) {
  const url = new URL(req.url);
  const parsed = mockTradeListQuerySchema.safeParse({
    account_key: url.searchParams.get("account_key") ?? undefined,
    page: url.searchParams.get("page") ?? undefined,
    limit: url.searchParams.get("limit") ?? undefined,
    status: url.searchParams.get("status") ?? undefined,
    side: url.searchParams.get("side") ?? undefined,
    strategy_id: url.searchParams.get("strategy_id") ?? undefined,
    strategy_family: url.searchParams.get("strategy_family") ?? undefined,
    blocker_gate: url.searchParams.get("blocker_gate") ?? undefined,
    profitability: url.searchParams.get("profitability") ?? undefined,
    age_mode: url.searchParams.get("age_mode") ?? undefined,
    age_min_minutes: url.searchParams.get("age_min_minutes") ?? undefined,
    age_max_minutes: url.searchParams.get("age_max_minutes") ?? undefined,
    sort: url.searchParams.get("sort") ?? undefined,
  });
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  try {
    const result = await listMockTrades(parsed.data);
    return NextResponse.json({ ok: true, ...result, source: "mongo", storage: "mongo" });
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

export async function POST(req: Request) {
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, code: "INVALID_JSON", error: "Invalid JSON" }, { status: 400 });
  }

  const parsed = mockTradeWriteBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Validation failed", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  try {
    const result = await upsertMockTrade(
      parsed.data.accountKey,
      parsed.data.trade,
      parsed.data.config,
      "MOCK_TRADE_CREATED",
    );
    return NextResponse.json({
      ok: true,
      storage: "mongo",
      tradeId: parsed.data.trade.id,
      ...result,
    });
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
