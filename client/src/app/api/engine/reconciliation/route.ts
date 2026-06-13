import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { getLatestMockAccountSnapshot } from "@/lib/trading/mockTradingMongo";
import { mongoUnconfigured } from "@/lib/trading/mockTradingApiErrors";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

type ReconEvent = {
  ts: string;
  drift_amount: number;
  action: string;
  kill_switch_triggered: boolean;
  domain?: string;
};

export async function GET() {
  const engineBase = process.env.INTERNAL_API_URL?.trim().replace(/\/$/, "") ?? "http://localhost:8080";
  const events: ReconEvent[] = [];
  let overall: "GREEN" | "AMBER" | "RED" = "GREEN";

  if (isMongoConfigured()) {
    try {
      const { account } = await getLatestMockAccountSnapshot(OWNER_ACCOUNT_KEY);
      if (account) {
        overall = "GREEN";
        events.push({
          ts: new Date().toISOString(),
          drift_amount: Math.abs(account.maxDrawdownPct),
          action: "MOCK_STATE_OK",
          kill_switch_triggered: false,
          domain: "mock_trading",
        });
      } else {
        overall = "AMBER";
        events.push({
          ts: new Date().toISOString(),
          drift_amount: 0,
          action: "NO_MOCK_ACCOUNT",
          kill_switch_triggered: false,
        });
      }
    } catch {
      overall = "RED";
    }
  } else {
    return mongoUnconfigured();
  }

  try {
    const res = await fetch(`${engineBase}/reconciliation/status`, {
      signal: AbortSignal.timeout(3_000),
      cache: "no-store",
    });
    if (res.ok) {
      const data = await res.json();
      events.push({
        ts: new Date().toISOString(),
        drift_amount: 0,
        action: String(data.status ?? "ENGINE_OK"),
        kill_switch_triggered: false,
        domain: "go_engine",
      });
    }
  } catch {
    events.push({
      ts: new Date().toISOString(),
      drift_amount: 0,
      action: "ENGINE_UNREACHABLE",
      kill_switch_triggered: false,
      domain: "go_engine",
    });
    if (overall === "GREEN") overall = "AMBER";
  }

  return NextResponse.json({
    ok: true,
    execution_authority: "mock-trading",
    overall,
    events,
    server_time: new Date().toISOString(),
  });
}
