/**
 * Mobile Emergency View — /mobile
 *
 * Designed for one use case: paged at 2am, something is wrong,
 * need to assess and halt from a phone.
 *
 * Optimized for 375px width. Large touch targets. No complex state.
 */

"use client";

import { useEffect, useState } from "react";
import { useKillSwitch } from "@/hooks/useKillSwitch";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { PnlValue } from "@/components/ui/PnlValue";
import { GaugeBar } from "@/components/ui/GaugeBar";
import { cn } from "@/components/ui/cn";
import type { RiskSummary } from "@/types/trading";
import { fmtISTClock } from "@/lib/istTime";

export default function MobileEmergencyPage() {
  const { status: ks, trigger, resume, loading: ksLoading } = useKillSwitch();
  const [risk, setRisk] = useState<RiskSummary | null>(null);
  const [showHalt, setShowHalt] = useState(false);
  const [showResume, setShowResume] = useState(false);
  const [showPauseCrypto, setShowPauseCrypto] = useState(false);

  useEffect(() => {
    const load = async () => {
      try {
        const res = await fetch("/api/risk/summary", { cache: "no-store" });
        const d = await res.json();
        if (!d.error) setRisk(d);
      } catch { /* ignore */ }
    };
    load();
    const id = setInterval(load, 10_000);
    return () => clearInterval(id);
  }, []);

  const isOffline = ks.dataState !== "ok";
  const hasAlert = ks.active || isOffline;

  return (
    <div className="min-h-screen bg-[var(--color-bg-base)] text-[var(--color-text-primary)] flex flex-col max-w-[480px] mx-auto px-4 py-4 gap-4">

      {/* System status banner */}
      <div
        className={cn(
          "rounded-xl p-5 text-center",
          ks.active
            ? "bg-[rgba(239,68,68,0.15)] border border-[rgba(239,68,68,0.4)]"
            : isOffline
            ? "bg-[rgba(249,115,22,0.12)] border border-[rgba(249,115,22,0.3)]"
            : "bg-[rgba(34,197,94,0.08)] border border-[rgba(34,197,94,0.2)]",
        )}
        role="status"
        aria-live="polite"
      >
        <div className="text-[28px] mb-1">
          {ks.active ? "🚨" : isOffline ? "⚠️" : "✅"}
        </div>
        <div className={cn(
          "text-[18px] font-bold",
          ks.active ? "text-[var(--color-loss)]" : isOffline ? "text-[var(--color-warning)]" : "text-[var(--color-profit)]",
        )}>
          {ks.active
            ? "KILL SWITCH TRIGGERED"
            : isOffline
            ? "ENGINE OFFLINE"
            : "All Systems Normal"}
        </div>
        <div className="text-[13px] text-[var(--color-text-secondary)] mt-1">
          {ks.active
            ? `All strategies halted · ${ks.triggeredAt ? fmtISTClock(ks.triggeredAt) + " IST" : ""}`
            : ks.activeStrategies > 0
            ? `${ks.activeStrategies} strategies active`
            : "Waiting for engine data…"}
        </div>
      </div>

      {/* HALT ALL or RESUME button */}
      {ks.active ? (
        <button
          type="button"
          onClick={() => setShowResume(true)}
          disabled={ksLoading}
          className="w-full h-16 rounded-xl bg-[var(--color-profit)] text-white text-[18px] font-bold disabled:opacity-50"
          aria-label="Resume all trading"
        >
          ▶ RESUME TRADING
        </button>
      ) : (
        <button
          type="button"
          onClick={() => setShowHalt(true)}
          disabled={ksLoading || isOffline}
          className="w-full h-16 rounded-xl bg-[var(--color-loss)] text-white text-[18px] font-bold disabled:opacity-40"
          aria-label="Halt all trading"
        >
          ⛔ HALT ALL TRADING
        </button>
      )}

      {/* Key metrics 2×2 grid */}
      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-4 text-center">
          <div className="text-[11px] text-[var(--color-text-muted)] mb-1">Session P&L</div>
          {risk ? (
            <PnlValue value={risk.dailyPnl} showSign size="xl" className="block" />
          ) : (
            <div className="skeleton h-8 rounded" />
          )}
        </div>

        <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-4 text-center">
          <div className="text-[11px] text-[var(--color-text-muted)] mb-1">Active Strategies</div>
          <div className="text-[28px] font-mono tnum font-bold text-[var(--color-text-primary)]">
            {ks.activeStrategies || "—"}
          </div>
        </div>

        <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-4 text-center">
          <div className="text-[11px] text-[var(--color-text-muted)] mb-1">Open Orders</div>
          <div className={cn(
            "text-[28px] font-mono tnum font-bold",
            ks.openOrders > 0 ? "text-[var(--color-caution)]" : "text-[var(--color-text-primary)]",
          )}>
            {ks.openOrders}
          </div>
        </div>

        <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-4 text-center">
          <div className="text-[11px] text-[var(--color-text-muted)] mb-1">Drawdown Used</div>
          {risk ? (
            <div className="flex flex-col items-center gap-2">
              <div className={cn(
                "text-[24px] font-mono tnum font-bold",
                risk.drawdownUsedPct > 80 ? "text-[var(--color-loss)]" : risk.drawdownUsedPct > 60 ? "text-[var(--color-caution)]" : "text-[var(--color-profit)]",
              )}>
                {risk.drawdownUsedPct.toFixed(0)}%
              </div>
              <GaugeBar value={risk.drawdownUsedPct} height={5} className="w-full" />
            </div>
          ) : (
            <div className="skeleton h-8 rounded" />
          )}
        </div>
      </div>

      {/* Quick actions */}
      <div className="flex flex-col gap-2">
        <div className="text-[12px] font-medium text-[var(--color-text-muted)] uppercase tracking-wide">Quick Actions</div>
        <button
          type="button"
          onClick={() => setShowPauseCrypto(true)}
          className="w-full h-12 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-surface)] text-[14px] font-medium text-[var(--color-text-primary)] hover:bg-[var(--color-bg-elevated)]"
        >
          Pause All Crypto Strategies
        </button>
        <a
          href="/terminal/trading"
          className="w-full h-12 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-surface)] text-[14px] font-medium text-[var(--color-info)] hover:bg-[var(--color-bg-elevated)] flex items-center justify-center"
        >
          View Full Dashboard →
        </a>
      </div>

      {/* Modals */}
      <ConfirmModal
        open={showHalt}
        onOpenChange={setShowHalt}
        title="⛔ Halt All Trading"
        consequence={`Cancels all ${ks.openOrders} open orders and halts all ${ks.activeStrategies} active strategies immediately.`}
        confirmLabel="HALT ALL"
        destructive
        onConfirm={async () => { await trigger("mobile_emergency_halt"); setShowHalt(false); }}
        loading={ksLoading}
      />
      <ConfirmModal
        open={showResume}
        onOpenChange={setShowResume}
        title="▶ Resume Trading"
        consequence="All strategies will be unblocked and may immediately place new orders."
        confirmLabel="Resume"
        destructive={false}
        onConfirm={async () => { await resume(); setShowResume(false); }}
        loading={ksLoading}
      />
      <ConfirmModal
        open={showPauseCrypto}
        onOpenChange={setShowPauseCrypto}
        title="Pause Crypto Strategies"
        description="Pauses all BINANCE, DELTA, and COINBASE strategies."
        consequence="Open crypto positions are NOT auto-closed."
        confirmLabel="Pause Crypto"
        destructive
        onConfirm={async () => {
          await fetch("/api/strategies", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ action: "pause_by_exchange", exchange: ["BINANCE","DELTA","COINBASE"] }) });
          setShowPauseCrypto(false);
        }}
      />
    </div>
  );
}
