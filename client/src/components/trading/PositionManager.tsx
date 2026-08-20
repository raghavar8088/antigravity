"use client";

import { useCallback, useEffect, useState } from "react";
import { PnlValue } from "@/components/ui/PnlValue";
import { Badge } from "@/components/ui/Badge";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { cn } from "@/components/ui/cn";
import type { Position } from "@/types/trading";
import { AutoSortTable } from "@/components/desk/ui";

function ageColor(ageMs: number): string {
  const h = ageMs / 3_600_000;
  if (h < 1)  return "text-[var(--color-profit)]";
  if (h < 4)  return "text-[var(--color-caution)]";
  if (h < 8)  return "text-[var(--color-warning)]";
  return "text-[var(--color-loss)]";
}

function formatAge(ms: number): string {
  const h = Math.floor(ms / 3_600_000);
  const m = Math.floor((ms % 3_600_000) / 60_000);
  return h > 0 ? `${h}h ${m}m` : `${m}m`;
}

export function PositionManager() {
  const [positions, setPositions] = useState<Position[]>([]);
  const [loading, setLoading] = useState(true);
  const [closeTarget, setCloseTarget] = useState<Position | null>(null);
  const [closeTyped, setCloseTyped] = useState("");
  const [closing, setClosing] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await fetch("/api/trading/positions", { cache: "no-store" });
      const data = await res.json();
      if (Array.isArray(data.positions)) setPositions(data.positions);
    } catch { /* engine offline */ }
    finally { setLoading(false); }
  }, []);

  useEffect(() => {
    load();
    const id = setInterval(load, 5_000);
    return () => clearInterval(id);
  }, [load]);

  const totalUnrealized = positions.reduce((s, p) => s + p.unrealizedPnl, 0);
  const totalExposure = positions.reduce((s, p) => s + Math.abs(p.size * p.currentPrice), 0);

  async function forceClose() {
    if (!closeTarget || closeTyped !== "CLOSE") return;
    setClosing(true);
    try {
      await fetch("/api/trading/positions", {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ positionId: closeTarget.positionId, symbol: closeTarget.symbol, quantity: closeTarget.size }),
      });
      await load();
    } finally {
      setClosing(false);
      setCloseTarget(null);
      setCloseTyped("");
    }
  }

  return (
    <>
      <div className="flex flex-col h-full">
        {/* Aggregate row */}
        <div className="flex items-center gap-4 px-3 py-2 border-b border-[var(--color-border)] text-[11px]">
          <span className="text-[var(--color-text-muted)]">{positions.length} positions</span>
          <span className="text-[var(--color-text-secondary)]">
            Unrealized: <PnlValue value={totalUnrealized} showSign size="sm" />
          </span>
          <span className="text-[var(--color-text-secondary)]">
            Exposure: <span className="font-mono tnum text-[var(--color-text-primary)]">
              ${totalExposure.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
            </span>
          </span>
        </div>

        <div className="flex-1 overflow-auto">
          {loading ? (
            <div className="p-4 space-y-2">
              {Array.from({ length: 4 }).map((_, i) => <div key={i} className="skeleton h-8 rounded" />)}
            </div>
          ) : positions.length === 0 ? (
            <div className="flex items-center justify-center h-full text-[12px] text-[var(--color-text-muted)]">
              No open positions
            </div>
          ) : (
            <AutoSortTable><table className="w-full" role="table">
              <thead className="sticky top-0 bg-[var(--color-bg-surface)]">
                <tr className="text-[10px] text-[var(--color-text-muted)]">
                  {["Symbol", "Exch", "Strategy", "Dir", "Size", "Entry", "Current", "P&L", "Return", "SL", "Target", "Age", ""].map((h) => (
                    <th key={h} className="px-2 py-1.5 text-left font-medium whitespace-nowrap">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {positions.map((pos) => (
                  <tr
                    key={pos.positionId}
                    className="border-b border-[var(--color-border)] hover:bg-[var(--color-bg-elevated)] text-[11px]"
                  >
                    <td className="px-2 py-1.5 font-mono font-medium text-[var(--color-text-primary)]">{pos.symbol}</td>
                    <td className="px-2 py-1.5">
                      <Badge variant="neutral" size="sm">{pos.exchange}</Badge>
                    </td>
                    <td className="px-2 py-1.5 text-[var(--color-text-secondary)] max-w-[100px] truncate">{pos.strategyId}</td>
                    <td className="px-2 py-1.5">
                      <Badge variant={pos.direction === "LONG" ? "profit" : "loss"} size="sm">{pos.direction}</Badge>
                    </td>
                    <td className="px-2 py-1.5 font-mono tnum text-right text-[var(--color-text-primary)]">{pos.size}</td>
                    <td className="px-2 py-1.5 font-mono tnum text-right text-[var(--color-text-secondary)]">
                      {pos.entryPrice.toLocaleString("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                    </td>
                    <td className="px-2 py-1.5 font-mono tnum text-right text-[var(--color-text-primary)]">
                      {pos.currentPrice.toLocaleString("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                    </td>
                    <td className="px-2 py-1.5 text-right">
                      <PnlValue value={pos.unrealizedPnl} showSign size="sm" />
                    </td>
                    <td className="px-2 py-1.5 text-right">
                      <PnlValue value={pos.returnPct} showSign size="sm" format={(v) => `${Math.abs(v).toFixed(2)}%`} />
                    </td>
                    <td className="px-2 py-1.5 font-mono tnum text-right text-[var(--color-loss)]">
                      {pos.stopLoss != null ? pos.stopLoss.toLocaleString("en-IN", { maximumFractionDigits: 2 }) : "—"}
                    </td>
                    <td className="px-2 py-1.5 font-mono tnum text-right text-[var(--color-profit)]">
                      {pos.target != null ? pos.target.toLocaleString("en-IN", { maximumFractionDigits: 2 }) : "—"}
                    </td>
                    <td className={cn("px-2 py-1.5 font-mono tnum whitespace-nowrap", ageColor(pos.ageMs))}>
                      {formatAge(pos.ageMs)}
                    </td>
                    <td className="px-2 py-1.5">
                      <button
                        type="button"
                        onClick={() => setCloseTarget(pos)}
                        className="text-[10px] text-[var(--color-loss)] hover:underline whitespace-nowrap"
                        aria-label={`Force close ${pos.symbol}`}
                      >
                        Force Close
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table></AutoSortTable>
          )}
        </div>
      </div>

      <ConfirmModal
        open={closeTarget !== null}
        onOpenChange={(v) => { if (!v) { setCloseTarget(null); setCloseTyped(""); } }}
        title="Force Close Position"
        description={
          closeTarget
            ? `Places a MARKET order to close ${closeTarget.size} ${closeTarget.symbol} at current price (~${closeTarget.currentPrice.toLocaleString("en-IN", { maximumFractionDigits: 2 })}). This bypasses strategy logic.`
            : ""
        }
        consequence={`Estimated fill at market price. Unrealized P&L: ${closeTarget ? (closeTarget.unrealizedPnl >= 0 ? "+" : "") + closeTarget.unrealizedPnl.toFixed(2) : "—"}. This bypasses all strategy exit logic.`}
        confirmLabel="Force Close"
        cancelLabel="Cancel"
        typeToConfirm="CLOSE"
        destructive
        onConfirm={forceClose}
        loading={closing}
      />
    </>
  );
}
