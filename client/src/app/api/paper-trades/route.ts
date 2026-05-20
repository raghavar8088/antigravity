import { NextResponse } from "next/server";
import { dbRowToBtcFuturesTrade, clientPayloadToInsertRow } from "@/lib/paperTradesMapper";
import {
  paperTradeGetQuerySchema,
  paperTradePostBodySchema,
  type PaperTradeDbRow,
} from "@/lib/paperTradesTypes";
import { isMongoConfigured, listTradesMongo, upsertTradeMongo } from "@/lib/mongoTradesClient";

export const dynamic = "force-dynamic";

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

  // No authentication: accept the provided accountKey verbatim.
  const resolvedAccountKey = parsed.data.accountKey;
  if (!resolvedAccountKey) {
    return NextResponse.json({ ok: false, error: "accountKey is required" }, { status: 400 });
  }

  const row = clientPayloadToInsertRow(
    resolvedAccountKey,
    parsed.data.trade,
    parsed.data.moduleKey,
    parsed.data.templateFamily,
    parsed.data.exchange,
  );

  // PRIMARY: MongoDB only. Fail if Mongo isn't configured or write fails.
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "No MongoDB configured" }, { status: 503 });
  }

  try {
    await upsertTradeMongo(row);
  } catch (err) {
    console.error("[paper-trades] mongo upsert failed", err);
    return NextResponse.json(
      { ok: false, error: "Mongo write failed", detail: err instanceof Error ? err.message : "unknown" },
      { status: 500 },
    );
  }

  return NextResponse.json({
    ok: true,
    idempotent: true,
    accountKey: resolvedAccountKey,
    clientTradeId: row.client_trade_id,
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
      { ok: false, error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  const account_key: string = parsed.data.account_key as string;
  const { limit, cursor, module_key: moduleKey } = parsed.data;

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "No MongoDB configured" }, { status: 503 });
  }

  try {
    const rows = await listTradesMongo({ accountKey: account_key, limit, cursor, moduleKey });
    const trades = rows.map(dbRowToBtcFuturesTrade);
    const last = rows[rows.length - 1];
    const nextCursor = rows.length === limit && last ? last.closed_at : null;
    return NextResponse.json({ ok: true, accountKey: account_key, trades, nextCursor, source: "mongo" });
  } catch (err) {
    console.error("[paper-trades] mongo read failed", err);
    return NextResponse.json({ ok: false, error: "Mongo read failed", detail: err instanceof Error ? err.message : "unknown" }, { status: 500 });
  }
}
