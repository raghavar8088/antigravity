"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";

type DeskBannerVariant = "info" | "warning" | "error" | "success";

type DeskBannerProps = {
  variant?: DeskBannerVariant;
  title?: string;
  children: ReactNode;
  className?: string;
};

const variantBg: Record<DeskBannerVariant, string> = {
  info: "var(--desk-primary-container)",
  warning: "var(--desk-warning-container)",
  error: "var(--desk-error-container)",
  success: "var(--desk-success-container)",
};

const variantColor: Record<DeskBannerVariant, string> = {
  info: "var(--desk-on-primary-container)",
  warning: "var(--desk-warning)",
  error: "var(--desk-error)",
  success: "var(--desk-success)",
};

export function DeskBanner({ variant = "info", title, children, className }: DeskBannerProps) {
  return (
    <div
      role="status"
      className={cn("desk-banner", className)}
      style={{
        padding: "var(--desk-space-3) var(--desk-space-4)",
        borderRadius: "var(--desk-radius-card)",
        background: variantBg[variant],
        color: variantColor[variant],
        fontSize: "0.8125rem",
        lineHeight: 1.45,
      }}
    >
      {title ? <p style={{ fontWeight: 600, marginBottom: 4 }}>{title}</p> : null}
      {children}
    </div>
  );
}
