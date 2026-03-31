"use client";

import { useState } from "react";

export default function DashboardHeader({
  online,
  balance,
  dailyPnL,
  openPositions,
  onResetSuccess,
  onAdminEvent,
}: {
  online: boolean;
  balance: number;
  dailyPnL: number;
  openPositions: number;
  onResetSuccess?: () => void;
  onAdminEvent?: (message: string, tone: "admin" | "info") => void;
}) {
  const [activeAction, setActiveAction] = useState<string | null>(null);

  const postAdminAction = async (
    endpoint: string,
    confirmation: string,
    successMessage: string,
    resetAfter = false,
  ) => {
    if (!confirm(confirmation)) {
      return;
    }

    setActiveAction(endpoint);
    try {
      const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const res = await fetch(`${API_URL}${endpoint}`, {
        method: "POST",
      });
      if (!res.ok) throw new Error("Reset failed");
      onAdminEvent?.(successMessage, "admin");
      if (resetAfter) {
        onResetSuccess?.();
      }
    } catch (error) {
      console.error(error);
      onAdminEvent?.("Admin action failed. Check engine connectivity.", "admin");
    } finally {
      setActiveAction(null);
    }
  };

  const handleReset = async () => {
    await postAdminAction(
      "/api/admin/reset",
      "Reset the paper trading account to its initial state? This clears positions and history.",
      "Paper account reset to a clean $100,000 state.",
      true,
    );
  };

  const handleKillSwitch = async () => {
    await postAdminAction(
      "/api/admin/kill",
      "Trigger the kill switch? This halts the engine and attempts to flatten exposure.",
      "Kill switch triggered. Engine halt requested.",
    );
  };

  const handleCloseAll = async () => {
    await postAdminAction(
      "/api/admin/close-all",
      "Close all open paper positions at the current market price?",
      "All open paper positions were closed.",
      true,
    );
  };

  const isBusy = activeAction !== null;

  return (
    <header className="glass-panel p-6 flex flex-col xl:flex-row justify-between items-start xl:items-center gap-5">
      <div>
        <h1 className="text-3xl font-extrabold tracking-tight">
          ANTI<span className="text-gradient">GRAVITY</span>
        </h1>
        <p className="text-sm text-gray-400 mt-1 flex items-center gap-2">
          {online ? (
            <>
              <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse shadow-[0_0_8px_#10b981]"></span>
              ENGINE ONLINE
            </>
          ) : (
            <>
              <span className="w-2 h-2 bg-red-500 rounded-full shadow-[0_0_8px_#ef4444]"></span>
              ENGINE OFFLINE
            </>
          )}
          <span className="px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20 text-xs font-mono ml-2">BTC/USDT ONLY</span>
        </p>
      </div>

      <div className="flex flex-col md:flex-row gap-4 md:gap-8 md:items-center w-full xl:w-auto">
        <div className="text-right">
          <p className="text-sm text-gray-400 font-semibold uppercase tracking-wider">Account Balance</p>
          <p className="text-2xl font-bold font-mono text-white">${balance.toLocaleString(undefined, { minimumFractionDigits: 2 })}</p>
        </div>

        <div className="text-right">
          <p className="text-sm text-gray-400 font-semibold uppercase tracking-wider">Daily PnL</p>
          <p className={`text-2xl font-bold font-mono ${dailyPnL >= 0 ? "text-green-400" : "text-red-400"}`}>
            {dailyPnL >= 0 ? "+" : ""}${dailyPnL.toLocaleString(undefined, { minimumFractionDigits: 2 })}
          </p>
        </div>

        <div className="flex flex-wrap gap-3 xl:ml-3">
          {openPositions > 0 && (
            <button
              onClick={handleCloseAll}
              disabled={isBusy}
              className="px-5 py-3 rounded-xl bg-amber-500/10 text-amber-300 border border-amber-500/30 hover:bg-amber-500 hover:text-zinc-950 transition-all font-bold uppercase tracking-widest text-sm outline-none focus:ring-4 focus:ring-amber-500/30 disabled:opacity-60"
            >
              {activeAction === "/api/admin/close-all" ? "CLOSING..." : `CLOSE ALL (${openPositions})`}
            </button>
          )}
          <button
            onClick={handleKillSwitch}
            disabled={isBusy}
            className="px-5 py-3 rounded-xl bg-red-500/10 text-red-400 border border-red-500/30 hover:bg-red-500 hover:text-white transition-all font-bold uppercase tracking-widest text-sm outline-none focus:ring-4 focus:ring-red-500/40 disabled:opacity-60 shadow-[0_0_15px_rgba(239,68,68,0.18)]"
          >
            {activeAction === "/api/admin/kill" ? "KILLING..." : "KILL SWITCH"}
          </button>
          <button
            onClick={handleReset}
            disabled={isBusy}
            className="px-5 py-3 rounded-xl bg-blue-500/10 text-blue-400 border border-blue-500/30 hover:bg-blue-500 hover:text-white transition-all font-bold uppercase tracking-widest text-sm outline-none focus:ring-4 focus:ring-blue-500/40 disabled:opacity-60 shadow-[0_0_15px_rgba(59,130,246,0.18)]"
          >
            {activeAction === "/api/admin/reset" ? "RESETTING..." : "RESET ACCOUNT"}
          </button>
        </div>
      </div>
    </header>
  );
}

