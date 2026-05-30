import { NextResponse } from "next/server";
import { z } from "zod";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { appendEquityCurvePoint, listEquityCurvePoints } from "@/lib/mockTradingMongo";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";

const querySchema = z.object({
  account_key: z.string().min(1).default(DEFAULT_MOCK_ACCOUNT_KEY),
  limit: z.coerce.number().int().min(1).max(5_000).default(1_500),
});

const writeSchema = z.object({
  accountKey: z.string().min(1).default(DEFAULT_MOCK_ACCOUNT_KEY),
  point: z.object({
    timestamp: z.number().int(),
    equity: z.number(),
    realizedPnl: z.number(),
    unrealizedPnl: z.number(),
    drawdownPct: z.number(),
    dailyPnl: z.number().optional(),
    regime: z.string().optional(),
  }),
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
  const url = new URL(req.url);
  const parsed = querySchema.safeParse({
    account_key: url.searchParams.get("account_key") ?? undefined,
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
    const points = await listEquityCurvePoints(parsed.data.account_key, parsed.data.limit);
    return NextResponse.json({ ok: true, storage: "mongo", points });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "MONGO_READ_FAILED", error: "Mongo read failed", detail: err instanceof Error ? err.message : "unknown" },
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

  const parsed = writeSchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Validation failed", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  try {
    const point = await appendEquityCurvePoint(parsed.data.accountKey, parsed.data.point);
    return NextResponse.json({ ok: true, storage: "mongo", point });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "MONGO_WRITE_FAILED", error: "Mongo write failed", detail: err instanceof Error ? err.message : "unknown" },
      { status: 500 },
    );
  }
}
