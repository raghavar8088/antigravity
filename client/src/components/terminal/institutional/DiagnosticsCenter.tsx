"use client";

import { useEffect, useState } from "react";
import { Metric, TerminalCard } from "@/components/terminal/institutional/TerminalCard";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";

type DiagnosticsPayload = {
  ok?: boolean;
  engine?: Record<string, unknown>;
  error?: string;
};

export function DiagnosticsCenter() {
  const [engineDiag, setEngineDiag] = useState<DiagnosticsPayload | null>(null);
  const [envBlockers, setEnvBlockers] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      const [diag, health] = await Promise.all([
        fetch("/api/paper-desk/diagnostics", { cache: "no-store" }).then((r) => (r.ok ? r.json() : { ok: false })).catch(() => ({ ok: false })),
        fetch("/api/system/health", { cache: "no-store" }).then((r) => (r.ok ? r.json() : null)).catch(() => null),
      ]);
      if (cancelled) return;
      setEngineDiag(diag);
      setEnvBlockers(health?.env?.blockers ?? null);
    };
    load();
    const id = setInterval(load, 30_000);
    return () => { cancelled = true; clearInterval(id); };
  }, []);

  return (
    <div className="grid gap-3 xl:grid-cols-2">
      <TerminalCard title="Engine Diagnostics" subtitle="/api/paper-desk/diagnostics → Go engine proxy">
        {!engineDiag ? (
          <TerminalNoData label="LOADING..." />
        ) : engineDiag.ok === false ? (
          <TerminalNoData label="ENGINE DIAGNOSTICS UNAVAILABLE" />
        ) : (
          <pre className="max-h-80 overflow-auto rounded-lg border border-zinc-800 bg-zinc-950/60 p-3 font-mono text-[10px] text-zinc-400">
            {JSON.stringify(engineDiag.engine ?? engineDiag, null, 2)}
          </pre>
        )}
      </TerminalCard>
      <TerminalCard title="Environment & Worker" subtitle="Startup validation · cron worker">
        <div className="grid gap-2">
          <Metric label="Env Blockers" value={envBlockers != null ? String(envBlockers) : "—"} tone={envBlockers && envBlockers > 0 ? "negative" : "positive"} />
          <p className="text-xs text-zinc-500">
            Cron tick: <span className="font-mono text-zinc-400">/api/cron/paper-desk-tick</span>
          </p>
          <p className="text-xs text-zinc-500">
            Worker probe: <span className="font-mono text-zinc-400">/api/health/desk-worker</span>
          </p>
        </div>
      </TerminalCard>
    </div>
  );
}
