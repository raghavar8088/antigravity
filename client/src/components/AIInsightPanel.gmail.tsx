"use client";

import type { AIDecision, AuditLog } from "@/hooks/useAIInsights";

function MiniBar({ value, color }: { value: number; color: string }) {
  const pct = Math.max(0, Math.min(100, Math.round(value * 100)));
  return (
    <div className="flex items-center gap-2">
      <div className="h-1.5 flex-1 overflow-hidden rounded-full" style={{ background: "var(--surface-3)" }}>
        <div className="h-full rounded-full" style={{ width: `${pct}%`, background: color }} />
      </div>
      <span className="font-mono text-[11px] font-medium" style={{ color }}>{pct}%</span>
    </div>
  );
}

function VerdictPill({ action }: { action: string }) {
  const map: Record<string, { bg: string; color: string; label: string }> = {
    BUY: { bg: "var(--green-dim)", color: "var(--green)", label: "BUY" },
    SELL: { bg: "var(--red-dim)", color: "var(--red)", label: "SELL" },
    HOLD: { bg: "var(--surface-3)", color: "var(--text-secondary)", label: "HOLD" },
  };
  const current = map[action] ?? map.HOLD;
  return <span className="rounded-full px-3 py-1 text-[11px] font-medium" style={{ background: current.bg, color: current.color }}>{current.label}</span>;
}

function DecisionCard({ decision, compact = false, geminiEnabled }: { decision: AIDecision; compact?: boolean; geminiEnabled: boolean }) {
  const bull = decision.bullSignal;
  const bear = decision.bearSignal;
  const macro = decision.macroSignal;
  const risk = decision.riskVerdict;

  return (
    <div className="rounded-[20px] border p-4" style={{ borderColor: "var(--border)", background: compact ? "var(--surface-2)" : "var(--surface)" }}>
      <div className="mb-3 flex items-center justify-between gap-3">
        <div>
          <div className="text-[11px] font-medium uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>RAIG AI state</div>
          <div className="mt-1 flex items-center gap-2">
            <VerdictPill action={decision.finalAction} />
            <span className="font-mono text-xs" style={{ color: "var(--text-secondary)" }}>{decision.regime}</span>
          </div>
        </div>
        <span className="font-mono text-xs" style={{ color: "var(--text-secondary)" }}>${decision.price.toLocaleString(undefined, { maximumFractionDigits: 0 })}</span>
      </div>

      <div className={`grid gap-3 ${geminiEnabled && !compact ? "md:grid-cols-3" : "md:grid-cols-2"}`}>
        <div className="rounded-2xl p-3" style={{ background: "var(--surface-2)" }}>
          <div className="mb-2 text-[11px] font-medium uppercase tracking-[0.12em]" style={{ color: "var(--green)" }}>Bull case</div>
          <MiniBar value={bull.confidence || 0} color="var(--green)" />
          <p className="mt-2 text-xs leading-5" style={{ color: "var(--text-secondary)" }}>{bull.thesis || "No bullish setup."}</p>
        </div>
        <div className="rounded-2xl p-3" style={{ background: "var(--surface-2)" }}>
          <div className="mb-2 text-[11px] font-medium uppercase tracking-[0.12em]" style={{ color: "var(--red)" }}>Bear case</div>
          <MiniBar value={bear.confidence || 0} color="var(--red)" />
          <p className="mt-2 text-xs leading-5" style={{ color: "var(--text-secondary)" }}>{bear.thesis || "No bearish setup."}</p>
        </div>
        {geminiEnabled && !compact && (
          <div className="rounded-2xl p-3" style={{ background: "var(--surface-2)" }}>
            <div className="mb-2 text-[11px] font-medium uppercase tracking-[0.12em]" style={{ color: "var(--accent)" }}>Macro view</div>
            <MiniBar value={macro?.confidence || 0} color="var(--accent)" />
            <p className="mt-2 text-xs leading-5" style={{ color: "var(--text-secondary)" }}>{macro?.thesis || "No macro note."}</p>
          </div>
        )}
      </div>

      {!compact && (
        <div className="mt-3 rounded-2xl border p-3" style={{ borderColor: risk.approved ? "rgba(24, 128, 56, 0.14)" : "var(--border)", background: risk.approved ? "var(--green-dim)" : "var(--surface-2)" }}>
          <div className="text-[11px] font-medium uppercase tracking-[0.12em]" style={{ color: risk.approved ? "var(--green)" : "var(--red)" }}>Risk review</div>
          <p className="mt-1 text-xs leading-5" style={{ color: "var(--text-secondary)" }}>{risk.vetoReason || risk.reasoning || "No risk note."}</p>
        </div>
      )}
    </div>
  );
}

function AuditItem({ log }: { log: AuditLog }) {
  return (
    <div className="border-t py-3 first:border-t-0" style={{ borderColor: "var(--border-subtle)" }}>
      <div className="flex items-center gap-2">
        <span className="text-sm" style={{ color: log.approved ? "var(--green)" : "var(--red)" }}>{log.approved ? "OK" : "NO"}</span>
        <span className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>{log.strategyName}</span>
        <span className="ml-auto text-[11px] font-medium uppercase" style={{ color: log.action === "BUY" ? "var(--green)" : "var(--red)" }}>{log.action}</span>
      </div>
      <p className="mt-1 text-xs leading-5" style={{ color: "var(--text-secondary)" }}>{log.reason}</p>
    </div>
  );
}

export default function AIInsightPanel({
  enabled,
  geminiEnabled,
  message,
  latest,
  recent,
  auditLogs = [],
}: {
  enabled: boolean;
  geminiEnabled: boolean;
  message?: string;
  latest: AIDecision | null;
  recent: AIDecision[];
  auditLogs?: AuditLog[];
}) {
  if (!enabled) {
    return <div className="glass-panel p-5"><div className="text-sm" style={{ color: "var(--text-secondary)" }}>{message || "Set OPENAI_API_KEY to enable AI review."}</div></div>;
  }

  return (
    <div className="glass-panel p-5">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div>
          <div className="text-[11px] font-medium uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>RAIG AI panel</div>
          <div className="mt-1 text-sm font-medium" style={{ color: "var(--text-primary)" }}>Live trade reasoning</div>
        </div>
        <span className="rounded-full px-3 py-1 text-[11px] font-medium" style={{ background: "var(--accent-dim)", color: "var(--accent)" }}>GPT active</span>
      </div>
      {latest && <DecisionCard decision={latest} geminiEnabled={geminiEnabled} />}
      <div className="mt-4 rounded-[20px] border p-4" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
        <div className="mb-2 text-[11px] font-medium uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>Audit trail</div>
        {auditLogs.length > 0 ? auditLogs.slice(0, 5).map((log) => <AuditItem key={log.id} log={log} />) : <p className="text-sm" style={{ color: "var(--text-secondary)" }}>Waiting for strategy audits.</p>}
      </div>
      {recent.length > 1 && (
        <div className="mt-4 space-y-3">
          <div className="text-[11px] font-medium uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>Recent decisions</div>
          {recent.slice(1, 3).map((decision) => <DecisionCard key={decision.id} decision={decision} compact geminiEnabled={geminiEnabled} />)}
        </div>
      )}
    </div>
  );
}
