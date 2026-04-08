"use client";

import Image from "next/image";
import { useState } from "react";

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
    RANGE: { color: "var(--blue)", bg: "var(--blue-dim)", label: "Ranging" },
    HIGH_VOL: { color: "var(--fuchsia)", bg: "var(--fuchsia-dim)", label: "High Vol" },
  };

  const current = map[regime] || { color: "var(--text-secondary)", bg: "var(--surface-3)", label: regime };

  return (
    <span
      className="ml-2 rounded-full px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-[0.08em]"
      style={{ background: current.bg, color: current.color, border: `1px solid ${current.color}22` }}
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
      const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
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
      <div className="mx-auto flex max-w-[1680px] flex-wrap items-center gap-4 px-5 py-4">
        <div className="flex min-w-[220px] items-center gap-3">
          <div
            className="flex h-12 w-12 items-center justify-center rounded-2xl border"
            style={{
              borderColor: "rgba(26, 115, 232, 0.18)",
              background: "rgba(26, 115, 232, 0.06)",
            }}
          >
            <Image
              src="/raig-logo.png"
              alt="RAIG"
              width={28}
              height={28}
              priority
              style={{ width: 28, height: 28, objectFit: "contain" }}
            />
          </div>
          <div>
            <div className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>
              RAIG Workspace
            </div>
            <div className="text-xs" style={{ color: "var(--text-secondary)" }}>
              Live BTC execution desk
            </div>
          </div>
        </div>

        <div className="flex min-w-[240px] flex-1 items-center gap-3 rounded-full border px-4 py-3" style={{
          borderColor: "var(--border)",
          background: "var(--surface)",
        }}>
          <div className={online ? "live-dot" : "live-dot-red"} />
          <div className="flex-1">
            <div className="flex items-center gap-2 text-sm font-medium" style={{ color: "var(--text-primary)" }}>
              {online ? "Engine live and monitoring BTC/USDT" : "Engine offline"}
              <RegimeBadge regime={regime} />
            </div>
            <div className="text-xs" style={{ color: "var(--text-secondary)" }}>
              {online ? `${openPositions} open positions across the live book` : "Waiting for engine heartbeat"}
            </div>
          </div>
          <button
            type="button"
            onClick={onToggleCombat}
            className={combatMode ? "combat-toggle-on" : "combat-toggle-off"}
            title="Toggle combat mode"
          >
            {combatMode ? "Combat" : "Normal"}
          </button>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <div className="summary-card min-w-[170px]">
            <div className="summary-label">Futures Equity</div>
            <div className="summary-value">${balance.toLocaleString(undefined, {
              minimumFractionDigits: 2,
              maximumFractionDigits: 2,
            })}</div>
          </div>

<div className="metric-card min-w-[120px]">
            <div className="metric-label">Open</div>
            <div className="metric-value">{openPositions}</div>
          </div>
        </div>

        <div className="ml-auto flex flex-wrap items-center gap-2">
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
            {activeAction === "/api/admin/close-all" ? "Closing" : "Close All"}
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
            {activeAction === "/api/admin/kill" ? "Stopping" : "Kill"}
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
            {activeAction === "/api/admin/clear-history" ? "Clearing" : "Clear Trade History"}
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
            className="btn-primary"
          >
            {activeAction === "/api/admin/reset" ? "Resetting" : "Reset"}
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
            {activeAction === "/api/backtest-demo" ? "Processing" : "RAIG Backtest"}
          </button>
        </div>
      </div>
    </header>
  );
}
