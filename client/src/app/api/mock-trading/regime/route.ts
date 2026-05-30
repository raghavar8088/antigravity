import { NextResponse } from "next/server";
import { z } from "zod";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getLatestRegimeSnapshot, upsertRegimeSnapshot } from "@/lib/mockTradingMongo";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";

const regimeSchema = z.enum(["TRENDING", "RANGING", "HIGH_VOLATILITY_BREAKOUT", "LOW_VOLATILITY_CHOP"]);

const querySchema = z.object({
  account_key: z.string().min(1).default(DEFAULT_MOCK_ACCOUNT_KEY),
});

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
  accountKey: z.string().min(1).default(DEFAULT_MOCK_ACCOUNT_KEY),
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

export async function GET(req: Request) {
  const url = new URL(req.url);
  const parsed = querySchema.safeParse({
    account_key: url.searchParams.get("account_key") ?? undefined,
  });
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  try {
    const snapshot = await getLatestRegimeSnapshot(parsed.data.account_key);
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
    const snapshot = await upsertRegimeSnapshot(parsed.data.accountKey, parsed.data.snapshot);
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
