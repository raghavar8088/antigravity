import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
import { btcFuturesTradeToClientPayload } from "@/lib/paperTradesMapper";
import { PAPER_TRADES_MAX_LOCAL, type PaperTradeModuleKey } from "@/lib/paperTradesTypes";

const MAX_SYNC_RETRIES = 3;
let syncFailureLogged = false;

const fetchOpts: RequestInit = { credentials: "include" };

type QueuedPaperTrade = {
  accountKey: string;
  trade: BTCFuturesTrade;
  retries: number;
  /** Module identifier — persisted with the row when re-POSTing from queue. */
  moduleKey?: PaperTradeModuleKey;
};

export function tradeSyncQueueKey(accountKey: string): string {
  return `${accountKey}_trade_sync_queue`;
}

function readQueue(accountKey: string): QueuedPaperTrade[] {
  if (typeof localStorage === "undefined") return [];
  try {
    const raw = localStorage.getItem(tradeSyncQueueKey(accountKey));
    if (!raw) return [];
    const parsed = JSON.parse(raw) as QueuedPaperTrade[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function writeQueue(accountKey: string, items: QueuedPaperTrade[]): void {
  if (typeof localStorage === "undefined") return;
  try {
    if (items.length === 0) {
      localStorage.removeItem(tradeSyncQueueKey(accountKey));
      return;
    }
    localStorage.setItem(tradeSyncQueueKey(accountKey), JSON.stringify(items));
  } catch {
    // quota / private mode
  }
}

export function enqueuePaperTrade(
  accountKey: string,
  trade: BTCFuturesTrade,
  moduleKey?: PaperTradeModuleKey,
): void {
  const queue = readQueue(accountKey);
  const key = trade.clientTradeId ?? trade.id;
  if (queue.some((q) => (q.trade.clientTradeId ?? q.trade.id) === key)) return;
  queue.push({ accountKey, trade, retries: 0, moduleKey });
  writeQueue(accountKey, queue);
}

async function postPaperTrade(
  accountKey: string,
  trade: BTCFuturesTrade,
  moduleKey?: PaperTradeModuleKey,
): Promise<boolean> {
  const body: Record<string, unknown> = {
    accountKey,
    trade: btcFuturesTradeToClientPayload(trade),
  };
  if (moduleKey) body.moduleKey = moduleKey;
  const res = await fetch("/api/paper-trades", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(body),
  });
  if (res.status === 401) return false;
  if (res.status === 503) return false;
  if (!res.ok) {
    if (process.env.NODE_ENV === "development") {
      const text = await res.text().catch(() => "");
      console.warn("[paper-trades-sync] POST /api/paper-trades failed", res.status, text.slice(0, 500));
    }
    return false;
  }
  return true;
}

/**
 * Fire-and-forget: persist one closed trade when `accountKey` is set.
 * Local paper-only mode remains if no key is available.
 *
 * `moduleKey` (optional) tags the row with the originating workspace tab so
 * dashboards can filter per-module leaderboards / exports.
 */
export function persistTradeToServer(
  trade: BTCFuturesTrade,
  accountKey: string | null,
  moduleKey?: PaperTradeModuleKey,
): void {
  if (!accountKey?.trim()) return;
  const key = accountKey.trim();
  void (async () => {
    try {
      const ok = await postPaperTrade(key, trade, moduleKey);
      if (!ok) enqueuePaperTrade(key, trade, moduleKey);
    } catch {
      enqueuePaperTrade(key, trade, moduleKey);
    }
  })();
}

/** POST all queued trades (max retries per item). No-op when logged out. */
export async function flushTradeSyncQueue(accountKey: string | null): Promise<void> {
  if (!accountKey?.trim()) return;
  const key = accountKey.trim();
  const queue = readQueue(key);
  if (queue.length === 0) return;

  const remaining: QueuedPaperTrade[] = [];
  for (const item of queue) {
    try {
      const ok = await postPaperTrade(item.accountKey, item.trade, item.moduleKey);
      if (ok) continue;
    } catch {
      // fall through to retry
    }
    const nextRetries = item.retries + 1;
    if (nextRetries < MAX_SYNC_RETRIES) {
      remaining.push({ ...item, retries: nextRetries });
    } else if (!syncFailureLogged) {
      syncFailureLogged = true;
      console.warn(
        "[paper-trades] Dropped trade(s) from sync queue after max retries. Check MongoDB env and server connectivity.",
      );
    }
  }
  writeQueue(key, remaining);
}

export async function fetchPaperTradesFromServer(
  accountKey: string | null,
  limit = 50,
): Promise<BTCFuturesTrade[]> {
  if (!accountKey?.trim()) return [];
  const params = new URLSearchParams({ limit: String(limit), account_key: accountKey });
  const res = await fetch(`/api/paper-trades?${params.toString()}`, {
    cache: "no-store",
    credentials: "include",
  });
  if (res.status === 401) return [];
  if (!res.ok) return [];
  const j = (await res.json()) as { ok?: boolean; trades?: BTCFuturesTrade[] };
  return j.ok && Array.isArray(j.trades) ? j.trades : [];
}

export function mergeTradesByClientTradeId(
  local: readonly BTCFuturesTrade[],
  server: readonly BTCFuturesTrade[],
): BTCFuturesTrade[] {
  const byKey = new Map<string, BTCFuturesTrade>();
  for (const t of [...local, ...server]) {
    const key = t.clientTradeId ?? t.id;
    const prev = byKey.get(key);
    if (!prev || new Date(t.closedAt).getTime() >= new Date(prev.closedAt).getTime()) {
      byKey.set(key, t);
    }
  }
  return [...byKey.values()]
    .sort((a, b) => new Date(b.closedAt).getTime() - new Date(a.closedAt).getTime())
    .slice(0, PAPER_TRADES_MAX_LOCAL);
}

export async function pullAndMergePaperTrades(
  accountKey: string | null,
  local: readonly BTCFuturesTrade[],
): Promise<BTCFuturesTrade[]> {
  if (!accountKey?.trim()) return [...local];
  const server = await fetchPaperTradesFromServer(accountKey, 50);
  if (server.length === 0) return [...local];
  return mergeTradesByClientTradeId(local, server);
}
