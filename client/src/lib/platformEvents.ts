import { isMongoConfigured } from "@/lib/mongoTradesClient";
import {
  listPaperTrades,
  listOpenPositions,
  listPaperOrders,
  getPaperState,
} from "@/lib/paperDeskClient";

export type EventType =
  | "FILL"
  | "SIGNAL"
  | "POSITION_OPEN"
  | "POSITION_CLOSE"
  | "RISK_EVENT"
  | "KILL_SWITCH"
  | "RECONCILIATION"
  | "SYSTEM"
  | "ORDER";

export type EventSeverity = "INFO" | "WARNING" | "CRITICAL";

export type PlatformEvent = {
  id: string;
  type: EventType;
  severity: EventSeverity;
  title: string;
  detail: string;
  strategy?: string;
  symbol?: string;
  pnl?: number;
  ts: string;
};

export async function buildPlatformEvents(accountKey: string): Promise<PlatformEvent[]> {
  if (!isMongoConfigured()) return [];

  const [trades, positions, state, orders] = await Promise.all([
    listPaperTrades({ accountKey, limit: 50, sortBy: "closed_at", sortDir: "desc" }),
    listOpenPositions(accountKey),
    getPaperState(accountKey),
    listPaperOrders({ accountKey, limit: 30 }),
  ]);

  const events: PlatformEvent[] = [];

  for (const t of trades ?? []) {
    const ts = t.closed_at ?? t.exit_at ?? new Date().toISOString();
    const pnl = typeof t.net_pnl === "number" ? t.net_pnl : 0;
    events.push({
      id: `fill-${t.client_trade_id ?? ts}`,
      type: "FILL",
      severity: pnl >= 0 ? "INFO" : "WARNING",
      title: `FILL · ${t.side ?? "?"} ${t.symbol ?? "BTCUSD"}`,
      detail: `${t.strategy_id ?? "unknown"} · Entry ${t.entry_price?.toFixed(0) ?? "?"} → Exit ${t.exit_price?.toFixed(0) ?? "?"} · PnL ${pnl >= 0 ? "+" : ""}${pnl.toFixed(2)} · slip ${t.slippage_bps ?? 0}bps`,
      strategy: t.strategy_id,
      symbol: t.symbol,
      pnl,
      ts,
    });
  }

  for (const p of positions ?? []) {
    events.push({
      id: `pos-${p.position_id ?? p.strategy_id}`,
      type: "POSITION_OPEN",
      severity: "INFO",
      title: `POSITION · ${p.side ?? "?"} ${p.symbol ?? "BTCUSD"}`,
      detail: `${p.strategy_id ?? "unknown"} · ${p.size ?? 0} BTC @ ${p.entry_price?.toFixed(0) ?? "?"}`,
      strategy: p.strategy_id,
      symbol: p.symbol,
      ts: p.opened_at ?? new Date().toISOString(),
    });
  }

  for (const o of orders ?? []) {
    events.push({
      id: `order-${o.order_id ?? o.transition_to}`,
      type: "ORDER",
      severity: "INFO",
      title: `OMS · ${o.transition_to?.replace(/_/g, " ") ?? "ORDER"}`,
      detail: `${o.strategy_id ?? ""} · ${o.side ?? ""} · ${o.symbol ?? "BTCUSD"}`,
      strategy: o.strategy_id,
      symbol: o.symbol,
      ts: o.recorded_at ?? o.transition_at ?? new Date().toISOString(),
    });
  }

  if (state) {
    const dd = state.current_drawdown ?? 0;
    if (dd < -0.03) {
      events.push({
        id: `risk-dd-${state.snapped_at}`,
        type: "RISK_EVENT",
        severity: "CRITICAL",
        title: "DRAWDOWN GUARD TRIGGERED",
        detail: `Current drawdown ${(dd * 100).toFixed(2)}% exceeds -3% threshold.`,
        ts: state.snapped_at ?? new Date().toISOString(),
      });
    } else if (dd < -0.01) {
      events.push({
        id: `risk-dd-warn-${state.snapped_at}`,
        type: "RISK_EVENT",
        severity: "WARNING",
        title: "DRAWDOWN WARNING",
        detail: `Current drawdown ${(dd * 100).toFixed(2)}% approaching limit.`,
        ts: state.snapped_at ?? new Date().toISOString(),
      });
    }
  }

  const engineBase = process.env.INTERNAL_API_URL?.trim().replace(/\/$/, "") ?? "http://localhost:8080";
  try {
    const r = await fetch(`${engineBase}/api/killswitch/status`, { signal: AbortSignal.timeout(2_000) });
    if (r.ok) {
      const kd = await r.json();
      if (kd.active === true || kd.killed === true) {
        events.push({
          id: `ks-${Date.now()}`,
          type: "KILL_SWITCH",
          severity: "CRITICAL",
          title: "KILL SWITCH TRIGGERED",
          detail: String(kd.reason ?? "Engine halted new orders"),
          ts: new Date().toISOString(),
        });
      }
    }
  } catch {
    // optional
  }

  events.sort((a, b) => new Date(b.ts).getTime() - new Date(a.ts).getTime());
  return events;
}
