"use client";

import { cn } from "./cn";

type SkeletonProps = {
  className?: string;
  width?: string | number;
  height?: string | number;
  rounded?: boolean;
};

export function Skeleton({ className, width, height = 16, rounded = true }: SkeletonProps) {
  return (
    <div
      className={cn("m3-skeleton", rounded && "m3-skeleton--rounded", className)}
      style={{ width, height }}
      aria-hidden
    />
  );
}

export function SkeletonCard({ rows = 3 }: { rows?: number }) {
  return (
    <div className="m3-surface-card" aria-busy="true" aria-label="Loading">
      <div className="m3-surface-card__header">
        <Skeleton width="40%" height={14} />
      </div>
      <div className="m3-surface-card__body" style={{ display: "grid", gap: "var(--m3-space-3)" }}>
        {Array.from({ length: rows }).map((_, i) => (
          <Skeleton key={i} width="100%" height={12} />
        ))}
      </div>
    </div>
  );
}
