"use client";

import { useEffect, useState } from "react";
import { Metric, TerminalCard } from "@/components/terminal/institutional/TerminalCard";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";

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
    <div className="grid gap-3 xl:grid-cols-2">
      <TerminalCard title="System Health" subtitle="/api/system/health · 15s poll">
        {error && !data ? (
          <TerminalNoData label={error.toUpperCase()} />
        ) : !data ? (
          <TerminalNoData label="LOADING..." />
        ) : (
          <div className="grid gap-2 sm:grid-cols-2">
            {checks.map((c) => (
              <Metric key={c.name} label={c.name} value={c.detail.toUpperCase()} tone={tone(c.status)} />
            ))}
          </div>
        )}
      </TerminalCard>
      <TerminalCard title="OMS & Reconciliation" subtitle="Engine authority chain">
        <div className="space-y-2 text-xs text-zinc-400">
          <p>Execution authority: Go Engine → MongoDB → Command Center UI.</p>
          <p>Browser-side trade creation is permanently disabled. All fills originate from engine OMS v3.</p>
          <p className="font-mono text-[10px] text-zinc-500">Data: paper_state · paper_positions · paper_trades · paper_orders</p>
        </div>
      </TerminalCard>
    </div>
  );
}
