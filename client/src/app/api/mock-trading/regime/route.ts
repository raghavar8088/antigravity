import { NextResponse } from "next/server";
import { z } from "zod";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getLatestRegimeSnapshot, upsertRegimeSnapshot } from "@/lib/mockTradingMongo";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";

export const dynamic = "force-dynamic";

const regimeSchema = z.enum(["TRENDING", "RANGING", "HIGH_VOLATILITY_BREAKOUT", "LOW_VOLATILITY_CHOP"]);

const snapshotSchema = z.object({
  regime: regimeSchema,
  confidence: z.number().min(0).max(100),
  adx: z.number(),
  atrPct: z.number(),
  bbWidthPct: z.number(),
  bbWidthPercentile: z.number().min(0).max(1),
  emaSlope: z.number(),
  realizedVol: z.number(),
  volRatio: z.number().default(1),
  roc5: z.number().default(0),
  timestamp: z.number().int(),
});

const writeSchema = z.object({
  snapshot: snapshotSchema,
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

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) return mongoNotConfigured();

  try {
    const snapshot = await getLatestRegimeSnapshot(accountKey);
    return NextResponse.json({ ok: true, storage: "mongo", snapshot });
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
    const snapshot = await upsertRegimeSnapshot(accountKey, parsed.data.snapshot);
    return NextResponse.json({ ok: true, storage: "mongo", snapshot });
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
