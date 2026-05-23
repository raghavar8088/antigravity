"use client";

import type { ReactNode } from "react";
import { DeskMetricTile } from "./ui/DeskMetricTile";
import { cn } from "./ui/cn";

export type DeskHeroMetric = {
  label: string;
  value: ReactNode;
  detail?: ReactNode;
  valueClassName?: string;
  highlight?: boolean;
};

type DeskHeroStripProps = {
  metrics: DeskHeroMetric[];
  className?: string;
};

export function DeskHeroStrip({ metrics, className }: DeskHeroStripProps) {
  return (
    <div className={cn("desk-hero-strip", className)}>
      {metrics.map((m, i) => (
        <div
          key={m.label}
          className={cn(i === 0 && "desk-hero-strip__highlight")}
        >
          <DeskMetricTile
            label={m.label}
            value={m.value}
            detail={m.detail}
            valueClassName={m.valueClassName}
            highlight={m.highlight}
            compact={i > 0}
          />
        </div>
      ))}
    </div>
  );
}
