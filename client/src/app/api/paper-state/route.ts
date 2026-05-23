import { NextResponse } from "next/server";
import { isMongoConfigured, getAccountState, upsertAccountState } from "@/lib/mongoTradesClient";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key")?.trim();
  if (!accountKey) {
    return NextResponse.json({ ok: false, error: "account_key required" }, { status: 400 });
  }
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }
  const state = await getAccountState(accountKey);
  return NextResponse.json({ ok: true, state });
}

export async function POST(req: Request) {
  let body: unknown;
  try { body = await req.json(); } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON" }, { status: 400 });
  }
  const b = body as Record<string, unknown>;
  const accountKey = typeof b.accountKey === "string" ? b.accountKey.trim() : null;
  if (!accountKey) {
    return NextResponse.json({ ok: false, error: "accountKey required" }, { status: 400 });
  }
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }
  await upsertAccountState({
    account_key: accountKey,
    balance: typeof b.balance === "number" ? b.balance : 1000,
    positions: Array.isArray(b.positions) ? b.positions : [],
    pause_entries: b.pauseEntries === true,
    disabled_strategies: Array.isArray(b.disabledStrategies) ? (b.disabledStrategies as number[]) : [],
    last_trade_at: typeof b.lastTradeAt === "number" ? b.lastTradeAt : 0,
    day_start_balance: typeof b.dayStartBalance === "number" ? b.dayStartBalance : 1000,
    day_start_date: typeof b.dayStartDate === "number" ? b.dayStartDate : 0,
    cleared_at: typeof b.clearedAt === "number" ? b.clearedAt : 0,
    updated_at: new Date().toISOString(),
  });
  return NextResponse.json({ ok: true });
}
