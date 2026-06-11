import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { buildMockStrategyIntelRows } from "@/lib/mockTradingSnapshotService";
import { listMockTrades, getLatestMockAccountSnapshot } from "@/lib/mockTradingMongo";
import { getTodayRealizedPnlUtc } from "@/lib/portfolioAccountingService";
import { mongoUnconfigured } from "@/lib/mockTradingApiErrors";
import { OWNER_ACCOUNT_KEY } from "@/lib/ownerAuth";

export const dynamic = "force-dynamic";

type RibbonStatus = "GREEN" | "AMBER" | "RED" | "UNKNOWN";

type RibbonItem = {
  label: string;
  status: RibbonStatus;
  value: string;
  detail?: string;
};

async function fetchBtcPrice(): Promise<{ price: number; change24h: number } | null> {
  try {
    const base = process.env.VERCEL_URL
      ? `https://${process.env.VERCEL_URL}`
      : `http://127.0.0.1:${process.env.PORT ?? 3000}`;
    const r = await fetch(`${base}/api/btc/price`, { signal: AbortSignal.timeout(3_000), cache: "no-store" });
    if (!r.ok) return null;
    const d = await r.json();
    if (!d.ok || !Number.isFinite(d.price)) return null;
    return { price: d.price, change24h: Number(d.change24h ?? 0) };
  } catch {
    return null;
  }
}

export async function GET() {
  const accountKey = OWNER_ACCOUNT_KEY;
  const items: RibbonItem[] = [];
  const engineBase = process.env.INTERNAL_API_URL?.trim().replace(/\/$/, "") ?? "http://localhost:8080";

  const btc = await fetchBtcPrice();
  items.push({
    label: "MARKET DATA",
    status: btc ? "GREEN" : "RED",
    value: btc ? `$${Math.round(btc.price).toLocaleString()}` : "OFFLINE",
    detail: btc ? `${btc.change24h >= 0 ? "+" : ""}${btc.change24h.toFixed(2)}% 24h` : undefined,
  });

  if (!isMongoConfigured()) {
    items.push({ label: "MOCK TRADING", status: "RED", value: "NO MONGO" });
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
      label: "MOCK TRADING",
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
    items.push({ label: "MOCK TRADING", status: "RED", value: "READ FAIL" });
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
