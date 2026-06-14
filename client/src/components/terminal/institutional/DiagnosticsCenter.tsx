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
  const [mockDiag, setMockDiag] = useState<DiagnosticsPayload | null>(null);
  const [envBlockers, setEnvBlockers] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      const [diag, health] = await Promise.all([
        fetch("/api/mock-trading/snapshot", { cache: "no-store" }).then((r) => (r.ok ? r.json() : { ok: false })).catch(() => ({ ok: false })),
        fetch("/api/system/health", { cache: "no-store" }).then((r) => (r.ok ? r.json() : null)).catch(() => null),
      ]);
      if (cancelled) return;
      setMockDiag(diag);
      setEnvBlockers(health?.env?.blockers ?? null);
    };
    load();
    const id = setInterval(load, 30_000);
    return () => { cancelled = true; clearInterval(id); };
  }, []);

  return (
    <div className="grid gap-3 xl:grid-cols-2">
      <TerminalCard title="Trade Engine State" subtitle="/api/mock-trading/snapshot">
        {!mockDiag ? (
          <TerminalNoData label="LOADING..." />
        ) : mockDiag.ok === false ? (
          <TerminalNoData label="TRADE ENGINE SNAPSHOT UNAVAILABLE" />
        ) : (
          <pre className="max-h-80 overflow-auto rounded-lg border border-zinc-800 bg-zinc-950/60 p-3 font-mono text-[10px] text-zinc-400">
            {JSON.stringify(mockDiag, null, 2)}
          </pre>
        )}
      </TerminalCard>
      <TerminalCard title="Environment" subtitle="Startup validation">
        <div className="grid gap-2">
          <Metric label="Env Blockers" value={envBlockers != null ? String(envBlockers) : "—"} tone={envBlockers && envBlockers > 0 ? "negative" : "positive"} />
          <p className="text-xs text-zinc-500">
            Execution authority: <span className="font-mono text-zinc-400">mock-trading</span>
          </p>
          <p className="text-xs text-zinc-500">
            Signal trace: <span className="font-mono text-zinc-400">/api/strategy-signal-trace</span>
          </p>
        </div>
      </TerminalCard>
    </div>
  );
}
