"use client";

import type { AIDecision, AuditLog } from "@/hooks/useAIInsights";

function AgentBadge({ label, active, color }: { label: string; active: boolean; color: string }) {
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 5,
        padding: "3px 10px",
        borderRadius: 999,
        fontSize: 10,
        fontWeight: 700,
        letterSpacing: "0.1em",
        background: active ? `${color}18` : "var(--surface-3)",
        color: active ? color : "var(--text-muted)",
        border: `1px solid ${active ? color + "35" : "transparent"}`,
      }}
    >
      {label}
    </span>
  );
}

function ConfidenceBar({ value, color }: { value: number; color: string }) {
  const pct = Math.round(value * 100);
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
      <div
        style={{
          flex: 1,
          height: 5,
          borderRadius: 3,
          background: "var(--surface-3)",
          overflow: "hidden",
        }}
      >
        <div
          style={{
            width: `${pct}%`,
            height: "100%",
            borderRadius: 3,
            background: color,
            transition: "width 0.4s ease",
          }}
        />
      </div>
      <span style={{ fontSize: 11, fontWeight: 700, color, minWidth: 32, textAlign: "right" }}>
        {pct}%
      </span>
    </div>
  );
}

function ActionPill({ action }: { action: string }) {
  const cfg = {
    BUY:  { bg: "var(--green-dim)",           color: "var(--green)",  label: "▲ BUY" },
    SELL: { bg: "var(--red-dim)",              color: "var(--red)",    label: "▼ SELL" },
    HOLD: { bg: "rgba(255,255,255,0.06)",      color: "var(--text-muted)", label: "— HOLD" },
  }[action] ?? { bg: "rgba(255,255,255,0.06)", color: "var(--text-muted)", label: action };

  return (
    <span
      style={{
        padding: "4px 14px",
        borderRadius: 999,
        fontSize: 12,
        fontWeight: 800,
        letterSpacing: "0.1em",
        background: cfg.bg,
        color: cfg.color,
      }}
    >
      {cfg.label}
    </span>
  );
}

// Color palette per agent
const AGENT_COLORS = {
  bull:  "var(--green)",
  bear:  "var(--red)",
  macro: "#7C9FFF",
};

function DecisionCard({
  decision,
  compact = false,
  geminiEnabled,
}: {
  decision: AIDecision;
  compact?: boolean;
  geminiEnabled?: boolean;
}) {
  const ts = new Date(decision.timestamp);
  const timeStr = ts.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
  const bull  = decision.bullSignal;
  const bear  = decision.bearSignal;
  const macro = decision.macroSignal;
  const risk  = decision.riskVerdict;

  return (
    <div
      style={{
        background: "var(--surface-2)",
        border: "1px solid var(--border)",
        borderRadius: 12,
        padding: compact ? "10px 14px" : "16px",
        display: "flex",
        flexDirection: "column",
        gap: 10,
      }}
    >
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: 6 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ fontFamily: "monospace", fontSize: 10, color: "var(--text-muted)" }}>{decision.id}</span>
          <ActionPill action={decision.finalAction} />
          {decision.executed && (
            <span style={{ fontSize: 10, color: "var(--green)", fontWeight: 700 }}>✓ EXECUTED</span>
          )}
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ fontSize: 10, color: "var(--text-muted)" }}>{decision.regime}</span>
          <span style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-secondary)" }}>
            ${decision.price.toLocaleString(undefined, { maximumFractionDigits: 0 })}
          </span>
          <span style={{ fontSize: 10, color: "var(--text-muted)" }}>{timeStr}</span>
        </div>
      </div>

      {!compact && (
        <>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: geminiEnabled ? "1fr 1fr 1fr" : "1fr 1fr",
              gap: 8,
            }}
          >
            <div style={{ background: "var(--surface-3)", borderRadius: 8, padding: "10px 12px" }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6 }}>
                <AgentBadge label="BULL" active={bull.shouldTrade} color={AGENT_COLORS.bull} />
                <span style={{ fontSize: 9, color: "var(--text-muted)" }}>GPT-4o</span>
              </div>
              <ConfidenceBar value={bull.confidence || 0} color={AGENT_COLORS.bull} />
              {bull.thesis && <p style={{ fontSize: 11, color: "var(--text-secondary)", marginTop: 6, lineHeight: 1.5 }}>{bull.thesis}</p>}
            </div>
            <div style={{ background: "var(--surface-3)", borderRadius: 8, padding: "10px 12px" }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6 }}>
                <AgentBadge label="BEAR" active={bear.shouldTrade} color={AGENT_COLORS.bear} />
                <span style={{ fontSize: 9, color: "var(--text-muted)" }}>GPT-4o</span>
              </div>
              <ConfidenceBar value={bear.confidence || 0} color={AGENT_COLORS.bear} />
              {bear.thesis && <p style={{ fontSize: 11, color: "var(--text-secondary)", marginTop: 6, lineHeight: 1.5 }}>{bear.thesis}</p>}
            </div>
            {geminiEnabled && (
              <div style={{ background: "var(--surface-3)", borderRadius: 8, padding: "10px 12px" }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6 }}>
                  <AgentBadge label="MACRO" active={macro?.shouldTrade ?? false} color={AGENT_COLORS.macro} />
                  <span style={{ fontSize: 9, color: AGENT_COLORS.macro, fontWeight: 600 }}>Gemini</span>
                </div>
                <ConfidenceBar value={macro?.confidence || 0} color={AGENT_COLORS.macro} />
                {macro?.thesis && <p style={{ fontSize: 11, color: "var(--text-secondary)", marginTop: 6, lineHeight: 1.5 }}>{macro.thesis}</p>}
              </div>
            )}
          </div>

          <div
            style={{
              background: risk.approved ? "var(--green-dim)" : "rgba(255,255,255,0.04)",
              border: `1px solid ${risk.approved ? "rgba(0,208,156,0.2)" : "var(--border)"}`,
              borderRadius: 8,
              padding: "8px 12px",
              display: "flex",
              alignItems: "flex-start",
              gap: 8,
            }}
          >
            <span style={{ fontSize: 13 }}>{risk.approved ? "✅" : "⛔"}</span>
            <div>
              <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                <span style={{ fontSize: 10, fontWeight: 700, color: risk.approved ? "var(--green)" : "var(--red)", letterSpacing: "0.1em" }}>RISK AGENT</span>
                <span style={{ fontSize: 9, color: "var(--text-muted)" }}>GPT-4o</span>
              </div>
              <p style={{ fontSize: 11, color: "var(--text-secondary)", marginTop: 2, lineHeight: 1.5 }}>{risk.vetoReason || risk.reasoning}</p>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

function AuditRow({ log }: { log: AuditLog }) {
  const ts = new Date(log.timestamp);
  const timeStr = ts.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 10,
        padding: "8px 0",
        borderBottom: "1px solid var(--border-subtle)",
      }}
    >
      <div style={{ minWidth: 20 }}>{log.approved ? "✅" : "⛔"}</div>
      <div style={{ flex: 1 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ fontSize: 11, fontWeight: 700, color: "white" }}>{log.strategyName}</span>
          <span style={{ fontSize: 10, fontWeight: 800, color: log.action === "BUY" ? "var(--green)" : "var(--red)" }}>{log.action}</span>
          <span style={{ marginLeft: "auto", fontSize: 10, color: "var(--text-muted)", fontFamily: "monospace" }}>{timeStr}</span>
        </div>
        <p style={{ fontSize: 10, color: "var(--text-secondary)", marginTop: 2, lineHeight: 1.4 }}>
          {log.reason}
        </p>
      </div>
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
    return (
      <div className="glass-panel p-5">
        <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 12 }}>
          <span style={{ fontSize: 16 }}>🤖</span>
          <h2 style={{ fontSize: 13, fontWeight: 700, color: "white", letterSpacing: "0.05em" }}>GPT AI Agents</h2>
          <span style={{ marginLeft: "auto", fontSize: 10, color: "var(--text-muted)", background: "var(--surface-3)", padding: "2px 8px", borderRadius: 999 }}>DISABLED</span>
        </div>
        <div style={{ background: "rgba(245,158,11,0.08)", border: "1px solid rgba(245,158,11,0.25)", borderRadius: 10, padding: "12px 14px" }}>
          <p style={{ fontSize: 11, color: "#F59E0B", lineHeight: 1.6 }}>{message || "Set OPENAI_API_KEY to enable GPT trading."}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="glass-panel p-5" style={{ display: "flex", flexDirection: "column", gap: 14 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span style={{ fontSize: 16 }}>🤖</span>
        <h2 style={{ fontSize: 13, fontWeight: 700, color: "white", letterSpacing: "0.05em" }}>GPT Agent Council</h2>
        <span style={{ marginLeft: 4, fontSize: 10, fontWeight: 700, color: "#10A37F", background: "rgba(16,163,127,0.1)", padding: "2px 8px", borderRadius: 999, border: "1px solid rgba(16,163,127,0.25)" }}>● GPT-4o LIVE</span>
      </div>

      {latest && <DecisionCard decision={latest} geminiEnabled={geminiEnabled} />}

      {/* NEW: AI Auditor Section */}
      <div style={{ background: "rgba(255,255,255,0.03)", borderRadius: 12, padding: "12px 14px", border: "1px solid var(--border-subtle)" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 8 }}>
          <div style={{ fontSize: 10, fontWeight: 700, color: "var(--text-muted)", letterSpacing: "0.1em" }}>SUPREME COURT AUDITS</div>
          <span style={{ fontSize: 9, color: "var(--text-muted)" }}>REAL-TIME VETTING</span>
        </div>
        
        {auditLogs.length > 0 ? (
          <div style={{ display: "flex", flexDirection: "column" }}>
            {auditLogs.slice(0, 5).map((log) => (
              <AuditRow key={log.id} log={log} />
            ))}
          </div>
        ) : (
          <p style={{ fontSize: 10, color: "var(--text-muted)", textAlign: "center", padding: "10px 0" }}>Waiting for strategy signals to audit...</p>
        )}
      </div>

      {recent.length > 1 && (
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          <div style={{ fontSize: 10, fontWeight: 600, color: "var(--text-muted)", letterSpacing: "0.12em", textTransform: "uppercase" }}>Recent Decisions</div>
          {recent.slice(1, 4).map((d) => (
            <DecisionCard key={d.id} decision={d} compact geminiEnabled={geminiEnabled} />
          ))}
        </div>
      )}
    </div>
  );
}
