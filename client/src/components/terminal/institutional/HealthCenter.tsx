"use client";

import { useEffect, useState } from "react";
import { Metric, TerminalCard } from "@/components/terminal/institutional/TerminalCard";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import { PageHeader } from "@/components/ui/PageHeader";
import { StatusChip } from "@/components/ui/StatusChip";

type HealthCheck = {
  name: string;
  status: "ok" | "warn" | "fail" | "unknown";
  detail: string;
};

type HealthPayload = {
  ok?: boolean;
  status?: string;
  checks?: Record<string, unknown>;
  mongo?: { configured?: boolean; ping_ok?: boolean; ping_ms?: number };
  engine?: { reachable?: boolean; ping_ms?: number; error?: string | null };
  env?: { blockers?: number; warnings?: number };
};

function tone(status: HealthCheck["status"]) {
  if (status === "ok") return "positive" as const;
  if (status === "warn") return "warning" as const;
  if (status === "fail") return "negative" as const;
  return "neutral" as const;
}

export function HealthCenter() {
  const [data, setData] = useState<HealthPayload | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const res = await fetch("/api/system/health", { cache: "no-store" });
        const json = (await res.json()) as HealthPayload;
        if (!cancelled) {
          setData(json);
          setError(res.ok ? null : `HTTP ${res.status}`);
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "Unavailable");
      }
    };
    load();
    const id = setInterval(load, 15_000);
    return () => { cancelled = true; clearInterval(id); };
  }, []);

  const checks: HealthCheck[] = [];
  if (data) {
    checks.push({
      name: "Environment",
      status: (data.env?.blockers ?? 0) > 0 ? "fail" : (data.env?.warnings ?? 0) > 0 ? "warn" : "ok",
      detail: `${data.env?.blockers ?? 0} blockers · ${data.env?.warnings ?? 0} warnings`,
    });
    checks.push({
      name: "MongoDB",
      status: !data.mongo?.configured ? "fail" : data.mongo.ping_ok ? "ok" : "fail",
      detail: data.mongo?.ping_ok ? `${data.mongo.ping_ms ?? 0}ms ping` : "unreachable",
    });
    checks.push({
      name: "Go Engine",
      status: data.engine?.reachable ? "ok" : "fail",
      detail: data.engine?.reachable ? `${data.engine.ping_ms ?? 0}ms` : (data.engine?.error ?? "offline"),
    });
    checks.push({
      name: "Overall",
      status: data.status === "operational" ? "ok" : data.status === "degraded" ? "warn" : "fail",
      detail: data.status ?? "unknown",
    });
  }

  return (
    <div className="m3-page-stack">
      <PageHeader
        title="Health"
        subtitle="System health checks and OMS reconciliation chain"
        actions={
          data ? (
            <StatusChip
              label={data.status ?? "unknown"}
              tone={data.status === "operational" ? "gain" : data.status === "degraded" ? "warn" : "loss"}
              pulse={data.status === "operational"}
            />
          ) : undefined
        }
      />
      <div className="grid gap-3 xl:grid-cols-2">
      <TerminalCard title="System Health" subtitle="/api/system/health · 15s poll">
        {error && !data ? (
          <TerminalNoData label={error.toUpperCase()} />
        ) : !data ? (
          <TerminalNoData label="Loading health checks" />
        ) : (
          <div className="grid gap-2 sm:grid-cols-2">
            {checks.map((c) => (
              <Metric key={c.name} label={c.name} value={c.detail} tone={tone(c.status)} />
            ))}
          </div>
        )}
      </TerminalCard>
      <TerminalCard title="OMS & Reconciliation" subtitle="Trade Engine authority chain">
        <div className="space-y-2 text-xs text-slate-500">
          <p>Execution authority flows from the engine through MongoDB into the Command Center UI.</p>
          <p>All fills originate from engine OMS v3 and reconciliation checks run continuously.</p>
        </div>
      </TerminalCard>
      </div>
    </div>
  );
}
