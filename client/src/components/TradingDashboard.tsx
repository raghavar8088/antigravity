"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { BTCFutureTradingScalper } from "@/components/BTCFutureTradingScalper";
import ReplayBacktestPanel from "@/components/ReplayBacktestPanel";
import WorkspaceSettingsCard from "@/components/desk/WorkspaceSettingsCard";
import { SignalTracePanel } from "@/components/SignalTracePanel";
import { DeskCard } from "@/components/desk/ui";
import { BTC_FUTURE_TRADING_STRATEGY_IDS } from "@/lib/btcFutureTradingRoster";
import { AppShell } from "@/components/terminal";

export default function TradingDashboard() {
  const [actionsEnabled, setActionsEnabled] = useState(false);
  const router = useRouter();

  return (
    <AppShell pageTitle="BTC Futures">
      <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>

        {/* Page header */}
        <div style={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          gap: 16,
          padding: "14px 18px",
          background: "var(--surface)",
          border: "1px solid var(--border)",
          borderRadius: "var(--radius-card)",
        }}>
          <div>
            <h1 style={{
              fontFamily: "var(--font-display)",
              fontSize: 15,
              fontWeight: 700,
              color: "var(--text-primary)",
              margin: "0 0 4px",
              letterSpacing: "-0.2px",
            }}>
              BTC Future Trading
            </h1>
            <p style={{ fontSize: 12, color: "var(--text-secondary)", margin: 0, lineHeight: 1.5 }}>
              Paper perpetual futures desk — {BTC_FUTURE_TRADING_STRATEGY_IDS.length} strategies, live BTC marks, Mongo-backed trade history.
            </p>
          </div>
          <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
            <span className="pill-blue">Paper</span>
            <span className="pill-blue">25× leverage</span>
            <span className="pill-blue">{BTC_FUTURE_TRADING_STRATEGY_IDS.length} strategies</span>

            {/* Actions lock toggle */}
            <button
              type="button"
              onClick={() => setActionsEnabled((v) => !v)}
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 5,
                padding: "4px 10px",
                borderRadius: 4,
                border: `1px solid ${actionsEnabled ? "rgba(239,83,80,0.3)" : "var(--border)"}`,
                background: actionsEnabled ? "var(--red-dim)" : "var(--surface-2)",
                color: actionsEnabled ? "var(--red)" : "var(--text-secondary)",
                fontSize: 11,
                fontWeight: 700,
                cursor: "pointer",
                letterSpacing: "0.03em",
              }}
              title={actionsEnabled ? "Controls active — reset/kill/close-all enabled" : "Controls locked"}
            >
              {actionsEnabled ? "⚠ ACTIONS ENABLED" : "🔒 LOCKED"}
            </button>
          </div>
        </div>

        {/* Signal Trace */}
        <DeskCard padding="md">
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 10 }}>
            <span style={{
              fontSize: 12,
              fontWeight: 700,
              color: "var(--accent-strong)",
              letterSpacing: "-0.1px",
              fontFamily: "var(--font-display)",
            }}>
              Signal Trace
            </span>
            <span style={{ fontSize: 10, color: "var(--text-muted)" }}>
              Per-strategy gate breakdown — why a signal did or did not become a trade
            </span>
          </div>
          <SignalTracePanel accountKey={null} />
        </DeskCard>

        <BTCFutureTradingScalper />

        <details className="trading-replay-fold">
          <summary>Developer tools — replay backtest (offline)</summary>
          <div className="trading-replay-fold__body">
            <ReplayBacktestPanel
              workspaceLabel="BTC Future Trading"
              priceSeries={[]}
              events={[]}
              deskReplay={{
                symbol: "BTCUSD",
                bars: 500,
                fixture: "live",
                accountKey: "btc_future_trading_20",
              }}
              summary="Run offline desk replay on fixture klines — does not change live paper wallet or open positions."
            />
          </div>
        </details>

        <WorkspaceSettingsCard
          workspaceKey="btcFutureTrading"
          workspaceLabel="BTC Future Trading"
          workspaceDescription="Display-only workspace preferences. Paper wallet balance is controlled by the desk engine above."
          currencyCode="USD"
          defaultMode="paper"
          defaultDataSource="binance"
          strategyItems={[]}
        />
      </div>
    </AppShell>
  );
}
