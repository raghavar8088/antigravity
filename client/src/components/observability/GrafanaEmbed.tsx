"use client";

/**
 * GrafanaEmbed — renders a Grafana iframe when NEXT_PUBLIC_GRAFANA_URL is set,
 * otherwise shows a native metrics fallback fetched from the Go engine health endpoint.
 */

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/Badge";
import { GaugeBar } from "@/components/ui/GaugeBar";
import { fmtISTClock } from "@/lib/istTime";

interface NativeMetrics {
  engineUptimeS?: number;
  wsConnections?: number;
  apiLatencyP50Ms?: number;
  apiLatencyP95Ms?: number;
  apiLatencyP99Ms?: number;
  dbConnPoolUsed?: number;
  dbConnPoolTotal?: number;
  redisHitRatePct?: number;
  recentErrors?: Array<{ timestamp: string; message: string; path: string }>;
}

function formatUptime(s: number): string {
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  return d > 0 ? `${d}d ${h}h ${m}m` : h > 0 ? `${h}h ${m}m` : `${m}m`;
}

export function GrafanaEmbed({ height = 600 }: { height?: number }) {
  const grafanaUrl = process.env.NEXT_PUBLIC_GRAFANA_URL;
  const [metrics, setMetrics] = useState<NativeMetrics | null>(null);
  const [offline, setOffline] = useState(false);

  useEffect(() => {
    if (grafanaUrl) return;
    const load = async () => {
      try {
        const res = await fetch("/api/engine/api/health", { cache: "no-store" });
        if (res.ok) { setMetrics(await res.json()); setOffline(false); }
        else setOffline(true);
      } catch { setOffline(true); }
    };
    load();
    const id = setInterval(load, 15_000);
    return () => clearInterval(id);
  }, [grafanaUrl]);

  if (grafanaUrl) {
    return (
      <div className="flex flex-col">
        <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--color-border)]">
          <span className="text-[11px] font-medium text-[var(--color-text-secondary)]">Grafana Dashboard</span>
          <a href={grafanaUrl} target="_blank" rel="noopener noreferrer"
            className="text-[10px] text-[var(--color-info)] hover:underline">
            Open in Grafana ↗
          </a>
        </div>
        <iframe
          src={grafanaUrl}
          style={{ height, border: "none", width: "100%" }}
          title="Grafana Dashboard"
          sandbox="allow-same-origin allow-scripts allow-popups allow-forms"
        />
      </div>
    );
  }

  if (offline) {
    return (
      <div className="rounded-lg border border-[rgba(249,115,22,0.3)] bg-[rgba(249,115,22,0.08)] p-4 text-[13px] text-[var(--color-warning)]">
        ⚠ Engine offline — native metrics unavailable.
        Set <code className="text-[var(--color-info)]">NEXT_PUBLIC_GRAFANA_URL</code> to embed Grafana.
      </div>
    );
  }

  if (!metrics) return null;

  const p50  = metrics.apiLatencyP50Ms  ?? 0;
  const p95  = metrics.apiLatencyP95Ms  ?? 0;
  const p99  = metrics.apiLatencyP99Ms  ?? 0;
  const poolUsed  = metrics.dbConnPoolUsed  ?? 0;
  const poolTotal = metrics.dbConnPoolTotal ?? 1;

  return (
    <div className="space-y-4">
      <div className="text-[11px] text-[var(--color-text-muted)]">
        Set <code className="text-[var(--color-info)]">NEXT_PUBLIC_GRAFANA_URL</code> to embed Grafana. Showing native engine metrics:
      </div>

      <div className="grid grid-cols-3 gap-3">
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-3">
          <div className="text-[10px] text-[var(--color-text-muted)] mb-1">Engine Uptime</div>
          <div className="text-[18px] font-mono font-semibold text-[var(--color-profit)]">
            {formatUptime(metrics.engineUptimeS ?? 0)}
          </div>
        </div>
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-3">
          <div className="text-[10px] text-[var(--color-text-muted)] mb-1">WS Connections</div>
          <div className="text-[18px] font-mono font-semibold text-[var(--color-text-primary)]">
            {metrics.wsConnections ?? 0}
          </div>
        </div>
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-3">
          <div className="text-[10px] text-[var(--color-text-muted)] mb-1">Redis Hit Rate</div>
          <GaugeBar value={metrics.redisHitRatePct ?? 0} valueLabel={`${(metrics.redisHitRatePct ?? 0).toFixed(1)}%`} height={6} />
        </div>
      </div>

      <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-4">
        <div className="text-[11px] font-medium text-[var(--color-text-secondary)] mb-3">API Latency</div>
        <div className="flex items-center gap-6">
          {([["P50", p50],["P95", p95],["P99", p99]] as [string, number][]).map(([label, ms]) => (
            <div key={label} className="text-center">
              <div className="text-[10px] text-[var(--color-text-muted)] mb-1">{label}</div>
              <Badge variant={ms < 50 ? "profit" : ms < 200 ? "caution" : "loss"} size="md">{ms}ms</Badge>
            </div>
          ))}
        </div>
      </div>

      <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-4">
        <div className="text-[11px] font-medium text-[var(--color-text-secondary)] mb-2">DB Connection Pool</div>
        <GaugeBar
          value={Math.round((poolUsed / poolTotal) * 100)}
          label={`${poolUsed} / ${poolTotal} connections`}
          height={8}
        />
      </div>

      {(metrics.recentErrors ?? []).length > 0 && (
        <div className="rounded-lg border border-[rgba(239,68,68,0.2)] bg-[rgba(239,68,68,0.04)] p-4">
          <div className="text-[11px] font-medium text-[var(--color-loss)] mb-2">Recent Errors (last 20)</div>
          <div className="space-y-1 max-h-48 overflow-y-auto font-mono text-[10px]">
            {(metrics.recentErrors ?? []).slice(0, 20).map((e, i) => (
              <div key={i} className="flex items-start gap-2 border-b border-[rgba(239,68,68,0.1)] pb-1">
                <span className="text-[var(--color-text-muted)] whitespace-nowrap">
                  {fmtISTClock(e.timestamp)}
                </span>
                <span className="text-[var(--color-caution)]">{e.path}</span>
                <span className="text-[var(--color-loss)]">{e.message}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
