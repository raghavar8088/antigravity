"use client";

import type { ReactNode } from "react";
import { Skeleton } from "./Skeleton";

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

/** @deprecated Consolidated onto the shared Skeleton component — this is now a thin wrapper. */
export function SkeletonBlock({ width = "100%", height = 16, rounded = 4 }: SkeletonBlockProps) {
  return <Skeleton width={width} height={height} rounded={rounded} />;
}
