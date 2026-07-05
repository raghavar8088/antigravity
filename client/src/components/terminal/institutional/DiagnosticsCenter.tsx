"use client";

import { useEffect, useState } from "react";
import { Metric, TerminalCard } from "@/components/terminal/institutional/TerminalCard";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import { PageHeader } from "@/components/ui/PageHeader";

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

  const diagnosticPreview = mockDiag
    ? JSON.stringify(mockDiag, null, 2)
        .replace(/mock-trading/gi, "Trade Engine")
        .replace(/mock_trading/gi, "trade_engine")
        .replace(/MAIN_ENGINE/g, "TRADE_ENGINE")
    : "";

  return (
    <div className="m3-page-stack">
      <PageHeader title="Diagnostics" subtitle="Trade Engine state snapshot and environment validation" />
      <div className="grid gap-3 xl:grid-cols-2">
      <TerminalCard title="Trade Engine State" subtitle="Live snapshot preview">
        {!mockDiag ? (
          <TerminalNoData label="Loading diagnostics" />
        ) : mockDiag.ok === false ? (
          <TerminalNoData label="Trade Engine snapshot unavailable" />
        ) : (
          <pre className="max-h-80 overflow-auto rounded-2xl border border-slate-200 bg-slate-50 p-4 font-mono text-[11px] text-slate-600">
            {diagnosticPreview}
          </pre>
        )}
      </TerminalCard>
      <TerminalCard title="Environment" subtitle="Startup validation">
        <div className="grid gap-2">
          <Metric label="Env Blockers" value={envBlockers != null ? String(envBlockers) : "—"} tone={envBlockers && envBlockers > 0 ? "negative" : "positive"} />
          <p className="text-xs text-slate-500">
            Execution authority: <span className="font-mono text-slate-700">Trade Engine</span>
          </p>
          <p className="text-xs text-slate-500">
            Signal trace: <span className="font-mono text-slate-700">/api/strategy-signal-trace</span>
          </p>
        </div>
      </TerminalCard>
      </div>
    </div>
  );
}
