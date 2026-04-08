"use client";
import { useState } from "react";
import useAngelOneOrders, { type AngelOrder } from "@/hooks/useAngelOneOrders";

// ── Formatters ────────────────────────────────────────────────────────────────

function fmtINR(n: number) {
  return "₹" + n.toLocaleString("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

// ── Status badge ──────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: string }) {
  const s = status.toUpperCase();
  let cls = "border-zinc-600/50 bg-zinc-800/60 text-zinc-400";
  if (s === "COMPLETE") cls = "border-emerald-500/40 bg-emerald-500/10 text-emerald-300";
  else if (s === "REJECTED" || s === "CANCELLED") {
    cls = s === "REJECTED"
      ? "border-rose-500/40 bg-rose-500/10 text-rose-300"
      : "border-zinc-600/50 bg-zinc-800/60 text-zinc-400";
  } else if (s === "PENDING" || s === "OPEN" || s === "TRIGGER PENDING") {
    cls = "border-amber-500/40 bg-amber-500/10 text-amber-300";
  }
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider ${cls}`}>
      {status}
    </span>
  );
}

// ── Order row ─────────────────────────────────────────────────────────────────

function OrderRow({ order, onCancel: _onCancel }: { order: AngelOrder; onCancel: (id: string) => void }) {
  const isBuy = order.transactionType === "BUY";
  const placedDate = order.placedAt
    ? new Date(order.placedAt).toLocaleTimeString("en-IN", { hour: "2-digit", minute: "2-digit", second: "2-digit" })
    : "-";

  return (
    <tr className="group border-b border-zinc-800/50 transition-colors hover:bg-white/[0.02]">
      <td className="px-3 py-2 font-mono text-xs text-white font-semibold">{order.tradingSymbol}</td>
      <td className={`px-3 py-2 font-mono text-xs font-bold ${isBuy ? "text-emerald-400" : "text-rose-400"}`}>
        {order.transactionType}
      </td>
      <td className="px-3 py-2 font-mono text-xs text-zinc-300 text-right">{order.quantity}</td>
      <td className="px-3 py-2 font-mono text-xs text-zinc-300 text-right">
        {order.price === 0 ? "MKT" : fmtINR(order.price)}
      </td>
      <td className="px-3 py-2 font-mono text-xs text-zinc-300 text-right">
        {order.averagePrice === 0 ? "-" : fmtINR(order.averagePrice)}
      </td>
      <td className="px-3 py-2 text-xs"><StatusBadge status={order.status} /></td>
      <td className="px-3 py-2 font-mono text-xs text-zinc-500">{placedDate}</td>
      <td className="px-3 py-2" />
    </tr>
  );
}

// ── Main component ────────────────────────────────────────────────────────────

export default function AngelOneOrderPanel() {
  const { orders, funds, loading, error, refresh } = useAngelOneOrders();

  const [cancelError] = useState("");

  return (
    <div className="glass-panel space-y-4 p-5">

      {/* ── Header ── */}
      <div className="flex flex-wrap items-center gap-3 justify-between">
        <div>
          <div className="text-[10px] font-bold uppercase tracking-[0.18em]" style={{ color: "var(--text-muted)" }}>
            Angel One
          </div>
          <div className="mt-0.5 text-base font-bold text-white">Order Book (Read-Only)</div>
        </div>
        <span className="rounded-lg border border-zinc-600/50 bg-zinc-800/60 px-3 py-1.5 text-xs font-bold text-zinc-400 tracking-wide">
          PAPER TRADING ONLY
        </span>
        <button
          type="button"
          onClick={refresh}
          disabled={loading}
          className="btn-primary text-xs px-3 py-1.5 disabled:opacity-50"
        >
          {loading ? "Loading..." : "Refresh"}
        </button>
      </div>

      {/* ── Funds ── */}
      {funds && (
        <div className="grid grid-cols-3 gap-3">
          <div className="rounded-xl border border-zinc-700/60 bg-zinc-900/60 p-3 text-center">
            <div className="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Available Cash</div>
            <div className="mt-1 font-mono text-sm font-bold text-emerald-400">{fmtINR(funds.availableCash)}</div>
          </div>
          <div className="rounded-xl border border-zinc-700/60 bg-zinc-900/60 p-3 text-center">
            <div className="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Used Margin</div>
            <div className="mt-1 font-mono text-sm font-bold text-rose-400">{fmtINR(funds.usedMargin)}</div>
          </div>
          <div className="rounded-xl border border-zinc-700/60 bg-zinc-900/60 p-3 text-center">
            <div className="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Total</div>
            <div className="mt-1 font-mono text-sm font-bold text-white">{fmtINR(funds.availableCash + funds.usedMargin)}</div>
          </div>
        </div>
      )}

      {/* ── Error display ── */}
      {error && (
        <div className="rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-2 text-xs text-rose-300">
          {error}
        </div>
      )}
      {cancelError && (
        <div className="rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-2 text-xs text-rose-300">
          {cancelError}
        </div>
      )}

      {/* ── Order book (read-only) ── */}
      <div>
        <div className="text-[10px] font-bold uppercase tracking-widest text-zinc-500 mb-2">
          Order Book ({orders.length} orders)
        </div>
        {orders.length === 0 ? (
          <div className="rounded-xl border border-zinc-800/60 bg-zinc-900/40 px-4 py-6 text-center text-xs text-zinc-500">
            No orders found
          </div>
        ) : (
          <div className="overflow-x-auto rounded-xl border border-zinc-800/60">
            <table className="w-full border-collapse text-left" style={{ minWidth: 620 }}>
              <thead>
                <tr style={{ background: "var(--surface-2)", borderBottom: "1px solid var(--border)" }}>
                  {["Symbol", "Type", "Qty", "Price", "Avg Price", "Status", "Time"].map((h) => (
                    <th key={h} className="px-3 py-2 text-[10px] font-bold uppercase tracking-[0.15em]" style={{ color: "var(--text-muted)" }}>
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {orders.map((order) => (
                  <OrderRow key={order.orderId} order={order} onCancel={() => {}} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
