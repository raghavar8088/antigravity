import { NextResponse } from "next/server";
import { assertCloudAccountMatchesSession } from "@/lib/paperTradesAuth";
import { getAuthenticatedPaperApiUser } from "@/lib/paperTradesApiAuth";
import { dbRowToBtcFuturesTrade, clientPayloadToInsertRow } from "@/lib/paperTradesMapper";
import {
  paperTradeGetQuerySchema,
  paperTradePostBodySchema,
  type PaperTradeDbRow,
} from "@/lib/paperTradesTypes";
import { createServiceSupabase } from "@/lib/supabase/server";
import {
  isMongoConfigured,
  listTradesMongo,
  upsertTradeMongo,
} from "@/lib/mongoTradesClient";
import { isAnonAccountKey } from "@/lib/anonAccountKey";

export const dynamic = "force-dynamic";

const ANON_ENABLED = process.env.ALLOW_ANON_PAPER_TRADES === "1";

export async function POST(req: Request) {
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON" }, { status: 400 });
  }

  const parsed = paperTradePostBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, error: "Validation failed", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  // Anonymous path: when feature flag is on AND the caller supplied an anon-shaped
  // accountKey, skip Supabase auth entirely and use the supplied key verbatim.
  // Security trade-off documented in anonAccountKey.ts.
  let resolvedAccountKey: string;
  if (ANON_ENABLED && isAnonAccountKey(parsed.data.accountKey)) {
    resolvedAccountKey = parsed.data.accountKey!;
  } else {
    const auth = await getAuthenticatedPaperApiUser();
    if (!auth.ok) return auth.response;
    const match = assertCloudAccountMatchesSession(auth.ctx.userId, parsed.data.accountKey);
    if (!match.ok) {
      return NextResponse.json({ ok: false, error: match.error }, { status: match.status });
    }
    resolvedAccountKey = match.userId;
  }

  const row = clientPayloadToInsertRow(
    resolvedAccountKey,
    parsed.data.trade,
    parsed.data.moduleKey,
  );

  // PRIMARY: MongoDB. When configured, a Mongo write failure aborts the request
  // so the engine retries — we don't want silent data loss on the source of truth.
  // When MONGODB_URI isn't set we transparently fall back to Supabase-only writes
  // (legacy mode), so existing deployments keep working until env is provisioned.
  let mongoWritten = false;
  if (isMongoConfigured()) {
    try {
      await upsertTradeMongo(row);
      mongoWritten = true;
    } catch (err) {
      console.error("[paper-trades] mongo upsert failed", err);
      return NextResponse.json(
        { ok: false, error: "Mongo write failed", detail: err instanceof Error ? err.message : "unknown" },
        { status: 500 },
      );
    }
  }

  // MIRROR: Supabase. Best-effort — failures are logged but do NOT fail the
  // request when Mongo already accepted the write. Keeps the legacy reader path
  // working as a safety net.
  let supabaseMirrored = false;
  const supabase = createServiceSupabase();
  if (supabase) {
    const { error } = await supabase.from("paper_trades").upsert(row, {
      onConflict: "client_trade_id",
      ignoreDuplicates: true,
    });
    if (error) {
      // If Mongo also failed (or isn't configured) we have no successful write — return 500.
      if (!mongoWritten) {
        console.error("[paper-trades] insert", error);
        return NextResponse.json({ ok: false, error: error.message }, { status: 500 });
      }
      console.warn("[paper-trades] supabase mirror write failed (mongo OK)", error.message);
    } else {
      supabaseMirrored = true;
    }
  } else if (!mongoWritten) {
    return NextResponse.json({ ok: false, error: "No trade store configured" }, { status: 503 });
  }

  return NextResponse.json({
    ok: true,
    idempotent: true,
    accountKey: resolvedAccountKey,
    clientTradeId: row.client_trade_id,
    persistedTo: { mongo: mongoWritten, supabase: supabaseMirrored },
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
      { ok: false, error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  // Same anon path as POST: when flag is on and key is anon-shaped, no auth needed.
  let account_key: string;
  if (ANON_ENABLED && isAnonAccountKey(parsed.data.account_key)) {
    account_key = parsed.data.account_key!;
  } else {
    const auth = await getAuthenticatedPaperApiUser();
    if (!auth.ok) return auth.response;
    const match = assertCloudAccountMatchesSession(auth.ctx.userId, parsed.data.account_key);
    if (!match.ok) {
      return NextResponse.json({ ok: false, error: match.error }, { status: match.status });
    }
    account_key = match.userId;
  }

  const { limit, cursor, module_key: moduleKey } = parsed.data;

  // PRIMARY: read from MongoDB. Fall back to Supabase if Mongo isn't configured
  // OR the Mongo query throws (transient connectivity blip — the mirror still has data).
  if (isMongoConfigured()) {
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
      });
    } catch (err) {
      console.warn("[paper-trades] mongo read failed, falling back to supabase", err);
    }
  }

  const supabase = createServiceSupabase();
  if (!supabase) {
    return NextResponse.json({ ok: false, error: "No trade store configured" }, { status: 503 });
  }

  let query = supabase
    .from("paper_trades")
    .select("*")
    .eq("account_key", account_key)
    .order("closed_at", { ascending: false })
    .limit(limit);

  if (cursor) {
    query = query.lt("closed_at", cursor);
  }
  if (moduleKey) {
    query = query.eq("module_key", moduleKey);
  }

  const { data, error } = await query;

  if (error) {
    console.error("[paper-trades] list", error);
    return NextResponse.json({ ok: false, error: error.message }, { status: 500 });
  }

  const rows = (data ?? []) as PaperTradeDbRow[];
  const trades = rows.map(dbRowToBtcFuturesTrade);
  const last = rows[rows.length - 1];
  const nextCursor = rows.length === limit && last ? last.closed_at : null;

  return NextResponse.json({
    ok: true,
    accountKey: account_key,
    trades,
    nextCursor,
    source: "supabase",
  });
}
