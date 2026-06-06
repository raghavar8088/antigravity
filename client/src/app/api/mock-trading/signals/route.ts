import { NextResponse } from "next/server";
import { z } from "zod";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { insertStrategySignals, listRecentSignals } from "@/lib/mockTradingMongo";
import { OWNER_ACCOUNT_KEY } from "@/lib/ownerAuth";

export const dynamic = "force-dynamic";

const signalQuerySchema = z.object({
  strategy_id: z.coerce.number().int().optional(),
  family: z.string().optional(),
  regime: z.string().optional(),
  since: z.coerce.number().int().optional(),
  limit: z.coerce.number().int().min(1).max(1_000).default(200),
});

const signalDocSchema = z.object({
  strategy_id: z.number().int(),
  strategy_name: z.string(),
  strategy_family: z.string(),
  symbol: z.string().default("BTCUSD"),
  side: z.enum(["BUY", "SELL", "NO_SIGNAL"]),
  timeframe: z.string().default("1m"),
  entry_price: z.number(),
  confidence_score: z.number().min(0).max(100),
  market_regime: z.string().optional(),
  indicator_snapshot: z.record(z.string(), z.unknown()).default({}),
  take_profit: z.number().optional(),
  stop_loss: z.number().optional(),
  expected_profit_usd: z.number().optional(),
  expected_loss_usd: z.number().optional(),
  risk_reward: z.number().optional(),
  entry_reason: z.string().default("Research strategy signal"),
  timestamp: z.number().int(),
});

const signalWriteSchema = z.object({
  signals: z.array(signalDocSchema).max(1_000),
});

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
  const accountKey = OWNER_ACCOUNT_KEY;

  const url = new URL(req.url);
  const parsed = signalQuerySchema.safeParse({
    strategy_id: url.searchParams.get("strategy_id") ?? undefined,
    family: url.searchParams.get("family") ?? undefined,
    regime: url.searchParams.get("regime") ?? undefined,
    since: url.searchParams.get("since") ?? undefined,
    limit: url.searchParams.get("limit") ?? undefined,
  });
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  try {
    const signals = await listRecentSignals(accountKey, parsed.data);
    return NextResponse.json({ ok: true, storage: "mongo", signals });
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
  const accountKey = OWNER_ACCOUNT_KEY;

  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, code: "INVALID_JSON", error: "Invalid JSON" }, { status: 400 });
  }

  const parsed = signalWriteSchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Validation failed", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  try {
    const insertedCount = await insertStrategySignals(accountKey, parsed.data.signals);
    return NextResponse.json({ ok: true, storage: "mongo", insertedCount });
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
