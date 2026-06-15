"use client";

import { useState } from "react";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { StatusDot } from "@/components/ui/StatusDot";
import { useKillSwitch } from "@/hooks/useKillSwitch";
import { cn } from "@/components/ui/cn";

export function KillSwitchPanel() {
  const { status, loading, trigger, resume } = useKillSwitch();
  const [showHaltModal, setShowHaltModal] = useState(false);
  const [showResumeModal, setShowResumeModal] = useState(false);

  const isOffline = status.dataState === "engine_offline" || status.dataState === "error";

  async function onHaltConfirm() {
    await trigger("operator_manual_halt");
    setShowHaltModal(false);
  }

  async function onResumeConfirm() {
    await resume();
    setShowResumeModal(false);
  }

  return (
    <>
      {/* Emergency bar — full-width strip above header */}
      <div
        className={cn(
          "flex items-center justify-between px-4 h-9 text-[11px] font-medium transition-colors",
          status.active
            ? "bg-[rgba(239,68,68,0.15)] border-b border-[rgba(239,68,68,0.4)] animate-pulse"
            : isOffline
            ? "bg-[rgba(249,115,22,0.08)] border-b border-[rgba(249,115,22,0.2)]"
            : "bg-[var(--color-bg-overlay)] border-b border-[var(--color-border)]",
        )}
        role="status"
        aria-live="polite"
      >
        <span
          className={cn(
            "flex items-center gap-2",
            status.active ? "text-[var(--color-loss)]" : isOffline ? "text-[var(--color-warning)]" : "text-[var(--color-text-secondary)]",
          )}
        >
          {status.active ? (
            <>
              <StatusDot variant="error" size={6} />
              ⚡ KILL SWITCH ACTIVE — ALL STRATEGIES HALTED
              {status.reason ? (
                <span className="font-mono text-[var(--color-text-muted)] max-w-[420px] truncate" title={status.reason}>
                  · {status.reason}
                </span>
              ) : null}
              {status.triggeredAt ? (
                <span className="font-mono tnum text-[var(--color-text-muted)]">
                  · {new Date(status.triggeredAt).toLocaleTimeString("en-IN", { timeZone: "Asia/Kolkata", hour12: false })} IST
                </span>
              ) : null}
            </>
          ) : isOffline ? (
            <>
              <StatusDot variant="warning" size={6} />
              ENGINE OFFLINE — Kill switch status unknown
            </>
          ) : (
            <>
              <StatusDot variant="live" size={6} />
              ✓ SYSTEM ACTIVE
              {status.activeStrategies > 0 ? ` · ${status.activeStrategies} strategies running` : null}
              · Kill switch armed
            </>
          )}
        </span>

        <div className="flex items-center gap-2">
          {status.active ? (
            <button
              type="button"
              onClick={() => setShowResumeModal(true)}
              disabled={loading}
              className="px-3 py-1 rounded text-[11px] font-semibold bg-[var(--color-profit)] text-white hover:bg-[rgba(34,197,94,0.85)] disabled:opacity-50"
              aria-label="Resume trading — opens confirmation dialog"
            >
              ▶ RESUME TRADING
            </button>
          ) : (
            <button
              type="button"
              onClick={() => setShowHaltModal(true)}
              disabled={loading || isOffline}
              className={cn(
                "px-3 py-1 rounded text-[11px] font-semibold transition-colors",
                "bg-[var(--color-loss)] text-white",
                "hover:bg-[rgba(239,68,68,0.85)]",
                "disabled:opacity-40 disabled:cursor-not-allowed",
              )}
              aria-label="Halt all trading — opens confirmation dialog"
            >
              ⛔ HALT ALL
            </button>
          )}
        </div>
      </div>

      {/* Halt confirmation modal */}
      <ConfirmModal
        open={showHaltModal}
        onOpenChange={setShowHaltModal}
        title="⛔ Halt All Trading"
        description={`This will immediately halt all ${status.activeStrategies || "active"} strategies and cancel all open orders.`}
        consequence="This will immediately cancel all open orders, halt all active strategies, and prevent new signals. This action cannot be auto-reversed."
        confirmLabel="HALT ALL TRADING"
        cancelLabel="Cancel"
        destructive
        onConfirm={onHaltConfirm}
        loading={loading}
      />

      {/* Resume confirmation modal */}
      <ConfirmModal
        open={showResumeModal}
        onOpenChange={setShowResumeModal}
        title="▶ Resume Trading"
        description={
          status.openOrders > 0
            ? `There are ${status.openOrders} open orders in the system. Resuming will allow strategies to place new orders.`
            : "All strategies will be unblocked and allowed to place new orders."
        }
        consequence="Strategies will immediately resume scanning for entries and may place orders in the market."
        confirmLabel="Resume Trading"
        cancelLabel="Cancel"
        destructive={false}
        onConfirm={onResumeConfirm}
        loading={loading}
      />
    </>
  );
}
