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

export const dynamic = "force-dynamic";

export async function POST(req: Request) {
  const auth = await getAuthenticatedPaperApiUser();
  if (!auth.ok) return auth.response;

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

  const match = assertCloudAccountMatchesSession(auth.ctx.userId, parsed.data.accountKey);
  if (!match.ok) {
    return NextResponse.json({ ok: false, error: match.error }, { status: match.status });
  }

  const supabase = createServiceSupabase();
  if (!supabase) {
    return NextResponse.json({ ok: false, error: "Supabase client unavailable" }, { status: 503 });
  }

  const row = clientPayloadToInsertRow(match.userId, parsed.data.trade);

  const { error } = await supabase.from("paper_trades").upsert(row, {
    onConflict: "client_trade_id",
    ignoreDuplicates: true,
  });

  if (error) {
    console.error("[paper-trades] insert", error);
    return NextResponse.json({ ok: false, error: error.message }, { status: 500 });
  }

  return NextResponse.json({
    ok: true,
    idempotent: true,
    accountKey: match.userId,
    clientTradeId: row.client_trade_id,
  });
}

export async function GET(req: Request) {
  const auth = await getAuthenticatedPaperApiUser();
  if (!auth.ok) return auth.response;

  const url = new URL(req.url);
  const parsed = paperTradeGetQuerySchema.safeParse({
    account_key: url.searchParams.get("account_key") ?? undefined,
    limit: url.searchParams.get("limit") ?? undefined,
    cursor: url.searchParams.get("cursor") ?? undefined,
  });

  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  const match = assertCloudAccountMatchesSession(auth.ctx.userId, parsed.data.account_key);
  if (!match.ok) {
    return NextResponse.json({ ok: false, error: match.error }, { status: match.status });
  }

  const supabase = createServiceSupabase();
  if (!supabase) {
    return NextResponse.json({ ok: false, error: "Supabase client unavailable" }, { status: 503 });
  }

  const { limit, cursor } = parsed.data;
  const account_key = match.userId;

  let query = supabase
    .from("paper_trades")
    .select("*")
    .eq("account_key", account_key)
    .order("closed_at", { ascending: false })
    .limit(limit);

  if (cursor) {
    query = query.lt("closed_at", cursor);
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
  });
}
