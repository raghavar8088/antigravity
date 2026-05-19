"use client";

import { DeskBanner } from "@/components/desk/ui/DeskBanner";
import { DeskCard } from "@/components/desk/ui/DeskCard";
import { DeskChip } from "@/components/desk/ui/DeskChip";
import { DeskCopyButton } from "@/components/desk/ui/DeskCopyButton";
import { DeskTabs, type DeskTabItem } from "@/components/desk/ui/DeskTabs";
import { BTC_FUTURE_TRADING_STRATEGY_IDS } from "@/lib/btcFutureTradingRoster";
import { isSupabaseAuthConfigured } from "@/lib/supabase/client";

type DashboardGroup = "crypto" | "india";
type DashboardModule =
  | "dashboard"
  | "engine"
  | "history"
  | "options"
  | "options-selling"
  | "chain"
  | "nifty"
  | "niftySelling"
  | "niftyBees"
  | "btcFuturesScalper"
  | "btcFutureTrading";

type ModuleTab = Omit<DeskTabItem<DashboardModule>, "trailing">;

const CRYPTO_MODULES: ModuleTab[] = [
  { key: "options", label: "BTC Option Buying" },
  { key: "options-selling", label: "BTC Option Selling" },
  { key: "btcFuturesScalper", label: "Future Trading" },
  { key: "btcFutureTrading", label: "BTC Future Trading" },
];

const INDIA_MODULES: ModuleTab[] = [
  { key: "nifty", label: "Nifty 50 Option Buying" },
  { key: "niftySelling", label: "Nifty Option Selling" },
  { key: "niftyBees", label: "Nifty BEES Scalper" },
];

/**
 * Resolve the deep-link URL for a workspace module. Modules with dedicated
 * routes return the canonical path; the rest land on the workspace root
 * with a `module` query param so the dashboard can preselect the tab when
 * deep-linked.
 */
export function moduleDeepLinkUrl(module: DashboardModule): string {
  const origin = typeof window !== "undefined" ? window.location.origin : "";
  switch (module) {
    case "btcFutureTrading":
      return `${origin}/btc-future-trading`;
    case "nifty":
      return `${origin}/nifty-options`;
    case "niftySelling":
      return `${origin}/nifty-selling`;
    case "niftyBees":
      return `${origin}/nifty-bees`;
    case "btcFuturesScalper":
    case "options":
    case "options-selling":
      return `${origin}/?module=${module}`;
    default:
      return origin || "/";
  }
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

export function moduleStatusChips(module: DashboardModule): string[] {
  switch (module) {
    case "btcFutureTrading":
      return ["Paper", "25× leverage", `${BTC_FUTURE_TRADING_STRATEGY_IDS.length} strategies`, "1 market"];
    case "btcFuturesScalper":
      return ["Paper", "25× leverage", "130 strategies", "Multi-asset"];
    case "options":
      return ["Paper", "50 strategies", "BTC options"];
    case "options-selling":
      return ["Paper", "Premium decay", "BTC shorts"];
    case "nifty":
      return ["Paper", "NSE live", "NIFTY options"];
    case "niftySelling":
      return ["Paper", "NIFTY selling", "Theta"];
    case "niftyBees":
      return ["Paper", "BEES ETF", "30 strategies"];
    default:
      return ["Paper desk"];
  }
}

type WorkspaceNavPanelProps = {
  activeGroup: DashboardGroup;
  activeModule: DashboardModule;
  onGroupChange: (group: DashboardGroup) => void;
  onModuleChange: (module: DashboardModule) => void;
  actionsEnabled: boolean;
  onActionsEnabledChange: (enabled: boolean) => void;
  actionToggleTitle: string;
  moduleDescription: string;
};

export function WorkspaceNavPanel({
  activeGroup,
  activeModule,
  onGroupChange,
  onModuleChange,
  actionsEnabled,
  onActionsEnabledChange,
  actionToggleTitle,
  moduleDescription,
}: WorkspaceNavPanelProps) {
  const cloudConfigured = isSupabaseAuthConfigured();
  const chips = moduleStatusChips(activeModule);

  return (
    <DeskCard padding="none" elevation={1}>
      <div style={{ padding: "0 var(--desk-space-4)" }}>
        <DeskTabs
          variant="primary"
          items={[
            { key: "crypto", label: "Crypto" },
            { key: "india", label: "Indian Market" },
          ]}
          active={activeGroup}
          onChange={(g) => {
            onGroupChange(g);
            onModuleChange(g === "crypto" ? "options" : "nifty");
          }}
        />
      </div>

      <div
        style={{
          display: "flex",
          flexWrap: "wrap",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 12,
          padding: "var(--desk-space-3) var(--desk-space-4)",
          borderTop: "1px solid var(--desk-outline-variant)",
        }}
      >
        <div style={{ overflowX: "auto", flex: "1 1 200px", minWidth: 0 }}>
          <DeskTabs
            variant="secondary"
            items={moduleItemsWithCopy(activeGroup === "crypto" ? CRYPTO_MODULES : INDIA_MODULES)}
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

      {!cloudConfigured && (activeModule === "btcFuturesScalper" || activeModule === "btcFutureTrading") ? (
        <div style={{ padding: "0 var(--desk-space-4) var(--desk-space-4)" }}>
          <DeskBanner variant="info" title="Cloud sync disabled">
            Sign in to sync paper trades across devices. See the{" "}
            <a href="#desk-auth-setup" style={{ textDecoration: "underline", color: "inherit" }}>
              Auth setup
            </a>{" "}
            section in README (Supabase magic link + RLS).
          </DeskBanner>
        </div>
      ) : null}
    </DeskCard>
  );
}
