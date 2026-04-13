"use client";
import { useState } from "react";
import useDeltaLive, {
  type DeltaLiveTrade,
  type DeltaLiveStats,
  type DeltaRuntimeStatus,
  type WalletEntry,
  type LivePosition,
  type OpenOrder,
} from "@/hooks/useDeltaLive";

type Props = { actionsEnabled?: boolean };

function fmt(n: number, dp = 2) {
  return n.toLocaleString("en-US", { minimumFractionDigits: dp, maximumFractionDigits: dp });
}
function fmtTime(iso: string) {
  try { return new Date(iso).toLocaleTimeString(); } catch { return iso; }
}
function fmtDate(iso: string) {
  try { return new Date(iso).toLocaleString(); } catch { return iso; }
}

function pnlColor(v: number) { return v >= 0 ? "text-green-400" : "text-red-400"; }
function pnlSign(v: number) { return v >= 0 ? "+" : ""; }

// ─── Status badge ───────────────────────────────────────────────────────────
function StatusBadge({ status }: { status: DeltaLiveTrade["status"] }) {
  const s: Record<string, string> = {
    OPEN:      "bg-blue-900 text-blue-200",
    CLOSED:    "bg-green-900 text-green-200",
    FAILED:    "bg-red-900 text-red-200",
    CANCELLED: "bg-gray-700 text-gray-300",
  };
  return <span className={`px-2 py-0.5 rounded text-xs font-bold ${s[status] ?? "bg-gray-700 text-gray-300"}`}>{status}</span>;
}

// ─── Enable/Disable banner ───────────────────────────────────────────────────
function RuntimeStatusPill({ label, configured, detail }: { label: string; configured: boolean; detail: string }) {
  return (
    <div className={`rounded-lg border px-3 py-2 text-xs ${
      configured ? "border-emerald-600/40 bg-emerald-900/20 text-emerald-200" : "border-red-700/40 bg-red-900/20 text-red-200"
    }`}>
      <div className="font-semibold">{label}: {configured ? "Configured" : "Not Configured"}</div>
      <div className="mt-1 opacity-80">{detail}</div>
    </div>
  );
}

function EnableBanner({
  stats,
  nextStatus,
  toggling,
  onToggle,
}: {
  stats: DeltaLiveStats;
  nextStatus: DeltaRuntimeStatus;
  toggling: boolean;
  onToggle: (v: boolean) => void;
}) {
  if (!stats.configured || !nextStatus.configured) {
    return (
      <div className="bg-yellow-900/40 border border-yellow-600 rounded-xl p-4">
        <div className="flex items-start gap-3">
          <span className="text-yellow-400 text-2xl">⚠️</span>
          <div className="flex-1">
            <div className="text-yellow-300 font-bold mb-1">Delta Runtime Configuration Check</div>
            <div className="text-yellow-200/80 text-xs mb-3">
              This screen depends on two different runtimes. The Go engine powers live mirroring, while Vercel / Next.js powers the test-order routes.
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              <RuntimeStatusPill
                label="Go Engine"
                configured={stats.configured}
                detail={stats.configured ? `Ready${stats.testnet ? " · testnet" : " · production"}` : "Set DELTA_API_KEY and DELTA_API_SECRET on the Go backend server"}
              />
              <RuntimeStatusPill
                label="Vercel / Next.js"
                configured={nextStatus.configured}
                detail={nextStatus.configured ? `Ready${nextStatus.testnet ? " · testnet" : " · production"}` : "Set DELTA_API_KEY and DELTA_API_SECRET in Vercel project env vars"}
              />
            </div>
            <div className="bg-black/50 rounded-lg p-3 font-mono text-xs space-y-1 mt-3">
              <div className="text-green-300">DELTA_API_KEY=<span className="text-gray-400">your_api_key</span></div>
              <div className="text-green-300">DELTA_API_SECRET=<span className="text-gray-400">your_api_secret</span></div>
              <div className="text-gray-500"># Optional testnet</div>
              <div className="text-gray-400">DELTA_TESTNET=true</div>
            </div>
            <div className="text-yellow-200/60 text-xs mt-2">API keys: <span className="text-yellow-300">india.delta.exchange → Settings → API Keys</span></div>
            {nextStatus.error && (
              <div className="text-yellow-200/80 text-xs mt-3">
                Vercel / Next.js message: {nextStatus.error}
              </div>
            )}
          </div>
        </div>
      </div>
    );
  }
  return (
    <div className={`border rounded-xl p-4 flex items-center justify-between ${stats.enabled ? "bg-green-900/20 border-green-600" : "bg-gray-800/60 border-gray-600"}`}>
      <div className="flex items-center gap-3">
        <div className={`w-3 h-3 rounded-full ${stats.enabled ? "bg-green-400 animate-pulse" : "bg-gray-500"}`} />
        <div>
          <div className="font-bold text-sm text-white">Live Order Mirroring {stats.enabled ? "ACTIVE" : "PAUSED"}</div>
          <div className="text-xs text-gray-400">
            {stats.testnet ? "🧪 Testnet" : "🔴 Production — real money"}&nbsp;·&nbsp;
            Enable this to mirror BTC Option Selling paper positions to Delta.
          </div>
        </div>
      </div>
      <button
        type="button"
        disabled={toggling}
        onClick={() => onToggle(!stats.enabled)}
        className={`px-5 py-2 rounded-lg text-sm font-bold transition-colors disabled:opacity-50 ${
          stats.enabled ? "bg-red-700 hover:bg-red-600 text-white" : "bg-green-700 hover:bg-green-600 text-white"
        }`}
      >
        {toggling ? "..." : stats.enabled ? "⏸ Disable" : "▶ Enable"}
      </button>
    </div>
  );
}

// ─── Wallet cards ────────────────────────────────────────────────────────────
function WalletCards({ wallets }: { wallets: WalletEntry[] }) {
  if (!wallets?.length) return <div className="text-gray-500 text-sm text-center py-4">No wallet data</div>;
  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
      {wallets.map((w) => (
        <div key={w.asset} className="bg-gray-800 rounded-xl p-3 border border-gray-700">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-bold text-gray-300 bg-gray-700 px-2 py-0.5 rounded">{w.asset}</span>
            {w.unrealisedPnl !== 0 && (
              <span className={`text-xs font-semibold ${pnlColor(w.unrealisedPnl)}`}>
                {pnlSign(w.unrealisedPnl)}{fmt(w.unrealisedPnl)} uPnL
              </span>
            )}
          </div>
          <div className="text-lg font-bold text-white mb-1">{fmt(w.balance, 4)}</div>
          <div className="space-y-0.5">
            <div className="flex justify-between text-xs">
              <span className="text-gray-400">Available</span>
              <span className="text-green-300 font-medium">{fmt(w.availableBalance, 4)}</span>
            </div>
            {w.blockedBalance > 0 && (
              <div className="flex justify-between text-xs">
                <span className="text-gray-400">In Margin</span>
                <span className="text-orange-300">{fmt(w.blockedBalance, 4)}</span>
              </div>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

// ─── Live positions on Delta ─────────────────────────────────────────────────
function LivePositionsTable({ positions }: { positions: LivePosition[] }) {
  if (!positions?.length) {
    return <div className="text-gray-500 text-sm text-center py-6">No open positions on Delta Exchange</div>;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="text-gray-400 border-b border-gray-700">
            <th className="px-3 py-2 text-left">Symbol</th>
            <th className="px-3 py-2 text-left">Side</th>
            <th className="px-3 py-2 text-right">Size</th>
            <th className="px-3 py-2 text-right">Entry</th>
            <th className="px-3 py-2 text-right">Mark</th>
            <th className="px-3 py-2 text-right">uPnL</th>
            <th className="px-3 py-2 text-right">rPnL</th>
            <th className="px-3 py-2 text-right">Margin</th>
          </tr>
        </thead>
        <tbody>
          {positions.map((p, i) => (
            <tr key={i} className="border-b border-gray-800 hover:bg-gray-800/40">
              <td className="px-3 py-2 font-mono text-blue-300 font-bold">{p.symbol}</td>
              <td className="px-3 py-2">
                <span className={`font-bold ${p.side === "LONG" ? "text-green-400" : "text-red-400"}`}>{p.side}</span>
              </td>
              <td className="px-3 py-2 text-right text-white">{fmt(Math.abs(p.size), 0)}</td>
              <td className="px-3 py-2 text-right text-white">${fmt(p.entryPrice, 2)}</td>
              <td className="px-3 py-2 text-right text-gray-300">${fmt(p.markPrice, 2)}</td>
              <td className={`px-3 py-2 text-right font-semibold ${pnlColor(p.unrealisedPnl)}`}>
                {pnlSign(p.unrealisedPnl)}${fmt(p.unrealisedPnl)}
              </td>
              <td className={`px-3 py-2 text-right ${pnlColor(p.realisedPnl)}`}>
                {pnlSign(p.realisedPnl)}${fmt(p.realisedPnl)}
              </td>
              <td className="px-3 py-2 text-right text-orange-300">${fmt(p.margin)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Open orders on Delta ─────────────────────────────────────────────────────
function OpenOrdersTable({ orders }: { orders: OpenOrder[] }) {
  if (!orders?.length) {
    return <div className="text-gray-500 text-sm text-center py-4">No open orders</div>;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="text-gray-400 border-b border-gray-700">
            <th className="px-3 py-2 text-left">Order ID</th>
            <th className="px-3 py-2 text-left">Symbol</th>
            <th className="px-3 py-2 text-left">Side</th>
            <th className="px-3 py-2 text-right">Size</th>
            <th className="px-3 py-2 text-right">Price</th>
            <th className="px-3 py-2 text-left">State</th>
            <th className="px-3 py-2 text-left">Time</th>
          </tr>
        </thead>
        <tbody>
          {orders.map((o) => (
            <tr key={o.orderId} className="border-b border-gray-800 hover:bg-gray-800/40">
              <td className="px-3 py-2 font-mono text-gray-400 text-xs">{o.orderId}</td>
              <td className="px-3 py-2 text-blue-300 font-bold">{o.symbol}</td>
              <td className="px-3 py-2">
                <span className={`font-bold ${o.side === "buy" ? "text-green-400" : "text-red-400"}`}>{o.side.toUpperCase()}</span>
              </td>
              <td className="px-3 py-2 text-right text-white">{fmt(o.size, 0)}</td>
              <td className="px-3 py-2 text-right text-white">${fmt(o.price, 2)}</td>
              <td className="px-3 py-2 text-yellow-300 text-xs font-medium">{o.state}</td>
              <td className="px-3 py-2 text-gray-400">{fmtTime(o.createdAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Mirrored trades table ────────────────────────────────────────────────────
function MirroredTradesTable({ trades }: { trades: DeltaLiveTrade[] }) {
  if (!trades.length) {
    return (
      <div className="text-center py-10 text-gray-500 text-sm">
        No Delta trade records are available on the server.
      </div>
    );
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="text-gray-400 border-b border-gray-700">
            <th className="px-3 py-2 text-left">ID</th>
            <th className="px-3 py-2 text-left">Strategy</th>
            <th className="px-3 py-2 text-left">Type</th>
            <th className="px-3 py-2 text-right">Strike</th>
            <th className="px-3 py-2 text-left">Delta Symbol</th>
            <th className="px-3 py-2 text-right">Qty</th>
            <th className="px-3 py-2 text-right">Fill $</th>
            <th className="px-3 py-2 text-right">PnL</th>
            <th className="px-3 py-2 text-left">Status</th>
            <th className="px-3 py-2 text-left">Opened</th>
          </tr>
        </thead>
        <tbody>
          {trades.map((t) => (
            <tr key={t.id} className="border-b border-gray-800 hover:bg-gray-800/40">
              <td className="px-3 py-2 font-mono text-gray-300">{t.id}</td>
              <td className="px-3 py-2 text-gray-200 max-w-[120px] truncate">{t.strategyName}</td>
              <td className="px-3 py-2">
                <span className={`font-bold ${t.optionType === "CALL" ? "text-green-400" : "text-red-400"}`}>{t.optionType}</span>
              </td>
              <td className="px-3 py-2 text-right text-white">${fmt(t.strike, 0)}</td>
              <td className="px-3 py-2 font-mono text-blue-300">{t.deltaSymbol || "—"}</td>
              <td className="px-3 py-2 text-right text-white">{t.contracts}</td>
              <td className="px-3 py-2 text-right text-white">${fmt(t.fillPrice, 4)}</td>
              <td className="px-3 py-2 text-right">
                {t.status === "CLOSED" && t.realizedPnl != null ? (
                  <span className={`font-semibold ${pnlColor(t.realizedPnl)}`}>
                    {pnlSign(t.realizedPnl)}${fmt(t.realizedPnl)}
                  </span>
                ) : t.status === "FAILED" ? (
                  <span className="text-red-400 text-xs truncate max-w-[100px]" title={t.failureReason}>ERR</span>
                ) : <span className="text-gray-500">—</span>}
              </td>
              <td className="px-3 py-2"><StatusBadge status={t.status} /></td>
              <td className="px-3 py-2 text-gray-400">{fmtTime(t.openedAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Main component ───────────────────────────────────────────────────────────
type TestOrderResponse = {
  ok: boolean;
  error?: string;
  orderId?: string;
  closeOrderId?: string;
  symbol?: string;
  productId?: number;
  contracts?: number;
  fillPrice?: number;
  closeFillPrice?: number;
  state?: string;
};

function TestOrderTab({
  actionsEnabled,
  positions,
  onOrderPlaced,
}: {
  actionsEnabled: boolean;
  positions: LivePosition[];
  onOrderPlaced: () => void;
}) {
  const [optionType, setOptionType] = useState<"CALL" | "PUT">("CALL");
  const [strike, setStrike] = useState("120000");
  const [premiumUsd, setPremiumUsd] = useState("100");
  const [closeProductId, setCloseProductId] = useState("");
  const [closeContracts, setCloseContracts] = useState("1");
  const [submitting, setSubmitting] = useState<"open" | "close" | null>(null);
  const [feedback, setFeedback] = useState<{ tone: "success" | "error"; text: string } | null>(null);
  const [lastResult, setLastResult] = useState<TestOrderResponse | null>(null);

  const submitOpen = async () => {
    setSubmitting("open");
    setFeedback(null);
    try {
      const response = await fetch("/api/delta/mirror", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "open",
          optionType,
          strike: Number(strike),
          premiumUsd: Number(premiumUsd),
        }),
      });
      const data = await response.json() as TestOrderResponse;
      setLastResult(data);
      if (response.ok && data.ok) {
        setFeedback({
          tone: "success",
          text: `Open order placed on Delta. Order ID ${data.orderId ?? "-"}${data.symbol ? ` | ${data.symbol}` : ""}`,
        });
        onOrderPlaced();
      } else {
        setFeedback({ tone: "error", text: data.error ?? "Failed to place Delta open order." });
      }
    } catch (error) {
      setFeedback({ tone: "error", text: String(error) });
    } finally {
      setSubmitting(null);
    }
  };

  const submitClose = async () => {
    setSubmitting("close");
    setFeedback(null);
    try {
      const response = await fetch("/api/delta/mirror", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "close",
          productId: Number(closeProductId),
          contracts: Number(closeContracts),
        }),
      });
      const data = await response.json() as TestOrderResponse;
      setLastResult(data);
      if (response.ok && data.ok) {
        setFeedback({
          tone: "success",
          text: `Close order placed on Delta. Order ID ${data.closeOrderId ?? "-"} | ${Number(closeContracts) || 0} contract(s)`,
        });
        onOrderPlaced();
      } else {
        setFeedback({ tone: "error", text: data.error ?? "Failed to place Delta close order." });
      }
    } catch (error) {
      setFeedback({ tone: "error", text: String(error) });
    } finally {
      setSubmitting(null);
    }
  };

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-amber-700/40 bg-amber-900/10 p-4 text-xs text-amber-200">
        This tab sends live test orders to Delta Exchange. Use small size and prefer testnet first.
      </div>

      {!actionsEnabled && (
        <div className="rounded-xl border border-gray-700 bg-gray-800/60 p-3 text-xs text-gray-400">
          Action buttons are disabled. Turn Action to Yes to use this test order panel.
        </div>
      )}

      <div className="grid gap-4 xl:grid-cols-2">
        <div className="rounded-xl border border-gray-800 bg-gray-950/70 p-4 space-y-4">
          <div>
            <div className="text-sm font-semibold text-white">Open Test Sell Order</div>
            <div className="text-xs text-gray-400 mt-1">Finds the nearest Delta option contract and sends a market sell order.</div>
          </div>

          <div className="grid gap-3 sm:grid-cols-3">
            <label className="space-y-1">
              <span className="text-xs text-gray-400">Option Type</span>
              <select
                value={optionType}
                onChange={(e) => setOptionType(e.target.value as "CALL" | "PUT")}
                className="w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-white outline-none"
              >
                <option value="CALL">CALL</option>
                <option value="PUT">PUT</option>
              </select>
            </label>
            <label className="space-y-1">
              <span className="text-xs text-gray-400">Strike</span>
              <input
                value={strike}
                onChange={(e) => setStrike(e.target.value)}
                inputMode="decimal"
                className="w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-white outline-none"
                placeholder="120000"
              />
            </label>
            <label className="space-y-1">
              <span className="text-xs text-gray-400">Premium USD</span>
              <input
                value={premiumUsd}
                onChange={(e) => setPremiumUsd(e.target.value)}
                inputMode="decimal"
                className="w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-white outline-none"
                placeholder="100"
              />
            </label>
          </div>

          <button
            type="button"
            disabled={!actionsEnabled || submitting !== null}
            onClick={() => void submitOpen()}
            className="rounded-lg bg-red-700 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-red-600 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {submitting === "open" ? "Placing..." : "Place Open Test Order"}
          </button>
        </div>

        <div className="rounded-xl border border-gray-800 bg-gray-950/70 p-4 space-y-4">
          <div>
            <div className="text-sm font-semibold text-white">Close Test Order</div>
            <div className="text-xs text-gray-400 mt-1">Sends a market buy order for a Delta option product ID to close a short test position.</div>
          </div>

          {positions.length > 0 && (
            <div className="space-y-2">
              <div className="text-xs text-gray-400">Quick fill from live Delta positions</div>
              <div className="flex flex-wrap gap-2">
                {positions.slice(0, 6).map((position) => (
                  <button
                    key={`${position.productId}-${position.symbol}`}
                    type="button"
                    onClick={() => {
                      setCloseProductId(String(position.productId));
                      setCloseContracts(String(Math.max(1, Math.round(Math.abs(position.size)))));
                    }}
                    className="rounded-full border border-gray-700 bg-gray-900 px-3 py-1 text-xs text-blue-200 hover:border-blue-500"
                  >
                    {position.symbol} | ID {position.productId}
                  </button>
                ))}
              </div>
            </div>
          )}

          <div className="grid gap-3 sm:grid-cols-2">
            <label className="space-y-1">
              <span className="text-xs text-gray-400">Product ID</span>
              <input
                value={closeProductId}
                onChange={(e) => setCloseProductId(e.target.value)}
                inputMode="numeric"
                className="w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-white outline-none"
                placeholder="12345"
              />
            </label>
            <label className="space-y-1">
              <span className="text-xs text-gray-400">Contracts</span>
              <input
                value={closeContracts}
                onChange={(e) => setCloseContracts(e.target.value)}
                inputMode="numeric"
                className="w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-white outline-none"
                placeholder="1"
              />
            </label>
          </div>

          <button
            type="button"
            disabled={!actionsEnabled || submitting !== null}
            onClick={() => void submitClose()}
            className="rounded-lg bg-emerald-700 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-emerald-600 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {submitting === "close" ? "Placing..." : "Place Close Test Order"}
          </button>
        </div>
      </div>

      {feedback && (
        <div className={`rounded-xl border px-4 py-3 text-sm ${
          feedback.tone === "success"
            ? "border-emerald-700 bg-emerald-900/20 text-emerald-200"
            : "border-red-700 bg-red-900/20 text-red-200"
        }`}>
          {feedback.text}
        </div>
      )}

      {lastResult && (
        <div className="rounded-xl border border-gray-800 bg-gray-950/70 p-4">
          <div className="text-sm font-semibold text-white mb-2">Last Test Response</div>
          <pre className="overflow-x-auto text-xs text-gray-300">{JSON.stringify(lastResult, null, 2)}</pre>
        </div>
      )}
    </div>
  );
}

type MainTab = "test" | "account" | "positions" | "orders" | "mirrored";

export default function DeltaLiveScalper({ actionsEnabled = true }: Props) {
  const [refreshKey, setRefreshKey] = useState(0);
  const { stats, trades, toggling, toggleEnabled, nextStatus } = useDeltaLive(refreshKey);
  const [tab, setTab] = useState<MainTab>("test");
  const [mirroredFilter, setMirroredFilter] = useState<"open" | "all">("open");

  const account = stats.account;
  const openTrades = trades.filter((t) => t.status === "OPEN");
  const displayTrades = mirroredFilter === "open" ? openTrades : trades;
  const failedTrades = trades.filter((t) => t.status === "FAILED");
  const refreshDeltaState = () => setRefreshKey((value) => value + 1);

  // Total unrealised PnL from Delta positions
  const totalUPnl = (account?.positions ?? []).reduce((s, p) => s + p.unrealisedPnl, 0);
  const winRate = stats.wins + stats.losses > 0 ? (stats.wins / (stats.wins + stats.losses)) * 100 : 0;

  return (
    <div className="p-4 space-y-4">

      {/* ── Header ──────────────────────────────────────────────────── */}
      <div className="flex items-start justify-between">
        <div>
          <h2 className="text-white font-bold text-lg flex items-center gap-2">
            <span className="text-red-400">🔴</span> Delta Exchange Live Trading
          </h2>
          <p className="text-gray-400 text-xs mt-0.5">
            When enabled, BTC Option Selling paper positions are mirrored to Delta Exchange.
          </p>
        </div>
        <div className="text-right">
          <div className="text-xs text-gray-500 bg-gray-800 rounded px-2 py-1">
            {stats.testnet ? "🧪 Testnet" : "🔴 Live"} · india.delta.exchange
          </div>
          {account?.fetchedAt && (
            <div className="text-xs text-gray-600 mt-1">Updated {fmtDate(account.fetchedAt)}</div>
          )}
        </div>
      </div>

      {/* ── Enable banner ───────────────────────────────────────────── */}
      <EnableBanner
        stats={stats}
        nextStatus={nextStatus}
        toggling={toggling}
        onToggle={actionsEnabled ? toggleEnabled : () => {}}
      />

      {/* ── Top KPI row ─────────────────────────────────────────────── */}
      {stats.configured && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <div className="bg-gray-800 rounded-xl p-3 border border-gray-700">
            <div className="text-gray-400 text-xs mb-1">USDT Available</div>
            <div className="text-xl font-bold text-white">${fmt(stats.walletUsdt)}</div>
          </div>
          <div className="bg-gray-800 rounded-xl p-3 border border-gray-700">
            <div className="text-gray-400 text-xs mb-1">Open Positions</div>
            <div className="text-xl font-bold text-blue-300">{account?.positions?.length ?? 0}</div>
          </div>
          <div className="bg-gray-800 rounded-xl p-3 border border-gray-700">
            <div className="text-gray-400 text-xs mb-1">Unrealised PnL</div>
            <div className={`text-xl font-bold ${pnlColor(totalUPnl)}`}>{pnlSign(totalUPnl)}${fmt(totalUPnl)}</div>
          </div>
          <div className="bg-gray-800 rounded-xl p-3 border border-gray-700">
            <div className="text-gray-400 text-xs mb-1">Mirror Win Rate</div>
            <div className={`text-xl font-bold ${winRate >= 50 ? "text-green-300" : "text-red-300"}`}>
              {fmt(winRate, 1)}%
            </div>
            <div className="text-xs text-gray-500">{stats.wins}W / {stats.losses}L</div>
          </div>
        </div>
      )}

      {/* ── Tabs ────────────────────────────────────────────────────── */}
      <div className="bg-gray-900 rounded-xl overflow-hidden border border-gray-800">
        <div className="flex border-b border-gray-700 overflow-x-auto">
          {([
            { key: "test",      label: `🧪 Test Orders` },
            { key: "account",   label: `💳 Wallet & Balances` },
            { key: "positions", label: `📊 Positions (${account?.positions?.length ?? 0})` },
            { key: "orders",    label: `📋 Open Orders (${account?.openOrders?.length ?? 0})` },
            { key: "mirrored",  label: `🔗 Mirrored Trades (${trades.length})` },
          ] as { key: MainTab; label: string }[]).map((t) => (
              <button
                type="button"
                key={t.key}
                onClick={() => setTab(t.key)}
                className={`px-4 py-3 text-sm font-medium whitespace-nowrap transition-colors ${
                  tab === t.key ? "text-white border-b-2 border-blue-500 bg-gray-800/30" : "text-gray-400 hover:text-gray-200"
                }`}
              >
                {t.label}
              </button>
          ))}
        </div>

        <div className="p-4">
          {tab === "account" && (
            <div className="space-y-4">
              <div className="text-gray-300 text-sm font-semibold">Account Balances</div>
              <WalletCards wallets={account?.wallets ?? []} />
              {account?.error && (
                <div className="text-red-400 text-xs bg-red-900/20 rounded p-2">{account.error}</div>
              )}
            </div>
          )}

          {tab === "positions" && (
            <div className="space-y-3">
              <div className="text-gray-300 text-sm font-semibold">Live Positions on Delta Exchange</div>
              <LivePositionsTable positions={account?.positions ?? []} />
            </div>
          )}

          {tab === "orders" && (
            <div className="space-y-3">
              <div className="text-gray-300 text-sm font-semibold">Open Orders on Delta Exchange</div>
              <OpenOrdersTable orders={account?.openOrders ?? []} />
            </div>
          )}

          {tab === "mirrored" && (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <div className="text-gray-300 text-sm font-semibold">Mirrored Orders</div>
                <div className="flex gap-1">
                  {(["open", "all"] as const).map((f) => (
                    <button
                      type="button"
                      key={f}
                      onClick={() => setMirroredFilter(f)}
                      className={`px-3 py-1 rounded text-xs font-medium ${
                        mirroredFilter === f ? "bg-blue-700 text-white" : "bg-gray-700 text-gray-300 hover:bg-gray-600"
                      }`}
                    >
                      {f === "open" ? `Open (${openTrades.length})` : `All (${trades.length})`}
                    </button>
                  ))}
                </div>
              </div>
              <MirroredTradesTable trades={displayTrades} />

              {failedTrades.length > 0 && (
                <div className="bg-red-900/20 border border-red-800 rounded-lg p-3 mt-2">
                  <div className="text-red-300 font-bold text-xs mb-2">⚠️ Failed Orders ({failedTrades.length})</div>
                  {failedTrades.slice(0, 5).map((t) => (
                    <div key={t.id} className="text-red-200/70 text-xs mb-1">
                      <span className="text-red-300 font-mono">{t.id}</span> — {t.failureReason}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {tab === "test" && (
            <TestOrderTab
              actionsEnabled={actionsEnabled}
              positions={account?.positions ?? []}
              onOrderPlaced={refreshDeltaState}
            />
          )}
        </div>
      </div>

      {/* Mirrored trade stats bar */}
      {stats.configured && trades.length > 0 && (
        <div className="grid grid-cols-3 gap-3">
          <div className="bg-gray-800 rounded-xl p-3 text-center border border-gray-700">
            <div className="text-2xl font-bold text-blue-300">{stats.openTrades}</div>
            <div className="text-gray-400 text-xs mt-1">Live Open</div>
          </div>
          <div className="bg-gray-800 rounded-xl p-3 text-center border border-gray-700">
            <div className={`text-2xl font-bold ${pnlColor(stats.totalPnl)}`}>
              {pnlSign(stats.totalPnl)}${fmt(stats.totalPnl)}
            </div>
            <div className="text-gray-400 text-xs mt-1">Realised PnL</div>
          </div>
          <div className="bg-gray-800 rounded-xl p-3 text-center border border-gray-700">
            <div className="text-2xl font-bold text-white">{stats.totalTrades}</div>
            <div className="text-gray-400 text-xs mt-1">Total Mirrored</div>
          </div>
        </div>
      )}
    </div>
  );
}
