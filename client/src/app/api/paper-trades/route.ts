import { NextResponse } from "next/server";
import { cookies } from "next/headers";
import { dbRowToBtcFuturesTrade, clientPayloadToInsertRow } from "@/lib/paperTradesMapper";
import {
  paperTradeGetQuerySchema,
  paperTradePostBodySchema,
} from "@/lib/paperTradesTypes";
import { isMongoConfigured, listTradesMongo, upsertTradeMongo } from "@/lib/mongoTradesClient";
import { verifySession, SESSION_COOKIE } from "@/lib/jwtSession";

export const dynamic = "force-dynamic";

function anonAllowed(): boolean {
  // Accept either flag name for backwards-compat.
  return (
    process.env.ALLOW_PAPER_TRADES_ANON === "1" ||
    process.env.ALLOW_ANON_PAPER_TRADES === "1"
  );
}

async function resolveAccountKey(bodyAccountKey: string | undefined): Promise<
  | { ok: true; accountKey: string; userId: string | null }
  | { ok: false; status: number; code: string; error: string }
> {
  // Try session cookie first (silent — don't fail if AUTH_JWT_SECRET missing).
  try {
    const cookieStore = await cookies();
    const token = cookieStore.get(SESSION_COOKIE)?.value;
    if (token && process.env.AUTH_JWT_SECRET) {
      const session = verifySession(token);
      if (session?.userId) {
        return { ok: true, accountKey: session.userId, userId: session.userId };
      }
    }
  } catch {
    // fall through to anon path
  }

  // No session → require either anon flag enabled OR explicit body.accountKey rejected.
  if (!bodyAccountKey || bodyAccountKey.trim().length === 0) {
    return {
      ok: false,
      status: 401,
      code: "AUTH_REQUIRED",
      error: "Sign in or provide an accountKey (anon mode disabled)",
    };
  }
  if (!anonAllowed()) {
    return {
      ok: false,
      status: 401,
      code: "AUTH_REQUIRED",
      error: "Anonymous writes disabled. Sign in or set ALLOW_PAPER_TRADES_ANON=1.",
    };
  }
  return { ok: true, accountKey: bodyAccountKey.trim(), userId: null };
}

export async function POST(req: Request) {
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json(
      { ok: false, code: "INVALID_JSON", error: "Invalid JSON" },
      { status: 400 },
    );
  }

  const parsed = paperTradePostBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      {
        ok: false,
        code: "VALIDATION_FAILED",
        error: "Validation failed",
        details: parsed.error.flatten(),
      },
      { status: 400 },
    );
  }

  const auth = await resolveAccountKey(parsed.data.accountKey);
  if (!auth.ok) {
    return NextResponse.json(
      { ok: false, code: auth.code, error: auth.error },
      { status: auth.status },
    );
  }

  const accountKey = auth.accountKey;
  const tradeId = parsed.data.trade.clientTradeId;
  const moduleKey = parsed.data.moduleKey;
  // Use the client-supplied collection (created on clear) or the default.
  const collectionName = parsed.data.collectionName ?? "paper_trades";

  if (!isMongoConfigured()) {
    console.warn("[paper-trades POST] MONGO_NOT_CONFIGURED", { tradeId, accountKey, moduleKey });
    return NextResponse.json(
      {
        ok: false,
        code: "MONGO_NOT_CONFIGURED",
        error: "MongoDB not configured",
        hint: "Set MONGODB_URI in server env",
      },
      { status: 503 },
    );
  }

  const row = clientPayloadToInsertRow(
    accountKey,
    parsed.data.trade,
    parsed.data.moduleKey,
    parsed.data.templateFamily,
    parsed.data.exchange,
  );

  console.log("[paper-trades POST] writing", {
    mongoConfigured: true,
    userId: auth.userId,
    tradeId,
    accountKey,
    moduleKey: row.module_key,
    collectionName,
  });

  const mongoResult = await upsertTradeMongo(row, collectionName);
  if (!mongoResult.ok) {
    console.error("[paper-trades POST] MONGO_WRITE_FAILED", { tradeId, error: mongoResult.error });
    return NextResponse.json(
      {
        ok: false,
        code: "MONGO_WRITE_FAILED",
        error: "Mongo write failed",
        message: mongoResult.error,
      },
      { status: 500 },
    );
  }

  console.log("[paper-trades POST] mongoResult", {
    tradeId,
    upsertedCount: mongoResult.upsertedCount,
    modifiedCount: mongoResult.modifiedCount,
    matchedCount: mongoResult.matchedCount,
  });

  return NextResponse.json({
    ok: true,
    storage: "mongo",
    clientTradeId: tradeId,
    accountKey,
    upsertedCount: mongoResult.upsertedCount,
    modifiedCount: mongoResult.modifiedCount,
    // Legacy fields kept for any old callers; safe to remove later.
    idempotent: true,
    persistedTo: { mongo: true },
  });
}

export async function GET(req: Request) {
  const url = new URL(req.url);
  const parsed = paperTradeGetQuerySchema.safeParse({
    account_key: url.searchParams.get("account_key") ?? undefined,
    limit: url.searchParams.get("limit") ?? undefined,
    cursor: url.searchParams.get("cursor") ?? undefined,
    module_key: url.searchParams.get("module_key") ?? undefined,
  });

  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  const account_key = parsed.data.account_key;
  if (!account_key) {
    return NextResponse.json(
      { ok: false, code: "ACCOUNT_KEY_REQUIRED", error: "account_key is required" },
      { status: 400 },
    );
  }
  const { limit, cursor, module_key: moduleKey } = parsed.data;

  if (!isMongoConfigured()) {
    return NextResponse.json(
      { ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" },
      { status: 503 },
    );
  }

  try {
    const rows = await listTradesMongo({ accountKey: account_key, limit, cursor, moduleKey });
    const trades = rows.map(dbRowToBtcFuturesTrade);
    const last = rows[rows.length - 1];
    const nextCursor = rows.length === limit && last ? last.closed_at : null;
    return NextResponse.json({
      ok: true,
      accountKey: account_key,
      trades,
      nextCursor,
      source: "mongo",
      storage: "mongo",
    });
  } catch (err) {
    console.error("[paper-trades GET] mongo read failed", err);
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
