"use client";

import { useEffect, useState } from "react";
import { Metric, TerminalCard } from "@/components/terminal/institutional/TerminalCard";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";

type ObservabilityPayload = {
  ok?: boolean;
  status?: string;
  mongo?: { ping_ok?: boolean; ping_ms?: number };
  engine?: { reachable?: boolean; ping_ms?: number };
  latency_ms?: number;
};

export function ObservabilityCenter() {
  const [health, setHealth] = useState<ObservabilityPayload | null>(null);
  const [ribbon, setRibbon] = useState<{ overall?: string; items?: Array<{ label: string; status: string; value: string }> } | null>(null);
  const [deskWorker, setDeskWorker] = useState<{ ok?: boolean; fresh?: boolean } | null>(null);

  useEffect(() => {
    let cancelled = false;
    const poll = async () => {
      const [h, r, w] = await Promise.all([
        fetch("/api/system/health", { cache: "no-store" }).then((res) => (res.ok ? res.json() : null)).catch(() => null),
        fetch("/api/risk-ribbon", { cache: "no-store" }).then((res) => (res.ok ? res.json() : null)).catch(() => null),
        fetch("/api/health/desk-worker", { cache: "no-store" }).then((res) => (res.ok ? res.json() : null)).catch(() => null),
      ]);
      if (cancelled) return;
      setHealth(h);
      setRibbon(r);
      setDeskWorker(w);
    };
    poll();
    const id = setInterval(poll, 10_000);
    return () => { cancelled = true; clearInterval(id); };
  }, []);

  const feedItems = ribbon?.items?.filter((i) =>
    ["MARKET DATA", "ENGINE", "DATABASE", "OMS", "EXECUTION", "WATCHDOG"].includes(i.label),
  ) ?? [];

  return (
    <div className="space-y-3">
      <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
        <Metric
          label="API Health"
          value={health?.status?.toUpperCase() ?? "—"}
          tone={health?.status === "operational" ? "positive" : health?.status === "degraded" ? "warning" : "negative"}
        />
        <Metric label="Engine Latency" value={health?.engine?.ping_ms != null ? `${health.engine.ping_ms}ms` : "—"} />
        <Metric label="Mongo Latency" value={health?.mongo?.ping_ms != null ? `${health.mongo.ping_ms}ms` : "—"} />
        <Metric
          label="Worker Heartbeat"
          value={deskWorker?.fresh ? "FRESH" : deskWorker?.ok ? "STALE" : "—"}
          tone={deskWorker?.fresh ? "positive" : deskWorker?.ok ? "warning" : "negative"}
        />
      </div>

      <TerminalCard title="Feed & Service Health" subtitle="/api/risk-ribbon · 10s poll">
        {feedItems.length === 0 ? (
          <TerminalNoData label="NO FEED DATA" />
        ) : (
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {feedItems.map((item) => (
              <Metric
                key={item.label}
                label={item.label}
                value={item.value}
                tone={item.status === "GREEN" ? "positive" : item.status === "AMBER" ? "warning" : item.status === "RED" ? "negative" : "neutral"}
              />
            ))}
          </div>
        )}
      </TerminalCard>

      <TerminalCard title="Queue & Execution" subtitle="OMS pipeline visibility">
        <div className="grid gap-2 sm:grid-cols-3">
          <Metric label="OMS" value={ribbon?.items?.find((i) => i.label === "OMS")?.value ?? "—"} />
          <Metric label="Reconciliation" value={ribbon?.items?.find((i) => i.label === "RECON")?.value ?? "—"} />
          <Metric label="Kill Switch" value={ribbon?.items?.find((i) => i.label === "KILL SWITCH")?.value ?? "—"} tone="warning" />
        </div>
      </TerminalCard>
    </div>
  );
}
