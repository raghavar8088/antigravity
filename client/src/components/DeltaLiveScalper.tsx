"use client";
import { useState } from "react";
import useDeltaLive, { type DeltaLiveTrade, type DeltaLiveStats } from "@/hooks/useDeltaLive";

type Props = { actionsEnabled?: boolean };

function fmt(n: number, dp = 2) {
  return n.toLocaleString("en-US", { minimumFractionDigits: dp, maximumFractionDigits: dp });
}

function fmtTime(iso: string) {
  try { return new Date(iso).toLocaleTimeString(); } catch { return iso; }
}

function StatusBadge({ status }: { status: DeltaLiveTrade["status"] }) {
  const styles: Record<string, string> = {
    OPEN:      "bg-blue-900 text-blue-200",
    CLOSED:    "bg-green-900 text-green-200",
    FAILED:    "bg-red-900 text-red-200",
    CANCELLED: "bg-gray-700 text-gray-300",
  };
  return (
    <span className={`px-2 py-0.5 rounded text-xs font-bold ${styles[status] ?? "bg-gray-700 text-gray-300"}`}>
      {status}
    </span>
  );
}

function ConfigBanner({ stats, toggling, onToggle }: { stats: DeltaLiveStats; toggling: boolean; onToggle: (v: boolean) => void }) {
  if (!stats.configured) {
    return (
      <div className="bg-yellow-900/40 border border-yellow-600 rounded-lg p-4 mb-4">
        <div className="flex items-start gap-3">
          <span className="text-yellow-400 text-xl">⚠️</span>
          <div>
            <div className="text-yellow-300 font-bold text-sm mb-1">Delta Exchange Not Configured</div>
            <div className="text-yellow-200/80 text-xs leading-relaxed">
              Set the following environment variables on your Go engine server to enable live trading:
            </div>
            <div className="mt-2 bg-black/40 rounded p-2 font-mono text-xs text-green-300 space-y-1">
              <div>DELTA_API_KEY=your_api_key</div>
              <div>DELTA_API_SECRET=your_api_secret</div>
              <div className="text-gray-400"># Optional: use testnet</div>
              <div className="text-gray-400">DELTA_TESTNET=true</div>
            </div>
            <div className="text-yellow-200/60 text-xs mt-2">
              Get API keys from <span className="text-yellow-300">india.delta.exchange → API Keys</span>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className={`border rounded-lg p-4 mb-4 flex items-center justify-between ${
      stats.enabled ? "bg-green-900/30 border-green-600" : "bg-gray-800 border-gray-600"
    }`}>
      <div className="flex items-center gap-3">
        <div className={`w-3 h-3 rounded-full ${stats.enabled ? "bg-green-400 animate-pulse" : "bg-gray-500"}`} />
        <div>
          <div className="text-sm font-bold text-white">
            Live Order Mirroring {stats.enabled ? "ACTIVE" : "PAUSED"}
          </div>
          <div className="text-xs text-gray-400">
            {stats.testnet ? "🧪 Testnet mode" : "🔴 Production — real money"} &nbsp;|&nbsp;
            Wallet: ${fmt(stats.walletUsdt)} USDT
          </div>
        </div>
      </div>
      <button
        disabled={toggling}
        onClick={() => onToggle(!stats.enabled)}
        className={`px-4 py-2 rounded text-sm font-bold transition-colors ${
          stats.enabled
            ? "bg-red-700 hover:bg-red-600 text-white"
            : "bg-green-700 hover:bg-green-600 text-white"
        } disabled:opacity-50`}
      >
        {toggling ? "..." : stats.enabled ? "⏸ Disable" : "▶ Enable"}
      </button>
    </div>
  );
}

function StatsRow({ stats }: { stats: DeltaLiveStats }) {
  const winRate = stats.wins + stats.losses > 0
    ? (stats.wins / (stats.wins + stats.losses)) * 100 : 0;

  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-4">
      {[
        { label: "Open Trades", value: stats.openTrades.toString(), color: "text-blue-300" },
        { label: "Total Trades", value: stats.totalTrades.toString(), color: "text-white" },
        { label: "Win Rate", value: `${fmt(winRate, 1)}%`, color: winRate >= 50 ? "text-green-300" : "text-red-300" },
        { label: "Realized PnL", value: `$${fmt(stats.totalPnl)}`, color: stats.totalPnl >= 0 ? "text-green-300" : "text-red-300" },
      ].map((s) => (
        <div key={s.label} className="bg-gray-800 rounded-lg p-3">
          <div className="text-gray-400 text-xs mb-1">{s.label}</div>
          <div className={`text-lg font-bold ${s.color}`}>{s.value}</div>
        </div>
      ))}
    </div>
  );
}

type Tab = "open" | "all";

export default function DeltaLiveScalper({ actionsEnabled = true }: Props) {
  const { stats, trades, toggling, toggleEnabled } = useDeltaLive();
  const [tab, setTab] = useState<Tab>("open");

  const openTrades = trades.filter((t) => t.status === "OPEN");
  const displayTrades = tab === "open" ? openTrades : trades;

  return (
    <div className="p-4 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-white font-bold text-lg">Delta Exchange Live Trading</h2>
          <p className="text-gray-400 text-xs mt-0.5">
            Mirrors BTC Option Selling paper signals to real Delta Exchange orders
          </p>
        </div>
        <div className="text-xs text-gray-500 bg-gray-800 rounded px-2 py-1">
          india.delta.exchange
        </div>
      </div>

      {/* Config / enable banner */}
      <ConfigBanner stats={stats} toggling={toggling} onToggle={actionsEnabled ? toggleEnabled : () => {}} />

      {/* Stats */}
      <StatsRow stats={stats} />

      {/* Trades table */}
      <div className="bg-gray-900 rounded-lg overflow-hidden">
        <div className="flex border-b border-gray-700">
          {(["open", "all"] as Tab[]).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`px-4 py-2 text-sm font-medium transition-colors ${
                tab === t ? "text-white border-b-2 border-blue-500" : "text-gray-400 hover:text-gray-200"
              }`}
            >
              {t === "open" ? `Open (${openTrades.length})` : `All Trades (${trades.length})`}
            </button>
          ))}
        </div>

        {displayTrades.length === 0 ? (
          <div className="text-center py-12 text-gray-500 text-sm">
            {stats.configured
              ? stats.enabled
                ? "No trades yet — waiting for BTC Option Selling signals..."
                : "Enable live mirroring above to start trading"
              : "Configure Delta API keys to start live trading"}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-gray-400 border-b border-gray-700">
                  <th className="px-3 py-2 text-left">ID</th>
                  <th className="px-3 py-2 text-left">Strategy</th>
                  <th className="px-3 py-2 text-left">Type</th>
                  <th className="px-3 py-2 text-right">Strike</th>
                  <th className="px-3 py-2 text-left">Symbol</th>
                  <th className="px-3 py-2 text-right">Contracts</th>
                  <th className="px-3 py-2 text-right">Fill $</th>
                  <th className="px-3 py-2 text-right">PnL</th>
                  <th className="px-3 py-2 text-left">Status</th>
                  <th className="px-3 py-2 text-left">Opened</th>
                </tr>
              </thead>
              <tbody>
                {displayTrades.map((t) => (
                  <tr key={t.id} className="border-b border-gray-800 hover:bg-gray-800/50">
                    <td className="px-3 py-2 font-mono text-gray-300">{t.id}</td>
                    <td className="px-3 py-2 text-gray-200 max-w-[120px] truncate">{t.strategyName}</td>
                    <td className="px-3 py-2">
                      <span className={`font-bold ${t.optionType === "CALL" ? "text-green-400" : "text-red-400"}`}>
                        {t.optionType}
                      </span>
                    </td>
                    <td className="px-3 py-2 text-right text-white">${fmt(t.strike, 0)}</td>
                    <td className="px-3 py-2 font-mono text-blue-300 text-xs">{t.deltaSymbol || "—"}</td>
                    <td className="px-3 py-2 text-right text-white">{t.contracts}</td>
                    <td className="px-3 py-2 text-right text-white">${fmt(t.fillPrice, 4)}</td>
                    <td className="px-3 py-2 text-right">
                      {t.status === "CLOSED" && t.realizedPnl !== undefined ? (
                        <span className={t.realizedPnl >= 0 ? "text-green-400" : "text-red-400"}>
                          {t.realizedPnl >= 0 ? "+" : ""}${fmt(t.realizedPnl)}
                        </span>
                      ) : (
                        <span className="text-gray-500">—</span>
                      )}
                    </td>
                    <td className="px-3 py-2"><StatusBadge status={t.status} /></td>
                    <td className="px-3 py-2 text-gray-400">{fmtTime(t.openedAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Failed trades warning */}
      {trades.some((t) => t.status === "FAILED") && (
        <div className="bg-red-900/30 border border-red-700 rounded-lg p-3">
          <div className="text-red-300 font-bold text-xs mb-1">⚠️ Failed Orders</div>
          {trades.filter((t) => t.status === "FAILED").slice(0, 3).map((t) => (
            <div key={t.id} className="text-red-200/70 text-xs">
              {t.id}: {t.failureReason}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
