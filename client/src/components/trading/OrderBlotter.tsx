"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { cn } from "@/components/ui/cn";
import type { Order, OrderStatus } from "@/types/trading";

const STATUS_VARIANT: Record<OrderStatus, BadgeVariant> = {
  PENDING:      "caution",
  OPEN:         "info",
  PARTIAL_FILL: "warning",
  FILLED:       "profit",
  CANCELLED:    "neutral",
  REJECTED:     "loss",
};

function latencyColor(ms: number | null): string {
  if (ms === null) return "text-[var(--color-text-muted)]";
  if (ms < 50)  return "text-[var(--color-profit)]";
  if (ms < 200) return "text-[var(--color-caution)]";
  return "text-[var(--color-loss)]";
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  if (diff < 60_000)  return `${Math.floor(diff / 1000)}s ago`;
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)}m ago`;
  return `${Math.floor(diff / 3600_000)}h ago`;
}

type Tab = "all" | "open" | "filled" | "rejected" | "cancelled";

export function OrderBlotter() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<Tab>("all");
  const [cancelTarget, setCancelTarget] = useState<Order | null>(null);
  const [cancelling, setCancelling] = useState(false);

  const loadOrders = useCallback(async () => {
    try {
      const res = await fetch("/api/trading/orders", { cache: "no-store" });
      const data = await res.json();
      if (Array.isArray(data.orders)) setOrders(data.orders);
    } catch { /* engine offline */ }
    finally { setLoading(false); }
  }, []);

  useEffect(() => {
    loadOrders();
    const id = setInterval(loadOrders, 5_000);
    return () => clearInterval(id);
  }, [loadOrders]);

  const filteredOrders = useMemo(() => {
    if (tab === "all") return orders;
    if (tab === "open") return orders.filter((o) => o.status === "OPEN" || o.status === "PENDING" || o.status === "PARTIAL_FILL");
    return orders.filter((o) => o.status === tab.toUpperCase() as OrderStatus);
  }, [orders, tab]);

  const counts: Record<Tab, number> = useMemo(() => ({
    all:       orders.length,
    open:      orders.filter((o) => ["OPEN", "PENDING", "PARTIAL_FILL"].includes(o.status)).length,
    filled:    orders.filter((o) => o.status === "FILLED").length,
    rejected:  orders.filter((o) => o.status === "REJECTED").length,
    cancelled: orders.filter((o) => o.status === "CANCELLED").length,
  }), [orders]);

  const TABS: { key: Tab; label: string }[] = [
    { key: "all", label: "All" },
    { key: "open", label: "Open" },
    { key: "filled", label: "Filled" },
    { key: "rejected", label: "Rejected" },
    { key: "cancelled", label: "Cancelled" },
  ];

  async function cancelOrder() {
    if (!cancelTarget) return;
    setCancelling(true);
    try {
      await fetch(`/api/trading/orders`, {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ orderId: cancelTarget.orderId }),
      });
      await loadOrders();
    } finally {
      setCancelling(false);
      setCancelTarget(null);
    }
  }

  return (
    <>
      <div className="flex flex-col h-full">
        {/* Tabs */}
        <div className="flex items-center gap-0 border-b border-[var(--color-border)] px-3">
          {TABS.map((t) => (
            <button
              key={t.key}
              type="button"
              onClick={() => setTab(t.key)}
              className={cn(
                "flex items-center gap-1.5 px-3 py-2 text-[11px] font-medium border-b-2 transition-colors",
                tab === t.key
                  ? "border-[var(--color-info)] text-[var(--color-text-primary)]"
                  : "border-transparent text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]",
              )}
            >
              {t.label}
              {counts[t.key] > 0 && (
                <span className={cn(
                  "rounded-full text-[10px] px-1 min-w-[16px] text-center",
                  tab === t.key ? "bg-[var(--color-info)] text-white" : "bg-[var(--color-bg-elevated)] text-[var(--color-text-muted)]",
                )}>
                  {counts[t.key]}
                </span>
              )}
            </button>
          ))}
        </div>

        {/* Table */}
        <div className="flex-1 overflow-auto">
          {loading ? (
            <div className="p-4 space-y-2">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="skeleton h-8 rounded" />
              ))}
            </div>
          ) : filteredOrders.length === 0 ? (
            <div className="flex items-center justify-center h-full text-[12px] text-[var(--color-text-muted)]">
              No {tab === "all" ? "" : tab} orders
            </div>
          ) : (
            <table className="w-full" role="table">
              <thead className="sticky top-0 bg-[var(--color-bg-surface)]">
                <tr className="text-[10px] text-[var(--color-text-muted)]">
                  {["Order ID", "Symbol", "Strategy", "Dir", "Type", "Qty", "Price", "Status", "Fill%", "Latency", "Time", ""].map((h) => (
                    <th key={h} className="px-2 py-1.5 text-left font-medium whitespace-nowrap">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {filteredOrders.map((order) => (
                  <tr
                    key={order.orderId}
                    className="border-b border-[var(--color-border)] hover:bg-[var(--color-bg-elevated)] text-[11px]"
                  >
                    <td className="px-2 py-1.5 font-mono text-[10px] text-[var(--color-text-muted)]">
                      {order.orderId.slice(-8)}
                    </td>
                    <td className="px-2 py-1.5 font-mono text-[var(--color-text-primary)]">{order.symbol}</td>
                    <td className="px-2 py-1.5 text-[var(--color-text-secondary)] max-w-[100px] truncate">
                      {order.strategyId}
                    </td>
                    <td className="px-2 py-1.5">
                      <Badge variant={order.direction === "BUY" ? "profit" : "loss"} size="sm">{order.direction}</Badge>
                    </td>
                    <td className="px-2 py-1.5 text-[var(--color-text-secondary)]">{order.orderType}</td>
                    <td className="px-2 py-1.5 font-mono tnum text-right text-[var(--color-text-primary)]">
                      {order.quantity}
                    </td>
                    <td className="px-2 py-1.5 font-mono tnum text-right text-[var(--color-text-primary)]">
                      {order.price != null ? order.price.toLocaleString("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : "—"}
                    </td>
                    <td className="px-2 py-1.5">
                      <Badge variant={STATUS_VARIANT[order.status]} size="sm" dot={order.status === "OPEN"}>
                        {order.status}
                        {order.status === "PARTIAL_FILL"
                          ? ` ${Math.round((order.filledQuantity / order.quantity) * 100)}%`
                          : ""}
                      </Badge>
                    </td>
                    <td className="px-2 py-1.5 font-mono tnum text-right text-[var(--color-text-secondary)]">
                      {order.filledQuantity > 0
                        ? `${Math.round((order.filledQuantity / order.quantity) * 100)}%`
                        : "—"}
                    </td>
                    <td className={cn("px-2 py-1.5 font-mono tnum text-right", latencyColor(order.latencyMs))}>
                      {order.latencyMs != null ? `${order.latencyMs}ms` : "—"}
                    </td>
                    <td className="px-2 py-1.5 font-mono text-[10px] text-[var(--color-text-muted)] whitespace-nowrap">
                      {relativeTime(order.updatedAt)}
                    </td>
                    <td className="px-2 py-1.5">
                      {(order.status === "PENDING" || order.status === "OPEN") ? (
                        <button
                          type="button"
                          onClick={() => setCancelTarget(order)}
                          className="text-[10px] text-[var(--color-loss)] hover:underline"
                          aria-label={`Cancel order ${order.orderId}`}
                        >
                          Cancel
                        </button>
                      ) : null}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      <ConfirmModal
        open={cancelTarget !== null}
        onOpenChange={(v) => { if (!v) setCancelTarget(null); }}
        title="Cancel Order"
        description={cancelTarget ? `Cancel ${cancelTarget.direction} ${cancelTarget.quantity} ${cancelTarget.symbol} (${cancelTarget.orderType})?` : ""}
        consequence="This will send a cancel request to the exchange. Partially filled orders may have already executed some quantity."
        confirmLabel="Cancel Order"
        cancelLabel="Keep Order"
        destructive
        onConfirm={cancelOrder}
        loading={cancelling}
      />
    </>
  );
}
