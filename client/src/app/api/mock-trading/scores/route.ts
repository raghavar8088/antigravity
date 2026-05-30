import { NextResponse } from "next/server";
import { z } from "zod";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { batchUpsertStrategyScores, listStrategyScoreHistory, listStrategyScores } from "@/lib/mockTradingMongo";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/mockTradingPersistenceTypes";
import type { StrategyScore } from "@/lib/strategyScoringEngine";

export const dynamic = "force-dynamic";

const querySchema = z.object({
  account_key: z.string().min(1).default(DEFAULT_MOCK_ACCOUNT_KEY),
  history: z.coerce.boolean().default(false),
  limit: z.coerce.number().int().min(1).max(20_000).default(1_000),
});

const scoreSchema = z.object({
  strategyId: z.number().int(),
  strategyName: z.string(),
  metrics: z.record(z.string(), z.unknown()),
  pnlScore: z.number(),
  profitFactorScore: z.number(),
  winRateScore: z.number(),
  drawdownScore: z.number(),
  sharpeScore: z.number(),
  recencyScore: z.number(),
  sampleSizeScore: z.number(),
  overallScore: z.number(),
  currentRegimeScore: z.number(),
  confidenceRating: z.enum(["HIGH", "MEDIUM", "LOW", "INSUFFICIENT"]),
  rank: z.number().int(),
  regimeRank: z.number().int(),
});

const writeSchema = z.object({
  accountKey: z.string().min(1).default(DEFAULT_MOCK_ACCOUNT_KEY),
  scores: z.array(scoreSchema).max(2_000),
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
    history: url.searchParams.get("history") ?? undefined,
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
    const scores = parsed.data.history
      ? await listStrategyScoreHistory(parsed.data.account_key, parsed.data.limit)
      : await listStrategyScores(parsed.data.account_key, parsed.data.limit);
    return NextResponse.json({ ok: true, storage: "mongo", scores });
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
    const updatedCount = await batchUpsertStrategyScores(parsed.data.accountKey, parsed.data.scores as unknown as StrategyScore[]);
    return NextResponse.json({ ok: true, storage: "mongo", updatedCount });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "MONGO_WRITE_FAILED", error: "Mongo write failed", detail: err instanceof Error ? err.message : "unknown" },
      { status: 500 },
    );
  }
}
