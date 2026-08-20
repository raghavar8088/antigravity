"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { PnlValue } from "@/components/ui/PnlValue";
import { Sparkline } from "@/components/ui/Sparkline";
import { Badge } from "@/components/ui/Badge";
import { cn } from "@/components/ui/cn";
import type { PnlUpdate } from "@/types/trading";
import { AutoSortTable } from "@/components/desk/ui";

interface StrategyRow extends PnlUpdate {
  history: number[];
  flashClass: string;
}

function useAutoReconnectSSE(url: string, onMessage: (data: unknown) => void) {
  const backoffRef = useRef(1000);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function connect() {
      while (!cancelled) {
        abortRef.current = new AbortController();
        try {
          const res = await fetch(url, { signal: abortRef.current.signal, cache: "no-store" });
          if (!res.body) throw new Error("No response body");
          const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
          backoffRef.current = 1000;
          let buf = "";
          while (!cancelled) {
            const { done, value } = await reader.read();
            if (done) break;
            buf += value;
            const lines = buf.split("\n");
            buf = lines.pop() ?? "";
            for (const line of lines) {
              if (line.startsWith("data: ")) {
                try { onMessage(JSON.parse(line.slice(6))); } catch { /* ignore malformed */ }
              }
            }
          }
        } catch {
          if (cancelled) break;
          await new Promise((r) => setTimeout(r, backoffRef.current));
          backoffRef.current = Math.min(backoffRef.current * 2, 30_000);
        }
      }
    }
    connect();
    return () => {
      cancelled = true;
      abortRef.current?.abort();
    };
  }, [url, onMessage]);
}

export function LivePnlFeed() {
  const [rows, setRows] = useState<Map<string, StrategyRow>>(new Map());
  const [showOpenOnly, setShowOpenOnly] = useState(false);
  const [sessionPnl, setSessionPnl] = useState(0);
  const [connected, setConnected] = useState(false);

  const onMessage = useCallback((data: unknown) => {
    const d = data as { ok?: boolean; events?: PnlUpdate[] };
    if (!d.ok || !Array.isArray(d.events)) return;
    setConnected(true);
    const updates = d.events;

    setRows((prev) => {
      const next = new Map(prev);
      for (const update of updates) {
        const existing = next.get(update.strategyId);
        const flashClass = update.unrealizedPnl >= (existing?.unrealizedPnl ?? 0) ? "flash-profit" : "flash-loss";
        next.set(update.strategyId, {
          ...update,
          history: [...(existing?.history ?? []), update.unrealizedPnl].slice(-30),
          flashClass,
        });
      }
      const total = Array.from(next.values()).reduce((s, r) => s + r.sessionPnl, 0);
      setSessionPnl(total);
      return next;
    });
  }, []);

  useAutoReconnectSSE("/api/engine/events", onMessage);

  const sorted = Array.from(rows.values())
    .filter((r) => !showOpenOnly || r.unrealizedPnl !== 0)
    .sort((a, b) => Math.abs(b.unrealizedPnl) - Math.abs(a.unrealizedPnl));

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--color-border)]">
        <div className="flex items-center gap-2">
          <span className="text-[12px] font-medium text-[var(--color-text-secondary)]">Live P&L</span>
          {connected ? (
            <span className="w-[5px] h-[5px] rounded-full bg-[var(--color-profit)] animate-pulse" aria-label="Connected" />
          ) : (
            <span className="text-[10px] text-[var(--color-warning)]">Reconnecting…</span>
          )}
        </div>
        <div className="flex items-center gap-3">
          <PnlValue value={sessionPnl} showSign size="lg" />
          <label className="flex items-center gap-1 text-[11px] text-[var(--color-text-muted)] cursor-pointer">
            <input
              type="checkbox"
              checked={showOpenOnly}
              onChange={(e) => setShowOpenOnly(e.target.checked)}
              className="accent-[var(--color-info)]"
            />
            Open only
          </label>
        </div>
      </div>

      {/* Rows */}
      <div className="flex-1 overflow-y-auto">
        {sorted.length === 0 ? (
          <div className="flex items-center justify-center h-full text-[12px] text-[var(--color-text-muted)]">
            No positions — waiting for data…
          </div>
        ) : (
          <AutoSortTable><table className="w-full" role="table">
            <thead>
              <tr className="text-[10px] text-[var(--color-text-muted)] border-b border-[var(--color-border)]">
                <th className="px-3 py-1.5 text-left font-medium">Strategy</th>
                <th className="px-2 py-1.5 text-left font-medium">Symbol</th>
                <th className="px-2 py-1.5 text-right font-medium">Entry</th>
                <th className="px-2 py-1.5 text-right font-medium">Current</th>
                <th className="px-2 py-1.5 text-right font-medium">Unrealized</th>
                <th className="px-2 py-1.5 text-center font-medium">7d</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((row) => (
                <tr
                  key={row.strategyId}
                  className={cn("border-b border-[var(--color-border)] hover:bg-[var(--color-bg-elevated)]", row.flashClass)}
                  onAnimationEnd={(e) => {
                    e.currentTarget.classList.remove("flash-profit", "flash-loss");
                  }}
                >
                  <td className="px-3 py-1.5 text-[11px] text-[var(--color-text-primary)] max-w-[120px] truncate">
                    {row.strategyId}
                  </td>
                  <td className="px-2 py-1.5">
                    <div className="flex items-center gap-1">
                      <span className="text-[11px] font-mono">{row.symbol}</span>
                      <Badge variant={row.direction === "LONG" ? "profit" : "loss"} size="sm">{row.direction}</Badge>
                    </div>
                  </td>
                  <td className="px-2 py-1.5 text-right font-mono tnum text-[11px] text-[var(--color-text-secondary)]">
                    {row.entryPrice.toLocaleString("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </td>
                  <td className="px-2 py-1.5 text-right font-mono tnum text-[11px] text-[var(--color-text-primary)]">
                    {row.currentPrice.toLocaleString("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </td>
                  <td className="px-2 py-1.5 text-right">
                    <PnlValue value={row.unrealizedPnl} showSign size="sm" />
                  </td>
                  <td className="px-2 py-1.5 flex justify-center">
                    <Sparkline data={row.history} width={60} height={20} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table></AutoSortTable>
        )}
      </div>
    </div>
  );
}
