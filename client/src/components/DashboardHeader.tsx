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
    if (!confirm(confirmation)) return;
    setActiveAction(endpoint);
    try {
      const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const res = await fetch(`${API_URL}${endpoint}`, { method: "POST" });
      if (!res.ok) throw new Error("Action failed");
      onAdminEvent?.(successMessage, "admin");
      if (resetAfter) onResetSuccess?.();
    } catch {
      onAdminEvent?.("Admin action failed. Check engine connectivity.", "admin");
    } finally {
      setActiveAction(null);
    }
  };

  const handleReset = () =>
    postAdminAction("/api/admin/reset", "Reset paper account to $100,000?", "Account reset to $100,000.", true);
  const handleKillSwitch = () =>
    postAdminAction("/api/admin/kill", "Trigger kill switch? Engine will halt.", "Kill switch triggered.");
  const handleCloseAll = () =>
    postAdminAction("/api/admin/close-all", "Close all open positions at market price?", "All positions closed.", true);

  const isBusy = activeAction !== null;
  const isPositive = dailyPnL >= 0;
  const invested = balance - dailyPnL;
  const pnlPct = invested > 0 ? (dailyPnL / invested) * 100 : 0;

  return (
    <header
      style={{
        background: "var(--surface)",
        borderBottom: "1px solid var(--border)",
      }}
      className="px-6 py-4"
    >
      <div className="max-w-[1600px] mx-auto flex flex-col md:flex-row items-start md:items-center justify-between gap-4">

        {/* ── Logo + status ── */}
        <div className="flex items-center gap-3 shrink-0">
          <img
            src="/raig-logo.png"
            alt="RAIG 888"
            style={{ width: 52, height: 52, objectFit: "contain", filter: "drop-shadow(0 0 8px rgba(250,188,44,0.5))" }}
          />
          <div>
            <div className="font-black text-white text-lg tracking-tighter leading-none flex items-center gap-1">
              RAIG <span style={{ color: "var(--green)", fontSize: 10, fontWeight: 700, letterSpacing: "0.1em" }}>AUTONOMOUS</span>
            </div>
            <div className="flex items-center gap-1.5 mt-1">
              <span
                className={`w-1.5 h-1.5 rounded-full ${online ? "animate-pulse" : ""}`}
                style={{ background: online ? "var(--green)" : "var(--red)" }}
              />
              <span style={{ color: "var(--text-secondary)" }} className="text-[10px] uppercase tracking-widest font-bold">
                {online ? "Engine Live" : "Offline"} · BTC/USDT
              </span>
            </div>
          </div>
        </div>

        {/* ── Groww-style portfolio value ── */}
        <div className="flex flex-col items-center text-center">
          <div style={{ color: "var(--text-muted)" }} className="text-[10px] uppercase tracking-widest mb-1 font-semibold">
            Paper Portfolio
          </div>
          <div className="text-3xl font-extrabold text-white tabular-nums leading-none">
            ${balance.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
          </div>
          <div className="mt-2 flex items-center gap-2">
            <span className={isPositive ? "pill-green" : "pill-red"}>
              <span>{isPositive ? "▲" : "▼"}</span>
              <span>
                {isPositive ? "+" : ""}
                {dailyPnL.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
              </span>
              <span className="opacity-60">
                ({isPositive ? "+" : ""}{pnlPct.toFixed(2)}%)
              </span>
            </span>
            <span style={{ color: "var(--text-muted)", fontSize: 10 }} className="uppercase tracking-widest">today</span>
          </div>
        </div>

        {/* ── Admin actions ── */}
        <div className="flex flex-wrap items-center gap-2 shrink-0">
          {/* Always in DOM to keep header height stable; hidden when no positions */}
          <button
            onClick={handleCloseAll}
            disabled={isBusy || openPositions === 0}
            style={{
              border: "1px solid rgba(245,158,11,0.35)",
              color: "#F59E0B",
              borderRadius: 10,
              padding: "7px 14px",
              fontSize: 11,
              fontWeight: 700,
              letterSpacing: "0.06em",
              background: "rgba(245,158,11,0.07)",
              cursor: (isBusy || openPositions === 0) ? "not-allowed" : "pointer",
              opacity: openPositions === 0 ? 0 : isBusy ? 0.6 : 1,
              pointerEvents: openPositions === 0 ? "none" : "auto",
              transition: "opacity 0.2s",
            }}
          >
            {activeAction === "/api/admin/close-all" ? "Closing…" : `Close All (${openPositions})`}
          </button>
          <button
            onClick={handleKillSwitch}
            disabled={isBusy}
            style={{
              border: "1px solid rgba(244,67,54,0.35)",
              color: "var(--red)",
              borderRadius: 10,
              padding: "7px 14px",
              fontSize: 11,
              fontWeight: 700,
              letterSpacing: "0.06em",
              background: "var(--red-dim)",
              cursor: isBusy ? "not-allowed" : "pointer",
              opacity: isBusy ? 0.6 : 1,
              transition: "background 0.15s",
            }}
          >
            {activeAction === "/api/admin/kill" ? "Killing…" : "Kill Switch"}
          </button>
          <button
            onClick={handleReset}
            disabled={isBusy}
            style={{
              border: "1px solid rgba(0,208,156,0.3)",
              color: "var(--green)",
              borderRadius: 10,
              padding: "7px 14px",
              fontSize: 11,
              fontWeight: 700,
              letterSpacing: "0.06em",
              background: "var(--green-dim)",
              cursor: isBusy ? "not-allowed" : "pointer",
              opacity: isBusy ? 0.6 : 1,
              transition: "background 0.15s",
            }}
          >
            {activeAction === "/api/admin/reset" ? "Resetting…" : "Reset Account"}
          </button>
        </div>
<div style={{ marginLeft: "auto", borderLeft: "1px solid var(--border)", paddingLeft: 16, display: "flex", flexDirection: "column", alignItems: "flex-end" }}>
          <div style={{ fontSize: 9, fontWeight: 800, color: "var(--gold)", letterSpacing: "0.1em" }}>SECURITY PROTOCOL</div>
          <div style={{ fontSize: 10, fontWeight: 700, color: "white" }}>ACTIVE ENCRYPTION</div>
        </div>

      </div>
    </header>
  );
}
