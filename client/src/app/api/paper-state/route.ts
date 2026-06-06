import { NextResponse } from "next/server";
import { isMongoConfigured, getAccountState, upsertAccountState } from "@/lib/mongoTradesClient";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";

export const dynamic = "force-dynamic";

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }
  const state = await getAccountState(accountKey);
  return NextResponse.json({ ok: true, state });
}

export async function POST(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  let body: unknown;
  try { body = await req.json(); } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON" }, { status: 400 });
  }
  const b = body as Record<string, unknown>;

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
    worker_id: typeof b.workerId === "string" ? b.workerId : null,
    worker_last_poll_at: typeof b.workerLastPollAt === "number" ? b.workerLastPollAt : null,
    worker_owner: b.workerOwner === "vps" || b.workerOwner === "browser" ? b.workerOwner : null,
  });
  return NextResponse.json({ ok: true });
}
