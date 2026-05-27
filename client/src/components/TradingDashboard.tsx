"use client";

import { useState } from "react";
import { BTCFutureTradingScalper } from "@/components/BTCFutureTradingScalper";
import ReplayBacktestPanel from "@/components/ReplayBacktestPanel";
import WorkspaceSettingsCard from "@/components/desk/WorkspaceSettingsCard";
import { WorkspaceNavPanel } from "@/components/desk/WorkspaceNavPanel";
import { SignalTracePanel } from "@/components/SignalTracePanel";
import { DeskCard, DeskChip } from "@/components/desk/ui";
import { workspaceModuleDescription } from "@/lib/workspaceModuleDescription";
import { BTC_FUTURE_TRADING_STRATEGY_IDS } from "@/lib/btcFutureTradingRoster";

export default function TradingDashboard() {
  const [actionsEnabled, setActionsEnabled] = useState(false);

  const actionToggleTitle = actionsEnabled
    ? "Dangerous controls (reset, clear, kill, close-all) are enabled."
    : "Locked: reset/clear/kill/close-all hidden. Server-side paper engines still run.";

  return (
    <main className="trading-shell">
      <div className="trading-shell__inner">
        <header className="trading-landing-header">
          <div>
            <h1 className="trading-landing-header__title">BTC Future Trading</h1>
            <p className="trading-landing-header__desc">
              Paper perpetual futures desk — curated strategy basket, live BTC marks, and Mongo-backed trade history.
            </p>
          </div>
          <div className="trading-landing-header__chips">
            <DeskChip tone="primary">Paper</DeskChip>
            <DeskChip tone="default">25× leverage</DeskChip>
            <DeskChip tone="default">{BTC_FUTURE_TRADING_STRATEGY_IDS.length} strategies</DeskChip>
            <DeskChip tone="default">BTCUSD</DeskChip>
          </div>
        </header>

        <div className="workspace-nav--slim">
          <WorkspaceNavPanel
            activeModule="btcFutureTrading"
            onModuleChange={() => {}}
            actionsEnabled={actionsEnabled}
            onActionsEnabledChange={setActionsEnabled}
            actionToggleTitle={actionToggleTitle}
            moduleDescription={workspaceModuleDescription("btcFutureTrading")}
          />
        </div>

        <DeskCard padding="md">
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 8 }}>
            <span style={{ fontSize: 13, fontWeight: 700, color: "#58a6ff" }}>Signal Trace</span>
            <span style={{ fontSize: 10, color: "#8b949e" }}>
              Why didn&apos;t this signal become a trade? — per-strategy gate breakdown
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
    </main>
  );
}
