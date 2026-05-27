import { randomUUID } from "crypto";
import { NextResponse } from "next/server";
import { isMongoConfigured, upsertTradeMongo } from "@/lib/mongoTradesClient";

export const dynamic = "force-dynamic";

export function paperDeskSmokeTestEnabled(): boolean {
  return process.env.NEXT_PUBLIC_DESK_SMOKE_TEST === "1";
}

export function buildPaperDeskSmokeTrade(accountKey: string, nowIso = new Date().toISOString()) {
  const tradeId = randomUUID();
  return {
    client_trade_id: tradeId,
    account_key: accountKey,
    module_key: "btc_future_trading",
    symbol: "BTCUSD",
    strategy_id: 0,
    strategy_name: "PAPER_EXECUTION_SMOKE_TEST",
    side: "LONG",
    entry_price: 100_000,
    exit_price: 100_000,
    contracts: 1,
    notional: 50,
    margin_used: 2,
    gross_pnl: 0,
    fees: 0,
    funding_costs: 0,
    net_pnl: 0,
    exit_reason: "TIME",
    opened_at: nowIso,
    closed_at: nowIso,
    payload: {
      diagnostic: true,
      excludedFromMetrics: true,
      message: "Paper execution plumbing smoke test",
    },
    exchange: "delta",
  };
}

export async function POST(req: Request) {
  if (!paperDeskSmokeTestEnabled()) {
    return NextResponse.json(
      { ok: false, error: "Smoke test disabled. Set NEXT_PUBLIC_DESK_SMOKE_TEST=1 to enable." },
      { status: 403 },
    );
  }
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  let body: unknown = {};
  try {
    body = await req.json();
  } catch {
    // Empty body is allowed when DESK_WORKER_ACCOUNT_KEY is set.
  }
  const b = body as Record<string, unknown>;
  const accountKey =
    (typeof b.accountKey === "string" ? b.accountKey.trim() : "") ||
    process.env.DESK_WORKER_ACCOUNT_KEY?.trim() ||
    "";
  if (!accountKey) {
    return NextResponse.json({ ok: false, error: "accountKey required" }, { status: 400 });
  }

  const trade = buildPaperDeskSmokeTrade(accountKey);
  const result = await upsertTradeMongo(trade);
  if (!result.ok) {
    return NextResponse.json({ ok: false, error: result.error }, { status: 500 });
  }

  return NextResponse.json({
    ok: true,
    opened: true,
    closed: true,
    tradeId: trade.client_trade_id,
    message: "Paper execution plumbing works; strategy gates may still block real entries.",
  });
}
