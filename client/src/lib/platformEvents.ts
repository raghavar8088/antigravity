import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { listMockLogs, listMockTrades } from "@/lib/mockTradingMongo";

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

  const [closed, open, logs] = await Promise.all([
    listMockTrades({ account_key: accountKey, status: "CLOSED", page: 1, limit: 50, sort: "newest" }),
    listMockTrades({ account_key: accountKey, status: "OPEN", page: 1, limit: 50, sort: "newest" }),
    listMockLogs({ account_key: accountKey, page: 1, limit: 50 }),
  ]);

  const events: PlatformEvent[] = [];

  for (const t of closed.trades) {
    const ts = t.closedAt ? new Date(t.closedAt).toISOString() : new Date().toISOString();
    events.push({
      id: `fill-${t.id}`,
      type: "FILL",
      severity: t.realizedPnl >= 0 ? "INFO" : "WARNING",
      title: `FILL · ${t.side} ${t.symbol}`,
      detail: `${t.strategyName} · PnL ${t.realizedPnl >= 0 ? "+" : ""}${t.realizedPnl.toFixed(2)}`,
      strategy: t.strategyName,
      symbol: t.symbol,
      pnl: t.realizedPnl,
      ts,
    });
  }

  for (const t of open.trades) {
    events.push({
      id: `open-${t.id}`,
      type: "POSITION_OPEN",
      severity: "INFO",
      title: `POSITION OPEN · ${t.side} ${t.symbol}`,
      detail: `${t.strategyName} · entry ${t.entryPrice.toFixed(2)}`,
      strategy: t.strategyName,
      symbol: t.symbol,
      ts: new Date(t.openedAt).toISOString(),
    });
  }

  for (const log of logs.logs) {
    events.push({
      id: `log-${log.ts}-${log.event}`,
      type: log.event.includes("REJECT") ? "SIGNAL" : "SYSTEM",
      severity: log.event.includes("REJECT") ? "WARNING" : "INFO",
      title: log.event,
      detail: log.message ?? log.event,
      ts: new Date(log.ts).toISOString(),
    });
  }

  return events.sort((a, b) => new Date(b.ts).getTime() - new Date(a.ts).getTime()).slice(0, 100);
}
