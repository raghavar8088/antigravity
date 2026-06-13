"use client";

import { useEffect, useState } from "react";
import {
  DeskButton,
  DeskCard,
  DeskChip,
  DeskMetricTile,
  DeskSectionHeader,
} from "@/components/desk/ui";
import type { StrategyToggleItem } from "@/components/WorkspaceSettingsPanel";
import { formatDeskInr, formatDeskUsd } from "@/lib/trading/deskFormat";
import { useDeskMounted } from "@/hooks/useDeskMounted";

type WorkspaceMode = "paper" | "live" | "analysis";
type DataSource = "binance" | "bybit" | "nse" | "angel" | "yahoo";

type StoredSettings = {
  capital: string;
  perTradeRisk: number;
  cooldownMinutes: number;
  mode: WorkspaceMode;
  dataSource: DataSource;
  autoExecute: boolean;
  notifications: boolean;
  strategyStates: Record<string, boolean>;
};

type Props = {
  workspaceKey: string;
  workspaceLabel: string;
  workspaceDescription: string;
  currencyCode: "USD" | "INR";
  defaultMode: WorkspaceMode;
  defaultDataSource: DataSource;
  strategyItems: StrategyToggleItem[];
};

const WORKSPACE_CAPITAL_TOOLTIP =
  "Display-only sizing for workspace settings. Does not change the module paper wallet (see Paper desk balance in the app bar).";

function buildDefaultState(
  currencyCode: "USD" | "INR",
  defaultMode: WorkspaceMode,
  defaultDataSource: DataSource,
  strategyItems: StrategyToggleItem[],
): StoredSettings {
  return {
    capital: "1000000",
    perTradeRisk: 1,
    cooldownMinutes: 15,
    mode: defaultMode,
    dataSource: defaultDataSource,
    autoExecute: false,
    notifications: true,
    strategyStates: Object.fromEntries(strategyItems.map((item) => [item.id, item.enabled])),
  };
}

export default function WorkspaceSettingsCard({
  workspaceKey,
  workspaceLabel,
  workspaceDescription,
  currencyCode,
  defaultMode,
  defaultDataSource,
  strategyItems,
}: Props) {
  const storageKey = `raig.workspace.settings.${workspaceKey}`;
  const [isExpanded, setIsExpanded] = useState(false);
  const [settings, setSettings] = useState<StoredSettings>(() => {
    const fallback = buildDefaultState(currencyCode, defaultMode, defaultDataSource, strategyItems);
    if (typeof window === "undefined") return fallback;
    try {
      const raw = window.localStorage.getItem(storageKey);
      if (!raw) return fallback;
      const parsed = JSON.parse(raw) as Partial<StoredSettings>;
      return {
        ...fallback,
        ...parsed,
        strategyStates: { ...fallback.strategyStates, ...(parsed.strategyStates ?? {}) },
      };
    } catch {
      return fallback;
    }
  });

  useEffect(() => {
    try {
      window.localStorage.setItem(storageKey, JSON.stringify(settings));
    } catch {
      // ignore
    }
  }, [settings, storageKey]);

  const mounted = useDeskMounted();
  const enabledCount = strategyItems.filter((item) => settings.strategyStates[item.id] ?? item.enabled).length;
  const capitalNum = Number(settings.capital || 0);
  const capitalDisplay = !mounted
    ? "—"
    : currencyCode === "INR"
      ? formatDeskInr(capitalNum, { decimals: 0 })
      : formatDeskUsd(capitalNum, { decimals: 0 });

  return (
    <DeskCard>
      <DeskSectionHeader
        title="Workspace settings"
        subtitle={workspaceLabel}
        actions={
          <DeskButton variant="outlined" onClick={() => setIsExpanded((v) => !v)}>
            {isExpanded ? "Hide settings" : "Open settings"}
          </DeskButton>
        }
      />
      <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", marginBottom: 16, maxWidth: 720 }}>
        {workspaceDescription}
      </p>
      <div className="desk-metrics-row" style={{ marginBottom: 12 }}>
        <DeskMetricTile
          label="Workspace display"
          value={capitalDisplay}
          detail="Not paper wallet"
          compact
          title={WORKSPACE_CAPITAL_TOOLTIP}
        />
        <DeskMetricTile label="Per-trade risk" value={`${settings.perTradeRisk.toFixed(2)}%`} compact />
        <DeskMetricTile label="Cooldown" value={`${settings.cooldownMinutes} min`} compact />
        <DeskMetricTile label="Execution" value={settings.autoExecute ? "Armed" : "Manual"} compact />
        <DeskMetricTile label="Default desk" value={settings.dataSource} detail={`${settings.mode} mode`} compact />
      </div>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
        <DeskChip title={WORKSPACE_CAPITAL_TOOLTIP}>Workspace</DeskChip>
        <DeskChip tone="primary">{settings.mode}</DeskChip>
        <DeskChip>{settings.dataSource}</DeskChip>
        <DeskChip>
          {enabledCount}/{strategyItems.length} strategies
        </DeskChip>
      </div>

      {isExpanded ? (
        <div
          style={{
            marginTop: 24,
            display: "grid",
            gap: 20,
            gridTemplateColumns: "repeat(auto-fit, minmax(280px, 1fr))",
          }}
        >
          <DeskCard variant="outlined" padding="md" elevation={0}>
            <p className="desk-title-md" style={{ marginBottom: 12 }}>
              Execution rules
            </p>
            <div className="settings-field-grid" style={{ display: "grid", gap: 12 }}>
              <label className="settings-field" title={WORKSPACE_CAPITAL_TOOLTIP}>
                <span className="desk-label-md">Workspace display capital (not paper wallet)</span>
                <input
                  value={settings.capital}
                  onChange={(e) =>
                    setSettings((c) => ({ ...c, capital: e.target.value.replace(/[^\d]/g, "") }))
                  }
                  className="settings-input"
                  inputMode="numeric"
                />
              </label>
              <label className="settings-field">
                <span className="desk-label-md">Per-trade risk % ({settings.perTradeRisk.toFixed(2)}%)</span>
                <input
                  type="range"
                  min="0.25"
                  max="5"
                  step="0.25"
                  value={settings.perTradeRisk}
                  onChange={(e) => setSettings((c) => ({ ...c, perTradeRisk: Number(e.target.value) }))}
                />
              </label>
              <label className="settings-field">
                <span className="desk-label-md">Cooldown (minutes)</span>
                <input
                  type="number"
                  min={0}
                  step={5}
                  value={settings.cooldownMinutes}
                  onChange={(e) => setSettings((c) => ({ ...c, cooldownMinutes: Number(e.target.value) }))}
                  className="settings-input"
                />
              </label>
              <label className="settings-field">
                <span className="desk-label-md">Data source</span>
                <select
                  value={settings.dataSource}
                  onChange={(e) => setSettings((c) => ({ ...c, dataSource: e.target.value as DataSource }))}
                  className="settings-input"
                >
                  <option value="binance">Binance</option>
                  <option value="bybit">Bybit</option>
                  <option value="nse">NSE mirror</option>
                  <option value="angel">Angel One</option>
                  <option value="yahoo">Yahoo Finance</option>
                </select>
              </label>
              <label className="settings-field">
                <span className="desk-label-md">Workspace mode</span>
                <select
                  value={settings.mode}
                  onChange={(e) => setSettings((c) => ({ ...c, mode: e.target.value as WorkspaceMode }))}
                  className="settings-input"
                >
                  <option value="paper">Paper</option>
                  <option value="live">Live</option>
                  <option value="analysis">Analysis</option>
                </select>
              </label>
            </div>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 12 }}>
              <button
                type="button"
                className={`settings-chip${settings.autoExecute ? " active" : ""}`}
                onClick={() => setSettings((c) => ({ ...c, autoExecute: !c.autoExecute }))}
              >
                Auto execute {settings.autoExecute ? "on" : "off"}
              </button>
              <button
                type="button"
                className={`settings-chip${settings.notifications ? " active" : ""}`}
                onClick={() => setSettings((c) => ({ ...c, notifications: !c.notifications }))}
              >
                Notifications {settings.notifications ? "on" : "off"}
              </button>
            </div>
          </DeskCard>

          <DeskCard variant="outlined" padding="md" elevation={0}>
            <p className="desk-title-md" style={{ marginBottom: 12 }}>
              Strategy controls
            </p>
            {strategyItems.length === 0 ? (
              <DeskEmptyStateInline text="Strategy controls appear when this workspace exposes a managed roster." />
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                {strategyItems.map((item) => {
                  const enabled = settings.strategyStates[item.id] ?? item.enabled;
                  return (
                    <button
                      key={item.id}
                      type="button"
                      className={`strategy-toggle-row${enabled ? " active" : ""}`}
                      onClick={() =>
                        setSettings((c) => ({
                          ...c,
                          strategyStates: { ...c.strategyStates, [item.id]: !enabled },
                        }))
                      }
                    >
                      <div>
                        <p className="desk-body-md" style={{ fontWeight: 500 }}>
                          {item.label}
                        </p>
                        <p className="desk-label-md" style={{ marginTop: 4 }}>
                          {item.description}
                        </p>
                      </div>
                      <DeskChip tone={enabled ? "success" : "default"}>{enabled ? "Enabled" : "Paused"}</DeskChip>
                    </button>
                  );
                })}
              </div>
            )}
          </DeskCard>
        </div>
      ) : null}
    </DeskCard>
  );
}

function DeskEmptyStateInline({ text }: { text: string }) {
  return (
    <p className="desk-label-md" style={{ padding: "16px 0", textAlign: "center" }}>
      {text}
    </p>
  );
}
