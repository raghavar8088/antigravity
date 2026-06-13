"use client";

import { ObservabilityCenter } from "@/components/terminal/institutional/ObservabilityCenter";
import { GrafanaEmbed } from "@/components/observability/GrafanaEmbed";

export default function ObservabilityPage() {
  return (
    <div className="flex flex-col gap-0">
      <ObservabilityCenter />
      <div className="px-4 pb-6 pt-2">
        <div className="text-[12px] font-semibold text-[var(--color-text-secondary)] mb-3 uppercase tracking-wide">Infrastructure Metrics</div>
        <GrafanaEmbed height={500} />
      </div>
    </div>
  );
}
