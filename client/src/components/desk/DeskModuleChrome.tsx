"use client";

import type { ReactNode } from "react";
import { DeskThemeToggle } from "@/components/desk/DeskThemeToggle";
import {
  DeskAppBar,
  DeskCard,
  DeskChip,
  DeskEmptyState,
  DeskSectionHeader,
  DeskShell,
  type DeskEngineStatus,
} from "@/components/desk/ui";

type DeskModuleChromeProps = {
  title: string;
  tagline: string;
  description: string;
  chips: string[];
  status: DeskEngineStatus;
  paperEquity?: number;
  paperCurrency?: "USD" | "INR";
  comingSoon?: boolean;
  loading?: boolean;
  deskDark?: boolean;
  onToggleDeskDark?: () => void;
  authSlot?: ReactNode;
  children?: ReactNode;
};

export function DeskModuleChrome({
  title,
  tagline,
  description,
  chips,
  status,
  paperEquity,
  paperCurrency = "USD",
  comingSoon = false,
  loading = false,
  deskDark = false,
  onToggleDeskDark,
  authSlot,
  children,
}: DeskModuleChromeProps) {
  return (
    <DeskShell
      loading={loading}
      appBar={
        <DeskAppBar
          title={title}
          subtitle={tagline}
          equity={paperEquity}
          equityCurrency={paperCurrency}
          status={status}
          authSlot={authSlot}
          themeToggle={
            onToggleDeskDark ? <DeskThemeToggle dark={deskDark} onToggle={onToggleDeskDark} /> : undefined
          }
        />
      }
    >
      <DeskCard>
        <DeskSectionHeader title={title} subtitle={tagline} />
        <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 12 }}>
          <DeskChip tone="primary" title="Live engine paper wallet for this module">
            Paper
          </DeskChip>
          {chips.map((c) => (
            <DeskChip key={c}>{c}</DeskChip>
          ))}
        </div>
        <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", marginBottom: comingSoon ? 16 : 0, maxWidth: 720 }}>
          {description}
        </p>
        {comingSoon ? (
          <DeskEmptyState
            title="Module coming soon"
            subtitle="This workspace tab is reserved. Switch to BTC Future Trading or another live desk."
          />
        ) : null}
      </DeskCard>
      {!comingSoon ? children : null}
    </DeskShell>
  );
}
