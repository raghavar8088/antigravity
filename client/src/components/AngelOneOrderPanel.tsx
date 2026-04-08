"use client";
import { useState } from "react";
import useAngelOneOrders, { type PlaceOrderParams, type AngelOrder } from "@/hooks/useAngelOneOrders";

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

function OrderRow({ order, onCancel }: { order: AngelOrder; onCancel: (id: string) => void }) {
  const canCancel = ["OPEN", "PENDING", "TRIGGER PENDING"].includes(order.status.toUpperCase());
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
      <td className="px-3 py-2">
        {canCancel && (
          <button
            type="button"
            onClick={() => onCancel(order.orderId)}
            className="rounded border border-rose-500/40 bg-rose-500/10 px-2 py-0.5 text-[10px] font-semibold text-rose-300 hover:bg-rose-500/20 transition-colors"
          >
            Cancel
          </button>
        )}
      </td>
    </tr>
  );
}

// ── Order form ────────────────────────────────────────────────────────────────

type OrderForm = {
  tradingsymbol: string;
  symboltoken: string;
  exchange: "NFO" | "NSE" | "BSE";
  transactiontype: "BUY" | "SELL";
  ordertype: "MARKET" | "LIMIT";
  quantity: number;
  price: number;
  variety: "NORMAL" | "STOPLOSS" | "AMO";
  producttype: "INTRADAY" | "CARRYFORWARD";
};

const defaultForm: OrderForm = {
  tradingsymbol: "",
  symboltoken: "",
  exchange: "NFO",
  transactiontype: "BUY",
  ordertype: "MARKET",
  quantity: 1,
  price: 0,
  variety: "NORMAL",
  producttype: "INTRADAY",
};

// ── Main component ────────────────────────────────────────────────────────────

export default function AngelOneOrderPanel() {
  const { orders, funds, loading, error, refresh, placeOrder, cancelOrder } = useAngelOneOrders();

  const [formOpen, setFormOpen] = useState(false);
  const [form, setForm] = useState<OrderForm>(defaultForm);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [placing, setPlacing] = useState(false);
  const [placeResult, setPlaceResult] = useState<{ ok: boolean; message?: string; error?: string } | null>(null);
  const [cancelError, setCancelError] = useState("");

  const handleFieldChange = <K extends keyof OrderForm>(key: K, value: OrderForm[K]) => {
    setForm((f) => ({ ...f, [key]: value }));
  };

  const handlePlaceClick = (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.tradingsymbol || !form.symboltoken || form.quantity < 1) return;
    setConfirmOpen(true);
  };

  const handleConfirmPlace = async () => {
    setConfirmOpen(false);
    setPlacing(true);
    setPlaceResult(null);

    const params: PlaceOrderParams = {
      tradingsymbol: form.tradingsymbol,
      symboltoken: form.symboltoken,
      exchange: form.exchange,
      transactiontype: form.transactiontype,
      ordertype: form.ordertype,
      quantity: form.quantity,
      producttype: form.producttype,
      price: form.ordertype === "MARKET" ? "0" : String(form.price),
      triggerprice: "0",
      variety: form.variety,
    };

    const result = await placeOrder(params);
    setPlaceResult(result);
    setPlacing(false);

    if (result.ok) {
      setForm(defaultForm);
      setFormOpen(false);
    }
  };

  const handleCancel = async (orderId: string) => {
    setCancelError("");
    const result = await cancelOrder(orderId);
    if (!result.ok) {
      setCancelError(result.error ?? "Cancel failed");
    }
  };

  const estimatedValue = funds
    ? form.ordertype === "MARKET"
      ? null
      : form.price * form.quantity
    : null;

  return (
    <div className="glass-panel space-y-4 p-5">

      {/* ── Header ── */}
      <div className="flex flex-wrap items-center gap-3 justify-between">
        <div>
          <div className="text-[10px] font-bold uppercase tracking-[0.18em]" style={{ color: "var(--text-muted)" }}>
            Angel One Orders
          </div>
          <div className="mt-0.5 text-base font-bold text-white">Live Order Management</div>
        </div>
        {/* Live trading badge */}
        <span className="rounded-lg border border-amber-500/50 bg-amber-500/10 px-3 py-1.5 text-xs font-bold text-amber-300 tracking-wide">
          LIVE TRADING — REAL MONEY
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

      {/* ── Warning banner ── */}
      <div className="rounded-xl border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-xs font-semibold text-amber-300">
        WARNING: Orders executed here are REAL orders on Angel One. They use real money and execute immediately. Ensure you understand the risks before placing any order.
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
          Cancel error: {cancelError}
        </div>
      )}

      {/* ── Place order toggle ── */}
      <div>
        <button
          type="button"
          onClick={() => setFormOpen((v) => !v)}
          className="rounded-xl border border-zinc-700/60 bg-zinc-900/60 px-4 py-2 text-xs font-semibold text-zinc-300 hover:text-zinc-100 transition-colors"
        >
          {formOpen ? "Hide Order Form" : "Place New Order"}
        </button>
      </div>

      {/* ── Order form ── */}
      {formOpen && (
        <form onSubmit={handlePlaceClick} className="rounded-xl border border-zinc-700/60 bg-zinc-900/40 p-4 space-y-3">
          <div className="text-[10px] font-bold uppercase tracking-widest text-zinc-500 mb-2">New Order</div>

          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            {/* Symbol */}
            <div className="space-y-1">
              <label className="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Symbol</label>
              <input
                type="text"
                placeholder="NIFTY25APR23000CE"
                value={form.tradingsymbol}
                onChange={(e) => handleFieldChange("tradingsymbol", e.target.value.toUpperCase())}
                className="w-full rounded-lg border border-zinc-700/60 bg-zinc-800/80 px-3 py-1.5 font-mono text-xs text-white focus:border-sky-500/50 focus:outline-none"
                required
              />
            </div>
            {/* Token */}
            <div className="space-y-1">
              <label className="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Token</label>
              <input
                type="text"
                placeholder="12345"
                value={form.symboltoken}
                onChange={(e) => handleFieldChange("symboltoken", e.target.value)}
                className="w-full rounded-lg border border-zinc-700/60 bg-zinc-800/80 px-3 py-1.5 font-mono text-xs text-white focus:border-sky-500/50 focus:outline-none"
                required
              />
            </div>
            {/* Exchange */}
            <div className="space-y-1">
              <label className="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Exchange</label>
              <select
                value={form.exchange}
                onChange={(e) => handleFieldChange("exchange", e.target.value as "NFO" | "NSE" | "BSE")}
                className="w-full rounded-lg border border-zinc-700/60 bg-zinc-800/80 px-3 py-1.5 text-xs text-white focus:border-sky-500/50 focus:outline-none"
              >
                <option value="NFO">NFO</option>
                <option value="NSE">NSE</option>
                <option value="BSE">BSE</option>
              </select>
            </div>
            {/* Type */}
            <div className="space-y-1">
              <label className="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Type</label>
              <select
                value={form.transactiontype}
                onChange={(e) => handleFieldChange("transactiontype", e.target.value as "BUY" | "SELL")}
                className="w-full rounded-lg border border-zinc-700/60 bg-zinc-800/80 px-3 py-1.5 text-xs text-white focus:border-sky-500/50 focus:outline-none"
              >
                <option value="BUY">BUY</option>
                <option value="SELL">SELL</option>
              </select>
            </div>
            {/* Order Type */}
            <div className="space-y-1">
              <label className="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Order Type</label>
              <select
                value={form.ordertype}
                onChange={(e) => handleFieldChange("ordertype", e.target.value as "MARKET" | "LIMIT")}
                className="w-full rounded-lg border border-zinc-700/60 bg-zinc-800/80 px-3 py-1.5 text-xs text-white focus:border-sky-500/50 focus:outline-none"
              >
                <option value="MARKET">MARKET</option>
                <option value="LIMIT">LIMIT</option>
              </select>
            </div>
            {/* Product */}
            <div className="space-y-1">
              <label className="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Product</label>
              <select
                value={form.producttype}
                onChange={(e) => handleFieldChange("producttype", e.target.value as "INTRADAY" | "CARRYFORWARD")}
                className="w-full rounded-lg border border-zinc-700/60 bg-zinc-800/80 px-3 py-1.5 text-xs text-white focus:border-sky-500/50 focus:outline-none"
              >
                <option value="INTRADAY">INTRADAY</option>
                <option value="CARRYFORWARD">CARRYFORWARD</option>
              </select>
            </div>
            {/* Quantity */}
            <div className="space-y-1">
              <label className="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Quantity</label>
              <input
                type="number"
                min={1}
                step={1}
                value={form.quantity}
                onChange={(e) => handleFieldChange("quantity", Math.max(1, parseInt(e.target.value) || 1))}
                className="w-full rounded-lg border border-zinc-700/60 bg-zinc-800/80 px-3 py-1.5 font-mono text-xs text-white focus:border-sky-500/50 focus:outline-none"
                required
              />
            </div>
            {/* Price (shown for LIMIT) */}
            {form.ordertype === "LIMIT" && (
              <div className="space-y-1">
                <label className="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Price</label>
                <input
                  type="number"
                  min={0}
                  step={0.05}
                  value={form.price}
                  onChange={(e) => handleFieldChange("price", parseFloat(e.target.value) || 0)}
                  className="w-full rounded-lg border border-zinc-700/60 bg-zinc-800/80 px-3 py-1.5 font-mono text-xs text-white focus:border-sky-500/50 focus:outline-none"
                />
              </div>
            )}
          </div>

          {/* Place order button */}
          <div className="flex items-center gap-3 pt-1">
            <button
              type="submit"
              disabled={placing || !form.tradingsymbol || !form.symboltoken}
              className={`rounded-xl border px-6 py-2 text-xs font-bold uppercase tracking-wider transition-colors disabled:opacity-50 ${
                form.transactiontype === "BUY"
                  ? "border-emerald-500/50 bg-emerald-500/10 text-emerald-300 hover:bg-emerald-500/20"
                  : "border-rose-500/50 bg-rose-500/10 text-rose-300 hover:bg-rose-500/20"
              }`}
            >
              {placing ? "Placing..." : `Place ${form.transactiontype} Order`}
            </button>
            {placeResult && !placeResult.ok && (
              <span className="text-xs text-rose-400">{placeResult.error}</span>
            )}
            {placeResult && placeResult.ok && (
              <span className="text-xs text-emerald-400">{placeResult.message ?? "Order placed successfully"}</span>
            )}
          </div>
        </form>
      )}

      {/* ── Confirm dialog ── */}
      {confirmOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="glass-panel mx-4 max-w-md w-full p-6 space-y-4">
            <div className="text-base font-bold text-amber-300">Confirm Live Order</div>
            <div className="rounded-xl border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-xs text-amber-200">
              This is a LIVE order on Angel One. It will execute with REAL MONEY immediately.
              {estimatedValue !== null && (
                <span className="block mt-1 font-bold">Estimated value: {fmtINR(estimatedValue)}</span>
              )}
            </div>
            <div className="font-mono text-xs text-zinc-300 space-y-1">
              <div>Symbol: <span className="text-white font-bold">{form.tradingsymbol}</span></div>
              <div>Type: <span className={form.transactiontype === "BUY" ? "text-emerald-400 font-bold" : "text-rose-400 font-bold"}>{form.transactiontype}</span></div>
              <div>Quantity: <span className="text-white font-bold">{form.quantity}</span></div>
              <div>Order Type: <span className="text-white font-bold">{form.ordertype}</span></div>
              {form.ordertype === "LIMIT" && <div>Price: <span className="text-white font-bold">{fmtINR(form.price)}</span></div>}
            </div>
            <div className="flex gap-3">
              <button
                type="button"
                onClick={handleConfirmPlace}
                className="flex-1 rounded-xl border border-rose-500/50 bg-rose-500/10 px-4 py-2 text-xs font-bold text-rose-300 hover:bg-rose-500/20 transition-colors"
              >
                Confirm — Place Order
              </button>
              <button
                type="button"
                onClick={() => setConfirmOpen(false)}
                className="flex-1 rounded-xl border border-zinc-700/60 bg-zinc-900/60 px-4 py-2 text-xs font-semibold text-zinc-300 hover:text-zinc-100 transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ── Order book ── */}
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
            <table className="w-full border-collapse text-left" style={{ minWidth: 680 }}>
              <thead>
                <tr style={{ background: "var(--surface-2)", borderBottom: "1px solid var(--border)" }}>
                  {["Symbol", "Type", "Qty", "Price", "Avg Price", "Status", "Time", ""].map((h) => (
                    <th key={h} className="px-3 py-2 text-[10px] font-bold uppercase tracking-[0.15em]" style={{ color: "var(--text-muted)" }}>
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {orders.map((order) => (
                  <OrderRow key={order.orderId} order={order} onCancel={handleCancel} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
