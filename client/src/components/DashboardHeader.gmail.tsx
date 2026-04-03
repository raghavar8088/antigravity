"use client";

import Image from "next/image";
import { useState } from "react";

type DashboardHeaderProps = {
  online: boolean;
  balance: number;
  dailyPnL: number;
  openPositions: number;
  onResetSuccess?: () => void;
  onAdminEvent?: (message: string, tone: "admin" | "info") => void;
  combatMode?: boolean;
  onToggleCombat?: () => void;
};

function formatSignedCurrency(value: number) {
  return `${value >= 0 ? "+" : "-"}$${Math.abs(value).toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

export default function DashboardHeader({
  online,
  balance,
  dailyPnL,
  openPositions,
  onResetSuccess,
  onAdminEvent,
  combatMode = false,
  onToggleCombat,
}: DashboardHeaderProps) {
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
  const invested = balance - dailyPnL;
  const pnlPct = invested > 0 ? (dailyPnL / invested) * 100 : 0;
  const positive = dailyPnL >= 0;

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
            <div className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>
              {online ? "Engine live and monitoring BTC/USDT" : "Engine offline"}
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
            <div className="summary-label">Equity</div>
            <div className="summary-value">${balance.toLocaleString(undefined, {
              minimumFractionDigits: 2,
              maximumFractionDigits: 2,
            })}</div>
          </div>

          <div className="summary-card min-w-[170px]">
            <div className="summary-label">PnL Today</div>
            <div className={`summary-value ${positive ? "profit-positive" : "profit-negative"}`}>
              {formatSignedCurrency(dailyPnL)}
            </div>
            <div className="mt-2 text-xs" style={{ color: "var(--text-secondary)" }}>
              {positive ? "+" : ""}
              {pnlPct.toFixed(2)}%
            </div>
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
            disabled={isBusy || openPositions === 0}
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
            disabled={isBusy}
            className="btn-danger"
          >
            {activeAction === "/api/admin/kill" ? "Stopping" : "Kill"}
          </button>
          <button
            type="button"
            onClick={() =>
              postAdminAction(
                "/api/admin/reset",
                "Reset paper account to $100,000?",
                "Account reset to $100,000.",
                true,
              )
            }
            disabled={isBusy}
            className="btn-primary"
          >
            {activeAction === "/api/admin/reset" ? "Resetting" : "Reset"}
          </button>
        </div>
      </div>
    </header>
  );
}
