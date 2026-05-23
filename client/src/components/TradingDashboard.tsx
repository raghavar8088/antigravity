"use client";

import { useEffect, useState } from "react";
import { BTCFutureTradingScalper } from "@/components/BTCFutureTradingScalper";
import ReplayBacktestPanel from "@/components/ReplayBacktestPanel";
import WorkspaceSettingsCard from "@/components/desk/WorkspaceSettingsCard";
import { WorkspaceNavPanel } from "@/components/desk/WorkspaceNavPanel";
import { DeskChip } from "@/components/desk/ui";
import { workspaceModuleDescription } from "@/lib/workspaceModuleDescription";
import { BTC_FUTURE_TRADING_STRATEGY_IDS } from "@/lib/btcFutureTradingRoster";

const SOUND_STORAGE_KEY = "raig.sound.enabled";

function readStoredSound(): boolean {
  if (typeof window === "undefined") return true;
  const stored = window.localStorage.getItem(SOUND_STORAGE_KEY);
  return stored === null ? true : stored === "true";
}

export default function TradingDashboard() {
  const [actionsEnabled, setActionsEnabled] = useState(false);
  const [isSoundOn, setIsSoundOn] = useState(() => readStoredSound());
  const [combatMode, setCombatMode] = useState(false);

  useEffect(() => {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(SOUND_STORAGE_KEY, String(isSoundOn));
    }
  }, [isSoundOn]);

  useEffect(() => {
    const root = document.documentElement;
    if (combatMode) {
      document.body.classList.add("combat-mode");
      root.setAttribute("data-theme", "dark");
    } else {
      document.body.classList.remove("combat-mode");
      root.setAttribute("data-theme", "light");
    }
    return () => {
      document.body.classList.remove("combat-mode");
      root.setAttribute("data-theme", "light");
    };
  }, [combatMode]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
      if (e.key === " ") {
        e.preventDefault();
        setCombatMode((p) => !p);
      }
      if (e.key === "m" || e.key === "M") setIsSoundOn((p) => !p);
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

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
