"use client";

import { useEffect, useState } from "react";

export type StrategyToggleItem = {
  id: string;
  label: string;
  description: string;
  enabled: boolean;
};

type WorkspaceMode = "paper" | "live" | "analysis";
type DataSource = "binance" | "bybit" | "nse" | "angel";

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

function formatCurrencyLabel(currencyCode: "USD" | "INR") {
  return currencyCode === "INR" ? "Rs" : "$";
}

function buildDefaultState(
  currencyCode: "USD" | "INR",
  defaultMode: WorkspaceMode,
  defaultDataSource: DataSource,
  strategyItems: StrategyToggleItem[],
): StoredSettings {
  return {
    capital: currencyCode === "INR" ? "1000000" : "1000000",
    perTradeRisk: 1,
    cooldownMinutes: 15,
    mode: defaultMode,
    dataSource: defaultDataSource,
    autoExecute: false,
    notifications: true,
    strategyStates: Object.fromEntries(strategyItems.map((item) => [item.id, item.enabled])),
  };
}

export default function WorkspaceSettingsPanel({
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
    if (typeof window === "undefined") {
      return fallback;
    }
    try {
      const raw = window.localStorage.getItem(storageKey);
      if (!raw) {
        return fallback;
      }
      const parsed = JSON.parse(raw) as Partial<StoredSettings>;
      return {
        ...fallback,
        ...parsed,
        strategyStates: {
          ...fallback.strategyStates,
          ...(parsed.strategyStates ?? {}),
        },
      };
    } catch {
      return fallback;
    }
  });

  useEffect(() => {
    try {
      window.localStorage.setItem(storageKey, JSON.stringify(settings));
    } catch {
      // Ignore local persistence failures.
    }
  }, [settings, storageKey]);

  const enabledCount = strategyItems.filter((item) => settings.strategyStates[item.id] ?? item.enabled).length;

  return (
    <section className="glass-panel px-5 py-5 md:px-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">Workspace Settings</div>
          <div className="text-xl font-semibold text-zinc-900">{workspaceLabel}</div>
          <div className="max-w-[780px] text-sm leading-6" style={{ color: "var(--text-secondary)" }}>
            {workspaceDescription}
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <span className="rounded-full border px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em]" style={{ borderColor: "var(--border)", background: "var(--surface-2)", color: "var(--text-secondary)" }}>
            {settings.mode} mode
          </span>
          <span className="rounded-full border px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em]" style={{ borderColor: "var(--border)", background: "var(--surface-2)", color: "var(--text-secondary)" }}>
            {settings.dataSource}
          </span>
          <span className="rounded-full border px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em]" style={{ borderColor: "var(--border)", background: "var(--surface-2)", color: "var(--text-secondary)" }}>
            {enabledCount}/{strategyItems.length} enabled
          </span>
          <button type="button" className="btn-gold text-sm" onClick={() => setIsExpanded((value) => !value)}>
            {isExpanded ? "Hide Settings" : "Open Settings"}
          </button>
        </div>
      </div>

      <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <div className="metric-card min-h-[104px]">
          <div className="metric-label">Capital</div>
          <div className="metric-value text-zinc-900">{formatCurrencyLabel(currencyCode)} {Number(settings.capital || 0).toLocaleString(currencyCode === "INR" ? "en-IN" : "en-US")}</div>
          <div className="mt-2 text-xs" style={{ color: "var(--text-secondary)" }}>Paper capital or operator reference balance.</div>
        </div>
        <div className="metric-card min-h-[104px]">
          <div className="metric-label">Per-Trade Risk</div>
          <div className="metric-value text-zinc-900">{settings.perTradeRisk.toFixed(2)}%</div>
          <div className="mt-2 text-xs" style={{ color: "var(--text-secondary)" }}>Applied as the intended risk budget for each entry.</div>
        </div>
        <div className="metric-card min-h-[104px]">
          <div className="metric-label">Cooldown</div>
          <div className="metric-value text-zinc-900">{settings.cooldownMinutes} min</div>
          <div className="mt-2 text-xs" style={{ color: "var(--text-secondary)" }}>Post-exit lockout window before the same strategy can re-enter.</div>
        </div>
        <div className="metric-card min-h-[104px]">
          <div className="metric-label">Execution</div>
          <div className="metric-value text-zinc-900">{settings.autoExecute ? "Armed" : "Manual"}</div>
          <div className="mt-2 text-xs" style={{ color: "var(--text-secondary)" }}>Separate UI state for reset/live/admin operator actions.</div>
        </div>
      </div>

      {isExpanded && (
        <div className="mt-5 grid gap-5 xl:grid-cols-[minmax(0,1.1fr)_minmax(320px,0.9fr)]">
          <div className="glass-panel px-5 py-5">
            <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">Execution Rules</div>
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              <label className="settings-field">
                <span>Capital</span>
                <input
                  value={settings.capital}
                  onChange={(event) => setSettings((current) => ({ ...current, capital: event.target.value.replace(/[^\d]/g, "") }))}
                  className="settings-input"
                  inputMode="numeric"
                />
              </label>
              <label className="settings-field">
                <span>Per-trade risk %</span>
                <input
                  type="range"
                  min="0.25"
                  max="5"
                  step="0.25"
                  value={settings.perTradeRisk}
                  onChange={(event) => setSettings((current) => ({ ...current, perTradeRisk: Number(event.target.value) }))}
                />
              </label>
              <label className="settings-field">
                <span>Cooldown minutes</span>
                <input
                  type="number"
                  min="0"
                  step="5"
                  value={settings.cooldownMinutes}
                  onChange={(event) => setSettings((current) => ({ ...current, cooldownMinutes: Number(event.target.value) }))}
                  className="settings-input"
                />
              </label>
              <label className="settings-field">
                <span>Data source</span>
                <select
                  value={settings.dataSource}
                  onChange={(event) => setSettings((current) => ({ ...current, dataSource: event.target.value as DataSource }))}
                  className="settings-input"
                >
                  <option value="binance">Binance</option>
                  <option value="bybit">Bybit</option>
                  <option value="nse">NSE mirror</option>
                  <option value="angel">Angel One</option>
                </select>
              </label>
              <label className="settings-field">
                <span>Workspace mode</span>
                <select
                  value={settings.mode}
                  onChange={(event) => setSettings((current) => ({ ...current, mode: event.target.value as WorkspaceMode }))}
                  className="settings-input"
                >
                  <option value="paper">Paper</option>
                  <option value="live">Live</option>
                  <option value="analysis">Analysis</option>
                </select>
              </label>
            </div>

            <div className="mt-4 flex flex-wrap gap-3">
              <button
                type="button"
                className={`settings-chip${settings.autoExecute ? " active" : ""}`}
                onClick={() => setSettings((current) => ({ ...current, autoExecute: !current.autoExecute }))}
              >
                Auto Execute {settings.autoExecute ? "On" : "Off"}
              </button>
              <button
                type="button"
                className={`settings-chip${settings.notifications ? " active" : ""}`}
                onClick={() => setSettings((current) => ({ ...current, notifications: !current.notifications }))}
              >
                Notifications {settings.notifications ? "On" : "Off"}
              </button>
            </div>
          </div>

          <div className="glass-panel px-5 py-5">
            <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">Strategy Controls</div>
            <div className="mt-4 space-y-3">
              {strategyItems.length === 0 ? (
                <div className="rounded-[20px] border border-dashed px-4 py-6 text-sm text-center" style={{ borderColor: "var(--border)", color: "var(--text-secondary)" }}>
                  Strategy-level controls will appear here when this workspace exposes a managed strategy roster.
                </div>
              ) : (
                strategyItems.map((item) => {
                  const enabled = settings.strategyStates[item.id] ?? item.enabled;
                  return (
                    <button
                      key={item.id}
                      type="button"
                      className={`strategy-toggle-row${enabled ? " active" : ""}`}
                      onClick={() => setSettings((current) => ({
                        ...current,
                        strategyStates: {
                          ...current.strategyStates,
                          [item.id]: !enabled,
                        },
                      }))}
                    >
                      <div>
                        <div className="text-sm font-medium text-zinc-900">{item.label}</div>
                        <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>{item.description}</div>
                      </div>
                      <span className="rounded-full border px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em]" style={{ borderColor: enabled ? "rgba(24,128,56,0.18)" : "var(--border)", background: enabled ? "rgba(24,128,56,0.08)" : "var(--surface-2)", color: enabled ? "var(--green)" : "var(--text-secondary)" }}>
                        {enabled ? "Enabled" : "Paused"}
                      </span>
                    </button>
                  );
                })
              )}
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
