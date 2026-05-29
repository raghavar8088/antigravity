"use client";

import { DeskBanner } from "@/components/desk/ui/DeskBanner";
import { DeskCard } from "@/components/desk/ui/DeskCard";
import { DeskChip } from "@/components/desk/ui/DeskChip";
import { DeskCopyButton } from "@/components/desk/ui/DeskCopyButton";
import { DeskTabs, type DeskTabItem } from "@/components/desk/ui/DeskTabs";
import { BTC_FUTURE_TRADING_STRATEGY_IDS } from "@/lib/btcFutureTradingRoster";
import { isSupabaseAuthConfigured } from "@/lib/supabase/client";

export type DashboardModule = "btcFutureTrading" | "mockTrading";

type ModuleTab = Omit<DeskTabItem<DashboardModule>, "trailing">;

const MODULES: ModuleTab[] = [
  { key: "btcFutureTrading", label: "BTC Future Trading" },
  { key: "mockTrading", label: "Mock Trading" },
];

const MODULE_PATHS: Record<DashboardModule, string> = {
  btcFutureTrading: "/btc-future-trading",
  mockTrading: "/mock-trading",
};

export function moduleDeepLinkUrl(module: DashboardModule = "btcFutureTrading"): string {
  const origin = typeof window !== "undefined" ? window.location.origin : "";
  return `${origin}${MODULE_PATHS[module]}`;
}

function moduleItemsWithCopy(items: ModuleTab[]): DeskTabItem<DashboardModule>[] {
  return items.map((item) => ({
    ...item,
    trailing: (
      <DeskCopyButton
        text={moduleDeepLinkUrl(item.key)}
        ariaLabel={`Copy link to ${item.label}`}
      />
    ),
  }));
}

export function moduleStatusChips(): string[] {
  return ["Paper", "25× leverage", `${BTC_FUTURE_TRADING_STRATEGY_IDS.length} strategies`, "1 market"];
}

type WorkspaceNavPanelProps = {
  activeModule: DashboardModule;
  onModuleChange: (module: DashboardModule) => void;
  actionsEnabled: boolean;
  onActionsEnabledChange: (enabled: boolean) => void;
  actionToggleTitle: string;
  moduleDescription: string;
};

export function WorkspaceNavPanel({
  activeModule,
  onModuleChange,
  actionsEnabled,
  onActionsEnabledChange,
  actionToggleTitle,
  moduleDescription,
}: WorkspaceNavPanelProps) {
  const cloudConfigured = isSupabaseAuthConfigured();
  const chips = moduleStatusChips();

  return (
    <DeskCard padding="none" elevation={1}>
      <div
        style={{
          display: "flex",
          flexWrap: "wrap",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 12,
          padding: "var(--desk-space-3) var(--desk-space-4)",
        }}
      >
        <div style={{ overflowX: "auto", flex: "1 1 200px", minWidth: 0 }}>
          <DeskTabs
            variant="secondary"
            items={moduleItemsWithCopy(MODULES)}
            active={activeModule}
            onChange={onModuleChange}
          />
        </div>

        <div
          style={{ display: "flex", alignItems: "center", gap: 8 }}
          title={actionToggleTitle}
        >
          <span className="desk-label-md">Controls</span>
          <div
            style={{
              display: "inline-flex",
              borderRadius: "var(--desk-radius-chip)",
              overflow: "hidden",
              border: "1px solid var(--desk-outline)",
              background: "var(--desk-surface-container)",
            }}
          >
            <button
              type="button"
              onClick={() => onActionsEnabledChange(false)}
              aria-pressed={!actionsEnabled}
              style={{
                padding: "8px 14px",
                minHeight: 36,
                border: "none",
                fontSize: "0.75rem",
                fontWeight: 500,
                cursor: "pointer",
                background: !actionsEnabled ? "var(--desk-surface)" : "transparent",
                color: !actionsEnabled ? "var(--desk-on-surface)" : "var(--desk-on-surface-variant)",
              }}
            >
              Locked
            </button>
            <button
              type="button"
              onClick={() => onActionsEnabledChange(true)}
              aria-pressed={actionsEnabled}
              style={{
                padding: "8px 14px",
                minHeight: 36,
                border: "none",
                fontSize: "0.75rem",
                fontWeight: 500,
                cursor: "pointer",
                background: actionsEnabled ? "var(--desk-success)" : "transparent",
                color: actionsEnabled ? "#fff" : "var(--desk-on-surface-variant)",
              }}
            >
              Actions
            </button>
          </div>
        </div>
      </div>

      <div
        style={{
          display: "flex",
          flexWrap: "wrap",
          gap: 8,
          padding: "var(--desk-space-3) var(--desk-space-4)",
          borderTop: "1px solid var(--desk-outline-variant)",
        }}
      >
        <DeskChip tone="success">Live</DeskChip>
        {chips.map((c) => (
          <DeskChip key={c}>{c}</DeskChip>
        ))}
      </div>

      <p
        className="desk-body-md"
        style={{
          padding: "var(--desk-space-3) var(--desk-space-4) var(--desk-space-4)",
          color: "var(--desk-on-surface-variant)",
          borderTop: "1px solid var(--desk-outline-variant)",
          margin: 0,
        }}
      >
        {moduleDescription}
      </p>

      {!cloudConfigured && (
        <div style={{ padding: "0 var(--desk-space-4) var(--desk-space-4)" }}>
          <DeskBanner variant="info" title="Cloud sync disabled">
            Sign in to sync paper trades across devices. See the{" "}
            <a href="#desk-auth-setup" style={{ textDecoration: "underline", color: "inherit" }}>
              Auth setup
            </a>{" "}
            section in README (Supabase magic link + RLS).
          </DeskBanner>
        </div>
      )}
    </DeskCard>
  );
}
