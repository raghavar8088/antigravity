"use client";

import { useState } from "react";
import { useKillSwitch } from "@/hooks/useKillSwitch";
import { cn } from "@/components/ui/cn";
import { ConfirmModal } from "@/components/ui/ConfirmModal";

/** Compact header emergency control. Always visible in the top app bar. */
export function KillSwitchIndicator() {
  const { status, loading, trigger, resume } = useKillSwitch();
  const [showHaltModal, setShowHaltModal] = useState(false);
  const [showResumeModal, setShowResumeModal] = useState(false);

  const isActive = status.active;
  const isOffline = status.dataState !== "ok";
  const label = isActive ? "Triggered" : isOffline ? "Status updating" : "Halt all";

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
      <button
        type="button"
        className={cn(
          "m3-kill-switch-indicator",
          isActive && "m3-kill-switch-indicator--active",
          isOffline && !isActive && "m3-kill-switch-indicator--offline",
        )}
        onClick={() => (isActive ? setShowResumeModal(true) : setShowHaltModal(true))}
        disabled={loading || (!isActive && isOffline)}
        aria-label={isActive ? "Kill switch triggered. Open resume confirmation." : "Halt all trading. Open confirmation."}
      >
        {label}
      </button>

      <ConfirmModal
        open={showHaltModal}
        onOpenChange={setShowHaltModal}
        title="Halt All Trading"
        description={`This will immediately halt all ${status.activeStrategies || "active"} strategies and cancel all open orders.`}
        consequence="This will cancel open orders, halt active strategies, and prevent new signals. This action cannot be auto-reversed."
        confirmLabel="HALT ALL TRADING"
        cancelLabel="Cancel"
        destructive
        onConfirm={onHaltConfirm}
        loading={loading}
      />

      <ConfirmModal
        open={showResumeModal}
        onOpenChange={setShowResumeModal}
        title="Resume Trading"
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
