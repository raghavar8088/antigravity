"use client";

import { useEffect, useState } from "react";
import { BTCFutureTradingScalper } from "@/components/BTCFutureTradingScalper";
import ReplayBacktestPanel from "@/components/ReplayBacktestPanel";
import WorkspaceSettingsCard from "@/components/desk/WorkspaceSettingsCard";
import { WorkspaceNavPanel } from "@/components/desk/WorkspaceNavPanel";
import { workspaceModuleDescription } from "@/lib/workspaceModuleDescription";

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
      if (e.key === " ") { e.preventDefault(); setCombatMode((p) => !p); }
      if (e.key === "m" || e.key === "M") setIsSoundOn((p) => !p);
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  const actionToggleTitle = actionsEnabled
    ? "Dangerous controls (reset, clear, kill, close-all) are enabled."
    : "Locked: reset/clear/kill/close-all hidden. Server-side paper engines still run.";

  return (
    <main className="gmail-shell">
      <div className="workspace-shell workspace-shell--no-sidebar">
        <div className="workspace-main" style={{ display: "flex", flexDirection: "column", gap: 20 }}>
          <WorkspaceSettingsCard
            workspaceKey="btcFutureTrading"
            workspaceLabel="BTC Future Trading"
            workspaceDescription="Dedicated BTC futures module with 20 globally popular strategy archetypes (trend, breakout, order-flow, smart-money, MTF)."
            currencyCode="USD"
            defaultMode="paper"
            defaultDataSource="binance"
            strategyItems={[]}
          />

          <WorkspaceNavPanel
            activeModule="btcFutureTrading"
            onModuleChange={() => {}}
            actionsEnabled={actionsEnabled}
            onActionsEnabledChange={setActionsEnabled}
            actionToggleTitle={actionToggleTitle}
            moduleDescription={workspaceModuleDescription("btcFutureTrading")}
          />

          <BTCFutureTradingScalper />

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
            summary="Run offline desk replay (dev API) on fixture klines — does not change live paper wallet or open positions."
          />
        </div>
      </div>
    </main>
  );
}
