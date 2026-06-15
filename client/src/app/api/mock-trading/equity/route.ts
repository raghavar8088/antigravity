import { NextResponse } from "next/server";
import { z } from "zod";
import { isMongoConfigured, getDb } from "@/lib/broker/mongoTradesClient";
import { appendEquityCurvePoint, listEquityCurvePoints, MOCK_EQUITY_CURVE_COLLECTION } from "@/lib/trading/mockTradingMongo";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

const EQUITY_RING_BUFFER_MAX = 4800;
const EQUITY_RING_BUFFER_TRIM = 200;

const querySchema = z.object({
  limit: z.coerce.number().int().min(1).max(5_000).default(1_500),
  resolution: z.enum(["raw", "15m", "1h", "1d"]).default("raw"),
});

const writeSchema = z.object({
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

/** PERF 3: LTTB (Largest-Triangle-Three-Buckets) downsampling. */
function lttb(data: { x: number; y: number }[], threshold: number): { x: number; y: number }[] {
  if (data.length <= threshold || threshold <= 2) return data;
  const result: { x: number; y: number }[] = [];
  const bucketSize = (data.length - 2) / (threshold - 2);
  result.push(data[0]);
  let a = 0;
  for (let i = 0; i < threshold - 2; i++) {
    const bucketStart = Math.floor((i + 1) * bucketSize) + 1;
    const bucketEnd = Math.min(Math.floor((i + 2) * bucketSize) + 1, data.length);
    let avgX = 0, avgY = 0;
    const cnt = bucketEnd - bucketStart;
    for (let j = bucketStart; j < bucketEnd; j++) { avgX += data[j].x; avgY += data[j].y; }
    avgX /= cnt; avgY /= cnt;
    const rangeStart = Math.floor(i * bucketSize) + 1;
    const rangeEnd = Math.min(Math.floor((i + 1) * bucketSize) + 1, data.length);
    let maxArea = -1, maxAreaIdx = rangeStart;
    const pa = data[a];
    for (let j = rangeStart; j < rangeEnd; j++) {
      const area = Math.abs((pa.x - avgX) * (data[j].y - pa.y) - (pa.x - data[j].x) * (avgY - pa.y)) * 0.5;
      if (area > maxArea) { maxArea = area; maxAreaIdx = j; }
    }
    result.push(data[maxAreaIdx]);
    a = maxAreaIdx;
  }
  result.push(data[data.length - 1]);
  return result;
}

export async function GET(req: Request) {
  const accountKey = OWNER_ACCOUNT_KEY;

  const url = new URL(req.url);
  const parsed = querySchema.safeParse({
    limit: url.searchParams.get("limit") ?? undefined,
    resolution: url.searchParams.get("resolution") ?? undefined,
  });
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  try {
    const rawPoints = await listEquityCurvePoints(accountKey, parsed.data.limit);
    // PERF 3: downsample based on resolution.
    const { resolution } = parsed.data;
    let threshold: number | null = null;
    if (resolution === "1h") threshold = 500;
    else if (resolution === "1d") threshold = 100;

    let points = rawPoints;
    if (threshold !== null) {
      const xy = rawPoints.map((p) => ({ x: p.timestamp, y: p.equity }));
      const downsampled = lttb(xy, threshold);
      const dsSet = new Set(downsampled.map((d) => d.x));
      points = rawPoints.filter((p) => dsSet.has(p.timestamp));
    }
    return NextResponse.json({ ok: true, storage: "mongo", points, resolution });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "MONGO_READ_FAILED", error: "Mongo read failed", detail: err instanceof Error ? err.message : "unknown" },
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

  const parsed = writeSchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Validation failed", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  try {
    // BUG 6: Equity ring buffer — trim oldest 200 docs when count >= 4800.
    const db = await getDb();
    const equityCol = db.collection(MOCK_EQUITY_CURVE_COLLECTION);
    const count = await equityCol.countDocuments({ account_key: accountKey });
    if (count >= EQUITY_RING_BUFFER_MAX) {
      const oldest = await equityCol
        .find({ account_key: accountKey })
        .sort({ timestamp: 1 })
        .limit(EQUITY_RING_BUFFER_TRIM)
        .project({ timestamp: 1 })
        .toArray();
      const timestamps = oldest.map((d: { timestamp?: number }) => d.timestamp).filter(Boolean);
      if (timestamps.length > 0) {
        await equityCol.deleteMany({ account_key: accountKey, timestamp: { $in: timestamps } });
      }
    }
    const point = await appendEquityCurvePoint(accountKey, parsed.data.point);
    return NextResponse.json({ ok: true, storage: "mongo", point });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "MONGO_WRITE_FAILED", error: "Mongo write failed", detail: err instanceof Error ? err.message : "unknown" },
      { status: 500 },
    );
  }
}
