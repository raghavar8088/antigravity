import { NextResponse } from "next/server";
import { fetchBtcSpotPrice } from "@/lib/trading/btcSpotPrice";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { buildMockStrategyIntelRows } from "@/lib/trading/mockTradingSnapshotService";
import { listMockTrades, getLatestMockAccountSnapshot } from "@/lib/trading/mockTradingMongo";
import { getTodayRealizedPnlUtc } from "@/lib/portfolio/portfolioAccountingService";
import { mongoUnconfigured } from "@/lib/trading/mockTradingApiErrors";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

type RibbonStatus = "GREEN" | "AMBER" | "RED" | "UNKNOWN";

type RibbonItem = {
  label: string;
  status: RibbonStatus;
  value: string;
  detail?: string;
};

export async function GET() {
  const accountKey = OWNER_ACCOUNT_KEY;
  const items: RibbonItem[] = [];
  const engineBase = process.env.INTERNAL_API_URL?.trim().replace(/\/$/, "") ?? "http://localhost:8080";

  const btc = await fetchBtcSpotPrice();
  items.push({
    label: "MARKET DATA",
    status: btc ? "GREEN" : "RED",
    value: btc ? `$${Math.round(btc.price).toLocaleString()}` : "OFFLINE",
    detail: btc ? `${btc.change24h >= 0 ? "+" : ""}${btc.change24h.toFixed(2)}% 24h` : undefined,
  });

  if (!isMongoConfigured()) {
    items.push({ label: "TRADE ENGINE", status: "RED", value: "NO MONGO" });
    return NextResponse.json({ ok: true, items, execution_authority: "mock-trading" });
  }

  try {
    const [{ account }, open, todayPnl, strategies] = await Promise.all([
      getLatestMockAccountSnapshot(accountKey),
      listMockTrades({ account_key: accountKey, status: "OPEN", page: 1, limit: 200, sort: "newest" }),
      getTodayRealizedPnlUtc(accountKey),
      buildMockStrategyIntelRows(accountKey),
    ]);

    const equity = account?.equity ?? 0;
    const dd = account?.maxDrawdownPct ?? 0;
    const critical = strategies.filter((s) => s.status === "CRITICAL").length;

    items.push({
      label: "TRADE ENGINE",
      status: "GREEN",
      value: `$${Math.round(equity).toLocaleString()}`,
      detail: `${open.trades.length} open · today ${todayPnl >= 0 ? "+" : ""}$${todayPnl.toFixed(0)}`,
    });
    items.push({
      label: "DRAWDOWN",
      status: dd > 0.05 ? "RED" : dd > 0.02 ? "AMBER" : "GREEN",
      value: `${(dd * 100).toFixed(2)}%`,
    });
    items.push({
      label: "STRATEGY HEALTH",
      status: critical > 5 ? "RED" : critical > 0 ? "AMBER" : "GREEN",
      value: critical > 0 ? `${critical} critical` : "OK",
    });
  } catch {
    items.push({ label: "TRADE ENGINE", status: "RED", value: "READ FAIL" });
  }

  try {
    const res = await fetch(`${engineBase}/health`, { signal: AbortSignal.timeout(2_000), cache: "no-store" });
    items.push({
      label: "GO ENGINE",
      status: res.ok ? "GREEN" : "AMBER",
      value: res.ok ? "UP" : "DEGRADED",
    });
  } catch {
    items.push({ label: "GO ENGINE", status: "AMBER", value: "OFFLINE" });
  }

  return NextResponse.json({
    ok: true,
    execution_authority: "mock-trading",
    items,
    server_time: new Date().toISOString(),
  });
}
