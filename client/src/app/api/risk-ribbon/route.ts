import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import {
  getPaperState,
  listOpenPositions,
  getClosedTradeStats,
  listPaperOrders,
} from "@/lib/paperDeskClient";

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
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  const items: RibbonItem[] = [];
  const engineBase = process.env.INTERNAL_API_URL?.trim().replace(/\/$/, "") ?? "http://localhost:8080";

  const btc = await fetchBtcPrice();
  items.push({
    label: "MARKET DATA",
    status: btc ? "GREEN" : "RED",
    value: btc ? `$${Math.round(btc.price).toLocaleString()}` : "OFFLINE",
    detail: btc ? `${btc.change24h >= 0 ? "+" : ""}${btc.change24h.toFixed(2)}% 24h` : undefined,
  });

  let engineStatus: RibbonStatus = "UNKNOWN";
  let engineDetail = "unreachable";
  try {
    const r = await fetch(`${engineBase}/health`, { signal: AbortSignal.timeout(3_000) });
    if (r.ok) {
      engineStatus = "GREEN";
      engineDetail = "online";
    } else {
      engineStatus = "RED";
      engineDetail = `HTTP ${r.status}`;
    }
  } catch {
    engineStatus = "RED";
    engineDetail = "timeout";
  }
  items.push({ label: "ENGINE", status: engineStatus, value: engineDetail.toUpperCase() });
  items.push({ label: "EXECUTION", status: engineStatus, value: engineStatus === "GREEN" ? "HEALTHY" : "DEGRADED" });
  items.push({ label: "WATCHDOG", status: engineStatus, value: engineStatus === "GREEN" ? "OK" : "ALERT" });

  const mongoOk = isMongoConfigured();
  items.push({
    label: "DATABASE",
    status: mongoOk ? "GREEN" : "RED",
    value: mongoOk ? "CONNECTED" : "UNCONFIGURED",
  });

  let dailyDrawdown = 0;
  let maxDrawdown = 0;
  let openPositions = 0;
  let balance = 0;
  let equity = 0;
  let todayPnl = 0;
  let portfolioStatus: RibbonStatus = "UNKNOWN";
  let omsStatus: RibbonStatus = "UNKNOWN";
  let reconStatus: RibbonStatus = "UNKNOWN";

  if (mongoOk) {
    try {
      const [state, positions, closedStats, orders] = await Promise.all([
        getPaperState(accountKey),
        listOpenPositions(accountKey),
        getClosedTradeStats(accountKey),
        listPaperOrders({ accountKey, limit: 5 }),
      ]);
      if (state) {
        dailyDrawdown = state.current_drawdown ?? 0;
        maxDrawdown = state.max_drawdown ?? 0;
        balance = state.balance ?? 0;
        equity = state.equity ?? balance;
        openPositions = (positions ?? []).length;
        todayPnl = closedStats.realized_pnl ?? 0;

        if (dailyDrawdown < -0.03 || maxDrawdown < -0.1) portfolioStatus = "RED";
        else if (dailyDrawdown < -0.01 || maxDrawdown < -0.05) portfolioStatus = "AMBER";
        else portfolioStatus = "GREEN";

        const staleMs = state.snapped_at ? Date.now() - new Date(state.snapped_at).getTime() : Infinity;
        reconStatus = staleMs > 120_000 ? "AMBER" : "GREEN";
      } else {
        portfolioStatus = "AMBER";
        reconStatus = "AMBER";
      }
      omsStatus = (orders ?? []).length > 0 || openPositions > 0 ? "GREEN" : "GREEN";
    } catch {
      portfolioStatus = "RED";
      omsStatus = "RED";
      reconStatus = "RED";
    }
  }

  items.push({ label: "OMS", status: omsStatus, value: omsStatus === "GREEN" ? "ACTIVE" : "DEGRADED" });
  items.push({ label: "RECON", status: reconStatus, value: reconStatus === "GREEN" ? "OK" : reconStatus === "AMBER" ? "STALE" : "FAIL" });

  items.push({
    label: "PORTFOLIO RISK",
    status: portfolioStatus,
    value: portfolioStatus === "UNKNOWN" ? "NO DATA" : portfolioStatus,
    detail: `DD: ${(dailyDrawdown * 100).toFixed(2)}%`,
  });

  items.push({
    label: "DAILY DRAWDOWN",
    status: dailyDrawdown < -0.03 ? "RED" : dailyDrawdown < -0.01 ? "AMBER" : "GREEN",
    value: `${(dailyDrawdown * 100).toFixed(2)}%`,
    detail: "limit: -3%",
  });

  items.push({
    label: "MAX DRAWDOWN",
    status: maxDrawdown < -0.10 ? "RED" : maxDrawdown < -0.05 ? "AMBER" : "GREEN",
    value: `${(maxDrawdown * 100).toFixed(2)}%`,
    detail: "limit: -10%",
  });

  items.push({
    label: "POSITIONS",
    status: openPositions === 0 ? "GREEN" : openPositions > 10 ? "AMBER" : "GREEN",
    value: String(openPositions),
    detail: "open",
  });

  items.push({
    label: "TODAY PnL",
    status: todayPnl >= 0 ? "GREEN" : todayPnl < -1000 ? "RED" : "AMBER",
    value: todayPnl >= 0 ? `+$${Math.round(todayPnl).toLocaleString()}` : `-$${Math.round(Math.abs(todayPnl)).toLocaleString()}`,
    detail: "realized",
  });

  items.push({
    label: "EXPOSURE",
    status: equity > 0 ? "GREEN" : "AMBER",
    value: equity > 0 ? `$${equity.toLocaleString("en-US", { maximumFractionDigits: 0 })}` : "NO DATA",
    detail: "equity",
  });

  let killSwitchStatus: RibbonStatus = "UNKNOWN";
  let killSwitchValue = "UNKNOWN";
  if (engineStatus === "GREEN") {
    try {
      const r = await fetch(`${engineBase}/api/killswitch/status`, {
        signal: AbortSignal.timeout(2_000),
      });
      if (r.ok) {
        const kd = await r.json();
        const active = kd.active === true || kd.killed === true;
        killSwitchStatus = active ? "RED" : "GREEN";
        killSwitchValue = active ? "TRIGGERED" : "ARMED";
      } else {
        killSwitchStatus = "AMBER";
        killSwitchValue = "UNREACHABLE";
      }
    } catch {
      killSwitchStatus = "AMBER";
      killSwitchValue = "UNREACHABLE";
    }
  }
  items.push({ label: "KILL SWITCH", status: killSwitchStatus, value: killSwitchValue });

  const redCount = items.filter((i) => i.status === "RED").length;
  const amberCount = items.filter((i) => i.status === "AMBER").length;
  const overallStatus: RibbonStatus = redCount > 0 ? "RED" : amberCount > 0 ? "AMBER" : "GREEN";

  return NextResponse.json({
    ok: true,
    overall: overallStatus,
    items,
    server_time: new Date().toISOString(),
  });
}
