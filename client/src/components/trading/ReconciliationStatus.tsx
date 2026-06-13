"use client";

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/Badge";
import { cn } from "@/components/ui/cn";
import type { ReconciliationState, ReconciliationStatus as RecStatus } from "@/types/trading";

const STATUS_VARIANT = {
  IN_SYNC:         { variant: "profit"  as const, label: "IN SYNC"  },
  DRIFT_DETECTED:  { variant: "warning" as const, label: "DRIFT"    },
  MISMATCH:        { variant: "loss"    as const, label: "MISMATCH" },
};

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  if (diff < 60_000) return `${Math.floor(diff / 1000)}s ago`;
  return `${Math.floor(diff / 60_000)}m ago`;
}

export function ReconciliationStatus() {
  const [state, setState] = useState<ReconciliationState | null>(null);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    const load = async () => {
      try {
        const res = await fetch("/api/engine/reconciliation", { cache: "no-store" });
        if (res.ok) setState(await res.json());
      } catch { /* engine offline */ }
      finally { setLoading(false); }
    };
    load();
    const id = setInterval(load, 10_000);
    return () => clearInterval(id);
  }, []);

  if (loading) {
    return (
      <div className="p-3 space-y-2">
        {Array.from({ length: 3 }).map((_, i) => <div key={i} className="skeleton h-6 rounded" />)}
      </div>
    );
  }

  if (!state) {
    return (
      <div className="flex items-center gap-2 p-3 text-[12px] text-[var(--color-warning)]">
        <span className="w-[6px] h-[6px] rounded-full bg-[var(--color-warning)]" />
        Reconciliation data unavailable — engine may be offline
      </div>
    );
  }

  const { variant, label } = STATUS_VARIANT[state.status] ?? STATUS_VARIANT.IN_SYNC;

  return (
    <div className="flex flex-col gap-2 p-3">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-[12px] font-medium text-[var(--color-text-secondary)]">Reconciliation</span>
          <Badge variant={variant} size="sm" dot>{label}</Badge>
        </div>
        <span className="text-[10px] text-[var(--color-text-muted)]">
          Last sync {relativeTime(state.lastSyncAt)}
        </span>
      </div>

      {/* Three-column sync overview */}
      <div className="grid grid-cols-3 gap-2">
        {(["Exchange", "OMS", "Ledger"] as const).map((col) => (
          <div key={col} className="rounded border border-[var(--color-border)] p-2 text-center">
            <div className="text-[10px] text-[var(--color-text-muted)] mb-1">{col}</div>
            <div className={cn(
              "w-[8px] h-[8px] rounded-full mx-auto",
              state.status === "IN_SYNC" ? "bg-[var(--color-profit)]" : "bg-[var(--color-warning)]",
            )} />
          </div>
        ))}
      </div>

      {/* Mismatch counter */}
      {state.mismatchCount > 0 ? (
        <div>
          <button
            type="button"
            onClick={() => setExpanded((v) => !v)}
            className="flex items-center gap-2 text-[11px] text-[var(--color-warning)] hover:underline"
          >
            ⚠ {state.mismatchCount} mismatch{state.mismatchCount > 1 ? "es" : ""} detected
            <span className="text-[10px]">{expanded ? "▲" : "▼"}</span>
          </button>

          {expanded ? (
            <div className="mt-2 space-y-1">
              {state.driftItems.map((item, i) => (
                <div key={i} className="flex items-center justify-between text-[10px] border-b border-[var(--color-border)] pb-1">
                  <span className="font-mono text-[var(--color-text-primary)]">{item.symbol}</span>
                  <div className="flex items-center gap-2 text-[var(--color-text-muted)]">
                    <span>Ex:{item.exchangeQty}</span>
                    <span>OMS:{item.omsQty}</span>
                    <span>Ledger:{item.ledgerQty}</span>
                    {item.resolvedAutomatically ? (
                      <Badge variant="profit" size="sm">Auto-fixed</Badge>
                    ) : (
                      <Badge variant="loss" size="sm">Manual</Badge>
                    )}
                  </div>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      ) : (
        <div className="text-[11px] text-[var(--color-profit)] flex items-center gap-1.5">
          <span className="w-[5px] h-[5px] rounded-full bg-[var(--color-profit)]" />
          Exchange ↔ OMS ↔ Ledger in sync
        </div>
      )}
    </div>
  );
}
