"use client";

import type { ReactNode } from "react";

type EmptyStateProps = {
  title: string;
  subtitle?: string;
  action?: ReactNode;
  icon?: ReactNode;
};

type SkeletonBlockProps = {
  width?: string | number;
  height?: number;
  rounded?: number;
};

export function EmptyState({ title, subtitle, action, icon }: EmptyStateProps) {
  return (
    <div className="m3-empty-state" role="status">
      {icon ? <div className="m3-empty-state__icon" aria-hidden>{icon}</div> : null}
      <p className="m3-empty-state__title">{title}</p>
      {subtitle ? <p className="m3-empty-state__subtitle">{subtitle}</p> : null}
      {action}
    </div>
  );
}

export function SkeletonBlock({ width = "100%", height = 16, rounded = 4 }: SkeletonBlockProps) {
  return (
    <div
      aria-hidden
      style={{
        width,
        height,
        borderRadius: rounded,
        background:
          "linear-gradient(90deg, var(--surface-3) 25%, var(--surface-4) 50%, var(--surface-3) 75%)",
        backgroundSize: "200% 100%",
        animation: "skeleton-shimmer 1.5s infinite",
      }}
    />
  );
}
