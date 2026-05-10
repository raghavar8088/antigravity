"use client";

import Image from "next/image";
import { useState } from "react";
import { resolveEngineApiUrl } from "@/lib/engineApi";

type DashboardHeaderProps = {
  online: boolean;
  balance: number;
  dailyPnL?: number;
  openPositions: number;
  regime?: string;
  onResetSuccess?: () => void;
  onAdminEvent?: (message: string, tone: "admin" | "info") => void;
  combatMode?: boolean;
  onToggleCombat?: () => void;
  actionsEnabled?: boolean;
};


function RegimeBadge({ regime }: { regime?: string }) {
  if (!regime || regime === "NO_TRADE") return null;

  const map: Record<string, { color: string; bg: string; label: string }> = {
    TRENDING_BULL: { color: "var(--green)", bg: "var(--green-dim)", label: "Bull Trend" },
    TRENDING_BEAR: { color: "var(--red)", bg: "var(--red-dim)", label: "Bear Trend" },
    RANGE: { color: "var(--accent)", bg: "var(--accent-dim)", label: "Ranging" },
    HIGH_VOL: { color: "var(--amber)", bg: "var(--amber-dim)", label: "High Vol" },
  };

  const current = map[regime] || { color: "var(--text-secondary)", bg: "var(--surface-3)", label: regime };

  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        padding: "2px 8px",
        borderRadius: "var(--radius-chip)",
        background: current.bg,
        color: current.color,
        fontSize: 11,
        fontWeight: 500,
        fontFamily: "var(--font-display)",
      }}
    >
      {current.label}
    </span>
  );
}


export default function DashboardHeader({
  online,
  balance,
  openPositions,
  regime,
  onResetSuccess,
  onAdminEvent,
  combatMode = false,
  onToggleCombat,
  actionsEnabled = false,
}: DashboardHeaderProps) {
  const [activeAction, setActiveAction] = useState<string | null>(null);

  const postAdminAction = async (
    endpoint: string,
    confirmation: string,
    successMessage: string,
    resetAfter = false,
  ) => {
    if (!actionsEnabled) {
      return;
    }
    if (!confirm(confirmation)) return;
    setActiveAction(endpoint);
    try {
      const apiUrl = resolveEngineApiUrl();
      const response = await fetch(`${apiUrl}${endpoint}`, { method: "POST" });
      if (!response.ok) {
        throw new Error("action failed");
      }
      onAdminEvent?.(successMessage, "admin");
      if (resetAfter) {
        onResetSuccess?.();
      }
    } catch {
      onAdminEvent?.("Admin action failed. Check engine connectivity.", "admin");
    } finally {
      setActiveAction(null);
    }
  };

  const isBusy = activeAction !== null;
  const actionButtonTitle = actionsEnabled
    ? "Action buttons are enabled."
    : "Set Action to Yes to enable admin buttons.";

  return (
    <header className="cockpit-header">
      <div
        style={{
          maxWidth: 1680,
          margin: "0 auto",
          display: "flex",
          alignItems: "center",
          gap: 16,
          padding: "0 20px",
          height: "var(--header-height)",
        }}
      >
        {/* ── Left: Logo + Title ──────────────────────────────────── */}
        <div style={{ display: "flex", alignItems: "center", gap: 12, minWidth: 200, flexShrink: 0 }}>
          <div
            style={{
              width: 36,
              height: 36,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              borderRadius: "var(--radius-card)",
              background: "var(--accent-dim)",
            }}
          >
            <Image
              src="/branding/in-loop-logo.png"
              alt="in.loop.com"
              width={22}
              height={22}
              priority
              style={{ width: 22, height: 22, objectFit: "contain" }}
            />
          </div>
          <div>
            <div
              style={{
                fontFamily: "var(--font-display)",
                fontSize: 15,
                fontWeight: 600,
                color: "var(--text-primary)",
                lineHeight: 1.2,
              }}
            >
              in.loop.com Workspace
            </div>
            <div style={{ fontSize: 11, color: "var(--text-muted)" }}>
              Live BTC execution desk
            </div>
          </div>
        </div>

        {/* ── Center: Status Chip ─────────────────────────────────── */}
        <div
          style={{
            flex: 1,
            display: "flex",
            alignItems: "center",
            gap: 10,
            padding: "0 16px",
            minWidth: 0,
          }}
        >
          <div className={online ? "live-dot" : "live-dot-red"} />
          <div style={{ minWidth: 0 }}>
            <div
              style={{
                fontFamily: "var(--font-display)",
                fontSize: 13,
                fontWeight: 500,
                color: "var(--text-primary)",
                display: "flex",
                alignItems: "center",
                gap: 8,
              }}
            >
              {online ? "Engine live · BTC/USDT" : "Engine offline"}
              <RegimeBadge regime={regime} />
            </div>
            <div style={{ fontSize: 11, color: "var(--text-muted)" }}>
              {online ? `${openPositions} open positions` : "Waiting for heartbeat"}
            </div>
          </div>
        </div>

        {/* ── Right: Metrics + Actions ────────────────────────────── */}
        <div style={{ display: "flex", alignItems: "center", gap: 12, flexShrink: 0 }}>
          {/* Equity pill */}
          <div
            style={{
              padding: "6px 14px",
              borderRadius: "var(--radius-card)",
              border: "1px solid var(--border)",
              background: "var(--surface)",
            }}
          >
            <div style={{ fontSize: 10, fontWeight: 500, color: "var(--text-muted)", letterSpacing: "0.03em" }}>
              Futures Equity
            </div>
            <div
              style={{
                fontFamily: "var(--font-display)",
                fontSize: 16,
                fontWeight: 600,
                color: "var(--text-primary)",
                marginTop: 2,
              }}
            >
              ${balance.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
            </div>
          </div>

          {/* Open positions pill */}
          <div
            style={{
              padding: "6px 14px",
              borderRadius: "var(--radius-card)",
              border: "1px solid var(--border)",
              background: "var(--surface)",
              textAlign: "center",
            }}
          >
            <div style={{ fontSize: 10, fontWeight: 500, color: "var(--text-muted)", letterSpacing: "0.03em" }}>
              Open
            </div>
            <div
              style={{
                fontFamily: "var(--font-display)",
                fontSize: 16,
                fontWeight: 600,
                color: "var(--text-primary)",
                marginTop: 2,
              }}
            >
              {openPositions}
            </div>
          </div>

          {/* Theme toggle */}
          <button
            type="button"
            onClick={onToggleCombat}
            className={combatMode ? "combat-toggle-on" : "combat-toggle-off"}
            title="Toggle dark mode"
          >
            {combatMode ? "🌙 Dark" : "☀️ Light"}
          </button>

          {/* Action buttons */}
          <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
            <button
              type="button"
              onClick={() =>
                postAdminAction(
                  "/api/admin/close-all",
                  "Close all open positions at market price?",
                  "All positions closed.",
                  true,
                )
              }
              disabled={!actionsEnabled || isBusy || openPositions === 0}
              title={actionButtonTitle}
              className="btn-gold"
            >
              {activeAction === "/api/admin/close-all" ? "Closing…" : "Close All"}
            </button>
            <button
              type="button"
              onClick={() =>
                postAdminAction(
                  "/api/admin/kill",
                  "Trigger kill switch? Engine will halt.",
                  "Kill switch triggered.",
                )
              }
              disabled={!actionsEnabled || isBusy}
              title={actionButtonTitle}
              className="btn-danger"
            >
              {activeAction === "/api/admin/kill" ? "Stopping…" : "Kill"}
            </button>
            <button
              type="button"
              onClick={() =>
                postAdminAction(
                  "/api/admin/clear-history",
                  "Clear completed trade history and strategy stats? Open positions and balance will be kept.",
                  "Trade history cleared.",
                  true,
                )
              }
              disabled={!actionsEnabled || isBusy}
              title={actionButtonTitle}
              className="btn-primary"
            >
              {activeAction === "/api/admin/clear-history" ? "Clearing…" : "Clear History"}
            </button>
            <button
              type="button"
              onClick={() =>
                postAdminAction(
                  "/api/admin/reset",
                  "Reset paper account to $1,000,000?",
                  "Account reset to $1,000,000.",
                  true,
                )
              }
              disabled={!actionsEnabled || isBusy}
              title={actionButtonTitle}
              className="btn-gold"
            >
              {activeAction === "/api/admin/reset" ? "Resetting…" : "Reset"}
            </button>
            <button
              type="button"
              onClick={() =>
                postAdminAction(
                  "/api/backtest-demo",
                  "Start RAIG BTC Alpha Backtest in terminal?",
                  "Backtest triggered. Check execution logs.",
                )
              }
              disabled={!actionsEnabled || isBusy}
              title={actionButtonTitle}
              className="btn-gold"
            >
              {activeAction === "/api/backtest-demo" ? "Running…" : "Backtest"}
            </button>
          </div>
        </div>
      </div>
    </header>
  );
}
