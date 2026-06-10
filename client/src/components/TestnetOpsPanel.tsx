"use client";

import { useCallback, useEffect, useState } from "react";
import { submitExecutionRequest } from "@/lib/executionRequest";
import { DeskBanner } from "@/components/desk/ui/DeskBanner";
import { DeskButton } from "@/components/desk/ui/DeskButton";
import { usePaperDeskAuth } from "@/hooks/usePaperDeskAuth";

type BalanceSnippet = { asset: string; availableBalance: number };

type OpenOrderRow = {
  orderId: string;
  symbol: string;
  side: string;
  size: number;
  state: string;
};

type PositionRow = {
  symbol: string;
  side: string;
  size: number;
  unrealisedPnl: number;
};

const DEFAULT_SYMBOL = "BTCUSD";
const DEFAULT_SIZE = 1;

async function readJson(res: Response): Promise<Record<string, unknown>> {
  try {
    return (await res.json()) as Record<string, unknown>;
  } catch {
    return {};
  }
}

export function TestnetOpsPanel() {
  const { user, configured, loading: authLoading } = usePaperDeskAuth();
  const [balances, setBalances] = useState<BalanceSnippet[] | null>(null);
  const [positions, setPositions] = useState<PositionRow[]>([]);
  const [openOrders, setOpenOrders] = useState<OpenOrderRow[]>([]);
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [side, setSide] = useState<"buy" | "sell">("buy");
  const [size, setSize] = useState(String(DEFAULT_SIZE));
  const [orderType, setOrderType] = useState<"market" | "limit">("market");
  const [limitPrice, setLimitPrice] = useState("");
  const [cancelId, setCancelId] = useState("");

  const refresh = useCallback(async () => {
    if (!user) return;
    setBusy(true);
    setMessage(null);
    try {
      const [pingRes, posRes] = await Promise.all([
        fetch("/api/delta/testnet/ping", { method: "POST", credentials: "include", cache: "no-store" }),
        fetch("/api/delta/testnet/positions", { credentials: "include", cache: "no-store" }),
      ]);
      const ping = await readJson(pingRes);
      const pos = await readJson(posRes);

      if (pingRes.ok && ping.ok) {
        setBalances((ping.balanceSnippet as BalanceSnippet[]) ?? []);
      } else {
        setBalances(null);
        setMessage(String(ping.error ?? `Ping failed (${pingRes.status})`));
      }

      if (posRes.ok && pos.ok) {
        setPositions((pos.positions as PositionRow[]) ?? []);
        setOpenOrders((pos.openOrders as OpenOrderRow[]) ?? []);
      } else if (!message) {
        setMessage(String(pos.error ?? `Positions failed (${posRes.status})`));
      }
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Refresh failed");
    } finally {
      setBusy(false);
    }
  }, [user]);

  useEffect(() => {
    if (user) void refresh();
  }, [user, refresh]);

  const placeOrder = async () => {
    if (!user) return;
    setBusy(true);
    setMessage(null);
    try {
      const result = await submitExecutionRequest({
        venue: "delta",
        symbol: DEFAULT_SYMBOL,
        side,
        contracts: Number(size),
        strategyName: "TESTNET_OPS",
        reason: "desk_testnet_panel",
      });
      if (!result.ok) {
        setMessage(result.message ?? "Execution request rejected");
        return;
      }
      setMessage(`Request ${result.status}: ${result.clientOrderId ?? result.requestId ?? "accepted"}`);
      await refresh();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Request failed");
    } finally {
      setBusy(false);
    }
  };

  const cancelOrder = async (_orderId: string) => {
    setMessage("Direct testnet cancel from UI is disabled — use institutional OMS cancel when available");
  };

  if (authLoading) {
    return (
      <div className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950">
        Loading testnet ops…
      </div>
    );
  }

  if (!configured) {
    return (
      <div className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950">
        Testnet ops require Supabase auth env.
      </div>
    );
  }

  if (!user) {
    return (
      <div className="rounded-lg border-2 border-amber-500 bg-amber-50 p-4">
        <p className="text-center text-sm font-bold uppercase tracking-wide text-amber-900">
          Testnet only — sign in to use manual orders
        </p>
      </div>
    );
  }

  return (
    <div>
      <DeskBanner variant="warning" title="TESTNET ONLY">
        Manual Delta testnet orders only. Not paper desk. Not mainnet.
      </DeskBanner>

      <div className="mb-3 flex flex-wrap items-center gap-2" style={{ marginTop: 16 }}>
        <DeskButton variant="outlined" disabled={busy} onClick={() => void refresh()} style={{ minHeight: 36 }}>
          {busy ? "…" : "Refresh"}
        </DeskButton>
        {balances?.length ? (
          <span className="text-[11px] text-amber-900">
            {balances.map((b) => `${b.asset}: ${b.availableBalance}`).join(" · ")}
          </span>
        ) : (
          <span className="text-[11px] text-amber-800">No balance snippet</span>
        )}
      </div>

      <div className="mb-3 grid gap-2 rounded border border-amber-200 bg-white/80 p-3 sm:grid-cols-2">
        <div className="text-[11px] font-semibold text-amber-950">
          Place small {DEFAULT_SYMBOL} test order
        </div>
        <div className="flex flex-wrap gap-2 sm:col-span-2">
          <select
            value={side}
            onChange={(e) => setSide(e.target.value as "buy" | "sell")}
            className="rounded border border-amber-200 px-2 py-1 text-[11px]"
          >
            <option value="buy">Buy</option>
            <option value="sell">Sell</option>
          </select>
          <input
            type="number"
            min={0.001}
            step={0.001}
            value={size}
            onChange={(e) => setSize(e.target.value)}
            className="w-20 rounded border border-amber-200 px-2 py-1 text-[11px]"
            aria-label="Size"
          />
          <select
            value={orderType}
            onChange={(e) => setOrderType(e.target.value as "market" | "limit")}
            className="rounded border border-amber-200 px-2 py-1 text-[11px]"
          >
            <option value="market">Market</option>
            <option value="limit">Limit</option>
          </select>
          {orderType === "limit" ? (
            <input
              type="number"
              value={limitPrice}
              onChange={(e) => setLimitPrice(e.target.value)}
              placeholder="Limit price"
              className="w-28 rounded border border-amber-200 px-2 py-1 text-[11px]"
            />
          ) : null}
          <button
            type="button"
            disabled={busy}
            onClick={() => void placeOrder()}
            className="rounded border border-amber-600 bg-amber-600 px-3 py-1 text-[11px] font-semibold text-white hover:bg-amber-700 disabled:opacity-50"
          >
            Place
          </button>
        </div>
      </div>

      <div className="mb-3 rounded border border-amber-200 bg-white/80 p-3">
        <div className="mb-2 text-[10px] font-semibold uppercase text-amber-800">Open orders</div>
        {openOrders.length === 0 ? (
          <p className="text-[11px] text-amber-800">None</p>
        ) : (
          <ul className="space-y-1 text-[11px] text-amber-950">
            {openOrders.map((o) => (
              <li key={o.orderId} className="flex flex-wrap items-center justify-between gap-2">
                <span>
                  #{o.orderId} {o.symbol} {o.side} ×{o.size} ({o.state})
                </span>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => void cancelOrder(o.orderId)}
                  className="rounded border border-rose-300 bg-rose-50 px-2 py-0.5 text-[10px] font-medium text-rose-800 hover:bg-rose-100"
                >
                  Cancel
                </button>
              </li>
            ))}
          </ul>
        )}
        <div className="mt-2 flex flex-wrap gap-2">
          <input
            value={cancelId}
            onChange={(e) => setCancelId(e.target.value)}
            placeholder="Order id to cancel"
            className="min-w-[120px] flex-1 rounded border border-amber-200 px-2 py-1 text-[11px]"
          />
          <button
            type="button"
            disabled={busy || !cancelId.trim()}
            onClick={() => void cancelOrder(cancelId)}
            className="rounded border border-rose-300 bg-rose-50 px-2 py-1 text-[10px] font-medium text-rose-800 disabled:opacity-50"
          >
            Cancel id
          </button>
        </div>
      </div>

      {positions.length > 0 ? (
        <div className="mb-2 text-[11px] text-amber-900">
          Positions:{" "}
          {positions.map((p) => `${p.symbol} ${p.side} ${p.size} uPnL ${p.unrealisedPnl}`).join(" · ")}
        </div>
      ) : null}

      {message ? <p className="text-[11px] text-amber-950">{message}</p> : null}
    </div>
  );
}
