import { NextResponse } from "next/server";
import { z } from "zod";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { listDailyPnlHistory, upsertDailyPnlPoint } from "@/lib/mockTradingMongo";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";

export const dynamic = "force-dynamic";

const querySchema = z.object({
  limit: z.coerce.number().int().min(1).max(5_000).default(500),
});

const writeSchema = z.object({
  point: z.object({
    day: z.number().int(),
    pnl: z.number(),
    tradeCount: z.number().int().min(0),
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
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  const url = new URL(req.url);
  const parsed = querySchema.safeParse({
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
    const points = await listDailyPnlHistory(accountKey, parsed.data.limit);
    return NextResponse.json({ ok: true, storage: "mongo", points });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "MONGO_READ_FAILED", error: "Mongo read failed", detail: err instanceof Error ? err.message : "unknown" },
      { status: 500 },
    );
  }
}

export async function POST(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

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
    const point = await upsertDailyPnlPoint(accountKey, parsed.data.point);
    return NextResponse.json({ ok: true, storage: "mongo", point });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "MONGO_WRITE_FAILED", error: "Mongo write failed", detail: err instanceof Error ? err.message : "unknown" },
      { status: 500 },
    );
  }
}
