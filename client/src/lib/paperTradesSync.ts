import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
import { btcFuturesTradeToClientPayload } from "@/lib/paperTradesMapper";
import { PAPER_TRADES_MAX_LOCAL, type PaperTradeModuleKey } from "@/lib/paperTradesTypes";

const MAX_SYNC_RETRIES = 3;
const FLUSH_THROTTLE_MS = 10_000;
let syncFailureLogged = false;
let lastFlushAtMs = 0;

type QueuedPaperTrade = {
  accountKey: string;
  trade: BTCFuturesTrade;
  retries: number;
  moduleKey?: PaperTradeModuleKey;
};

// In-memory retry queue — no localStorage. Trades are POSTed immediately on
// close; this queue only holds items that failed and need retry on next flush.
const inMemoryQueues = new Map<string, QueuedPaperTrade[]>();

function readQueue(accountKey: string): QueuedPaperTrade[] {
  return inMemoryQueues.get(accountKey) ?? [];
}

function writeQueue(accountKey: string, items: QueuedPaperTrade[]): void {
  if (items.length === 0) {
    inMemoryQueues.delete(accountKey);
  } else {
    inMemoryQueues.set(accountKey, items);
  }
}

function enqueuePaperTrade(
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
  const tradeId = trade.clientTradeId ?? trade.id;
  const body: Record<string, unknown> = {
    accountKey,
    trade: btcFuturesTradeToClientPayload(trade),
    collectionName: "paper_trades",
  };
  if (moduleKey) body.moduleKey = moduleKey;

  if (process.env.NODE_ENV === "development") {
    console.log("[paper-sync] POST start", { tradeId, accountKey, moduleKey });
  }

  let res: Response;
  try {
    res = await fetch("/api/paper-trades", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify(body),
    });
  } catch (err) {
    if (process.env.NODE_ENV === "development") {
      console.warn("[paper-sync] POST network error", {
        tradeId,
        error: err instanceof Error ? err.message : String(err),
      });
    }
    return false;
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    console.error("[paper-sync] POST failed", { tradeId, status: res.status, body });
    return false;
  }

  if (process.env.NODE_ENV === "development") {
    console.log("[paper-sync] POST ok", { tradeId, status: res.status });
  }
  return true;
}

/**
 * Fire-and-forget: persist one closed trade when `accountKey` is set.
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

/**
 * Flush the in-memory retry queue. Throttled to once per 10s; pass
 * `{ force: true }` on mount to drain immediately.
 */
export async function flushTradeSyncQueue(
  accountKey: string | null,
  opts?: { force?: boolean },
): Promise<void> {
  if (!accountKey?.trim()) return;
  const now = Date.now();
  if (!opts?.force && now - lastFlushAtMs < FLUSH_THROTTLE_MS) return;
  lastFlushAtMs = now;
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
      console.warn("[paper-trades] Dropped trade(s) from sync queue after max retries.");
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
