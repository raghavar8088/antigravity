"use client";

import type { ReactNode } from "react";
import { formatDeskInr, formatDeskUsd } from "@/lib/deskFormat";
import { DeskChip } from "./DeskChip";
import { useDeskMounted } from "@/hooks/useDeskMounted";
import { StatusBadge, type DeskEngineStatus } from "./StatusBadge";
import { cn } from "./cn";

type DeskAppBarProps = {
  workspaceName?: string;
  title: string;
  subtitle?: string;
  equity?: number;
  /** Shown under the paper balance amount (e.g. session PnL hint). */
  equityDetail?: string;
  equityCurrency?: "USD" | "INR";
  equityLabel?: string;
  status: DeskEngineStatus;
  trailing?: ReactNode;
  authSlot?: ReactNode;
  themeToggle?: ReactNode;
  className?: string;
};

function formatDeskEquity(value: number, currency: "USD" | "INR") {
  return currency === "INR" ? formatDeskInr(value) : formatDeskUsd(value);
}

export function DeskAppBar({
  workspaceName = "in.loop.com",
  title,
  subtitle,
  equity,
  equityDetail,
  equityCurrency = "USD",
  equityLabel = "Paper desk balance",
  status,
  trailing,
  authSlot,
  themeToggle,
  className,
}: DeskAppBarProps) {
  const mounted = useDeskMounted();
  const paperTooltip =
    "Live paper wallet for this module (engine state). Workspace settings capital is display-only.";

  return (
    <header
      className={cn("desk-app-bar", className)}
      style={{
        position: "sticky",
        top: 0,
        zIndex: 100,
        background: "var(--desk-surface)",
        borderBottom: "1px solid var(--desk-outline)",
        boxShadow: "var(--desk-elevation-1)",
      }}
    >
      <div
        style={{
          maxWidth: "var(--desk-page-max)",
          margin: "0 auto",
          display: "flex",
          alignItems: "center",
          gap: "var(--desk-space-4)",
          padding: "0 var(--desk-space-5)",
          minHeight: "var(--desk-header-height)",
          flexWrap: "wrap",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 12, minWidth: 0, flex: "1 1 200px" }}>
          <img
            src="/branding/in-loop-logo.png"
            alt={workspaceName}
            width={120}
            height={32}
            style={{ height: 32, width: "auto", flexShrink: 0 }}
          />
          <div style={{ minWidth: 0 }}>
            <p className="desk-label-md" style={{ marginBottom: 2 }}>
              {workspaceName}
            </p>
            <p className="desk-title-md" style={{ lineHeight: 1.2 }}>
              {title}
            </p>
            {subtitle ? (
              <p className="desk-label-md" style={{ fontWeight: 400, marginTop: 2 }}>
                {subtitle}
              </p>
            ) : null}
          </div>
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap", justifyContent: "center" }}>
          <StatusBadge status={status} />
        </div>

        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "var(--desk-space-3)",
            marginLeft: "auto",
            flexWrap: "wrap",
            justifyContent: "flex-end",
          }}
        >
          {equity !== undefined ? (
            <div style={{ textAlign: "right" }} title={paperTooltip}>
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "flex-end",
                  gap: 6,
                  marginBottom: 2,
                }}
              >
                <DeskChip tone="primary" title={paperTooltip}>
                  Paper
                </DeskChip>
                <p className="desk-label-md">{equityLabel}</p>
              </div>
              <p className="desk-mono desk-title-md">
                {mounted ? formatDeskEquity(equity, equityCurrency) : "—"}
              </p>
              {equityDetail ? (
                <p className="desk-label-md" style={{ fontWeight: 400, marginTop: 2 }}>
                  {equityDetail}
                </p>
              ) : null}
            </div>
          ) : null}
          {authSlot}
          {themeToggle}
          {trailing}
        </div>
      </div>
    </header>
  );
}
