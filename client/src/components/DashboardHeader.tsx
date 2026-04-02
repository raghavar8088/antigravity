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

  const handleReset    = () => postAdminAction("/api/admin/reset",     "Reset paper account to $100,000?",       "Account reset to $100,000.", true);
  const handleKillSwitch = () => postAdminAction("/api/admin/kill",    "Trigger kill switch? Engine will halt.", "Kill switch triggered.");
  const handleCloseAll = () => postAdminAction("/api/admin/close-all", "Close all open positions at market price?", "All positions closed.", true);

  const isBusy = activeAction !== null;
  const isPositive = dailyPnL >= 0;
  const invested = balance - dailyPnL;
  const pnlPct = invested > 0 ? (dailyPnL / invested) * 100 : 0;

  return (
    <header className="cockpit-header px-6 py-3">
      <div className="max-w-[1700px] mx-auto flex items-center justify-between gap-6">

        {/* ── RAIG Logo + Identity ── */}
        <div className="flex items-center gap-4 shrink-0">
          <div style={{ position: "relative" }}>
            <img
              src="/raig-logo.png"
              alt="RAIG 888"
              style={{
                width: 48,
                height: 48,
                objectFit: "contain",
                filter: "drop-shadow(0 0 12px rgba(212,175,55,0.6))",
              }}
            />
          </div>
          <div>
            <div
              style={{ fontFamily: "var(--font-display, 'Orbitron', sans-serif)", fontSize: 18, fontWeight: 900, letterSpacing: "0.08em", lineHeight: 1 }}
              className="text-gold"
            >
              RAIG <span style={{ color: "#333", fontSize: 10 }}>888</span>
            </div>
            <div style={{ fontSize: 8, fontWeight: 700, letterSpacing: "0.25em", color: "var(--text-muted)", marginTop: 4, fontFamily: "var(--font-display, 'Orbitron', sans-serif)" }}>
              AUTONOMOUS · AI · SCALPING
            </div>
          </div>

          {/* Vertical divider */}
          <div style={{ width: 1, height: 36, background: "var(--border-gold)", marginLeft: 8 }} />

          {/* Status */}
          <div className="flex items-center gap-2">
            <div className={online ? "live-dot" : "live-dot-red"} />
            <div>
              <div style={{ fontSize: 9, fontWeight: 700, color: online ? "var(--green)" : "var(--red)", letterSpacing: "0.15em", fontFamily: "var(--font-display)" }}>
                {online ? "ENGINE LIVE" : "OFFLINE"}
              </div>
              <div style={{ fontSize: 8, color: "var(--text-muted)", letterSpacing: "0.1em" }}>BTC / USDT</div>
            </div>
          </div>
        </div>

        {/* ── Portfolio Value (center) ── */}
        <div style={{ flex: 1, display: "flex", flexDirection: "column", alignItems: "center" }}>
          <div style={{ fontSize: 8, fontWeight: 700, letterSpacing: "0.22em", color: "var(--text-muted)", marginBottom: 4, fontFamily: "var(--font-display)" }}>
            PAPER PORTFOLIO VALUE
          </div>
          <div
            className="mono"
            style={{
              fontSize: 28,
              fontWeight: 800,
              letterSpacing: "-0.02em",
              lineHeight: 1,
              color: "var(--gold-bright)",
              textShadow: "0 0 30px rgba(212,175,55,0.35)",
              fontFamily: "var(--font-display)",
            }}
          >
            ${balance.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
          </div>
          <div className="flex items-center gap-2 mt-2">
            <span className={isPositive ? "pill-green" : "pill-red"}>
              <span>{isPositive ? "▲" : "▼"}</span>
              <span>{isPositive ? "+" : ""}{dailyPnL.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
              <span style={{ opacity: 0.7 }}>({isPositive ? "+" : ""}{pnlPct.toFixed(2)}%)</span>
            </span>
            <span style={{ fontSize: 8, color: "var(--text-muted)", letterSpacing: "0.12em", fontFamily: "var(--font-display)" }}>TODAY</span>
          </div>
        </div>

        {/* ── System Stats ── */}
        <div className="flex items-center gap-5 shrink-0">
          <div style={{ textAlign: "center" }}>
            <div style={{ fontSize: 8, fontWeight: 700, letterSpacing: "0.18em", color: "var(--text-muted)", fontFamily: "var(--font-display)", marginBottom: 3 }}>OPEN</div>
            <div style={{ fontSize: 20, fontWeight: 800, color: openPositions > 0 ? "var(--gold)" : "var(--text-muted)", fontFamily: "var(--font-display)", lineHeight: 1 }}>
              {openPositions}
            </div>
            <div style={{ fontSize: 7, color: "var(--text-muted)", letterSpacing: "0.1em" }}>POSITIONS</div>
          </div>

          <div style={{ width: 1, height: 32, background: "var(--border)" }} />

          {/* ── Admin Controls ── */}
          <div className="flex items-center gap-2">
            <button
              onClick={handleCloseAll}
              disabled={isBusy || openPositions === 0}
              className="btn-gold"
              style={{
                opacity: openPositions === 0 ? 0.3 : isBusy ? 0.5 : 1,
                pointerEvents: openPositions === 0 ? "none" : "auto",
                fontSize: 9,
                padding: "6px 12px",
              }}
            >
              {activeAction === "/api/admin/close-all" ? "CLOSING…" : `CLOSE ALL (${openPositions})`}
            </button>
            <button
              onClick={handleKillSwitch}
              disabled={isBusy}
              className="btn-danger"
              style={{ fontSize: 9, padding: "6px 12px" }}
            >
              {activeAction === "/api/admin/kill" ? "KILLING…" : "KILL SWITCH"}
            </button>
            <button
              onClick={handleReset}
              disabled={isBusy}
              className="btn-primary"
              style={{ fontSize: 9, padding: "6px 12px" }}
            >
              {activeAction === "/api/admin/reset" ? "RESETTING…" : "RESET"}
            </button>
          </div>

          <div style={{ width: 1, height: 32, background: "var(--border)" }} />

          {/* Security badge */}
          <div style={{ textAlign: "right" }}>
            <div style={{ fontSize: 7, fontWeight: 800, color: "var(--gold)", letterSpacing: "0.18em", fontFamily: "var(--font-display)" }}>SECURITY</div>
            <div style={{ fontSize: 8, fontWeight: 700, color: "var(--text-secondary)", letterSpacing: "0.1em" }}>ENCRYPTED</div>
            <div style={{ fontSize: 7, color: "var(--green)", letterSpacing: "0.1em" }}>● ACTIVE</div>
          </div>
        </div>

      </div>

      {/* Bottom gold line */}
      <div style={{ height: 1, background: "linear-gradient(90deg, transparent, var(--gold-glow), rgba(0,255,136,0.2), var(--gold-glow), transparent)", marginTop: 8 }} />
    </header>
  );
}
