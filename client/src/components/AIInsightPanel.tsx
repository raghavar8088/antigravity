"use client";

import type { AIDecision } from "@/hooks/useAIInsights";

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
  // Gemini blue
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
      {/* Header row */}
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
          {/* ── 3-column agent grid ── */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: geminiEnabled ? "1fr 1fr 1fr" : "1fr 1fr",
              gap: 8,
            }}
          >
            {/* Bull */}
            <div style={{ background: "var(--surface-3)", borderRadius: 8, padding: "10px 12px" }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6 }}>
                <AgentBadge label="BULL" active={bull.shouldTrade} color={AGENT_COLORS.bull} />
                <span style={{ fontSize: 9, color: "var(--text-muted)" }}>GPT-4o</span>
              </div>
              <ConfidenceBar value={bull.confidence || 0} color={AGENT_COLORS.bull} />
              {bull.thesis && (
                <p style={{ fontSize: 11, color: "var(--text-secondary)", marginTop: 6, lineHeight: 1.5 }}>
                  {bull.thesis}
                </p>
              )}
              {bull.error && (
                <p style={{ fontSize: 10, color: "var(--red)", marginTop: 4 }}>⚠ {bull.error}</p>
              )}
            </div>

            {/* Bear */}
            <div style={{ background: "var(--surface-3)", borderRadius: 8, padding: "10px 12px" }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6 }}>
                <AgentBadge label="BEAR" active={bear.shouldTrade} color={AGENT_COLORS.bear} />
                <span style={{ fontSize: 9, color: "var(--text-muted)" }}>GPT-4o</span>
              </div>
              <ConfidenceBar value={bear.confidence || 0} color={AGENT_COLORS.bear} />
              {bear.thesis && (
                <p style={{ fontSize: 11, color: "var(--text-secondary)", marginTop: 6, lineHeight: 1.5 }}>
                  {bear.thesis}
                </p>
              )}
              {bear.error && (
                <p style={{ fontSize: 10, color: "var(--red)", marginTop: 4 }}>⚠ {bear.error}</p>
              )}
            </div>

            {/* Macro */}
            {geminiEnabled && (
              <div style={{ background: "var(--surface-3)", borderRadius: 8, padding: "10px 12px" }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6 }}>
                  <AgentBadge label="MACRO" active={macro?.shouldTrade ?? false} color={AGENT_COLORS.macro} />
                  <span style={{ fontSize: 9, color: AGENT_COLORS.macro, fontWeight: 600 }}>Gemini</span>
                </div>
                <ConfidenceBar value={macro?.confidence || 0} color={AGENT_COLORS.macro} />
                {macro?.thesis && (
                  <p style={{ fontSize: 11, color: "var(--text-secondary)", marginTop: 6, lineHeight: 1.5 }}>
                    {macro.thesis}
                  </p>
                )}
                {macro?.error && !macro.thesis?.includes("disabled") && (
                  <p style={{ fontSize: 10, color: "var(--red)", marginTop: 4 }}>⚠ {macro.error}</p>
                )}
              </div>
            )}
          </div>

          {/* Risk verdict */}
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
                <span style={{ fontSize: 10, fontWeight: 700, color: risk.approved ? "var(--green)" : "var(--red)", letterSpacing: "0.1em" }}>
                  RISK AGENT
                </span>
                <span style={{ fontSize: 9, color: "var(--text-muted)" }}>GPT-4o</span>
              </div>
              <p style={{ fontSize: 11, color: "var(--text-secondary)", marginTop: 2, lineHeight: 1.5 }}>
                {risk.vetoReason || risk.reasoning}
              </p>
            </div>
          </div>
        </>
      )}

      {compact && (
        <p style={{ fontSize: 11, color: "var(--text-secondary)", lineHeight: 1.5 }}>
          {risk.vetoReason || risk.reasoning}
        </p>
      )}
    </div>
  );
}

// ─── Main panel ───────────────────────────────────────────────────

export default function AIInsightPanel({
  enabled,
  geminiEnabled,
  message,
  latest,
  recent,
}: {
  enabled: boolean;
  geminiEnabled: boolean;
  message?: string;
  latest: AIDecision | null;
  recent: AIDecision[];
}) {
  if (!enabled) {
    return (
      <div className="glass-panel p-5">
        <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 12 }}>
          <span style={{ fontSize: 16 }}>🤖</span>
          <h2 style={{ fontSize: 13, fontWeight: 700, color: "white", letterSpacing: "0.05em" }}>
            GPT AI Agents
          </h2>
          <span style={{ marginLeft: "auto", fontSize: 10, color: "var(--text-muted)", background: "var(--surface-3)", padding: "2px 8px", borderRadius: 999 }}>
            DISABLED
          </span>
        </div>
        <div
          style={{
            background: "rgba(245,158,11,0.08)",
            border: "1px solid rgba(245,158,11,0.25)",
            borderRadius: 10,
            padding: "12px 14px",
          }}
        >
          <p style={{ fontSize: 11, color: "#F59E0B", lineHeight: 1.6 }}>
            {message || "Set OPENAI_API_KEY in Render environment to enable GPT multi-agent trading."}
          </p>
          <p style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 6 }}>
            When enabled: Bull + Bear (GPT-4o-mini) debate every 5m candle. GPT-4o Risk Agent arbitrates using the Trading Constitution. Gemini Flash provides macro overlay.
          </p>
        </div>
      </div>
    );
  }

  const agentLabel = geminiEnabled
    ? "Bull · Bear · Macro (Gemini) · Risk"
    : "Bull · Bear · Risk · every 5m";

  return (
    <div className="glass-panel p-5" style={{ display: "flex", flexDirection: "column", gap: 14 }}>
      {/* Panel header */}
      <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
        <span style={{ fontSize: 16 }}>🤖</span>
        <h2 style={{ fontSize: 13, fontWeight: 700, color: "white", letterSpacing: "0.05em" }}>
          GPT Agent Council
        </h2>
        <span
          style={{
            marginLeft: 4,
            fontSize: 10,
            fontWeight: 700,
            color: "#10A37F",
            background: "rgba(16,163,127,0.1)",
            padding: "2px 8px",
            borderRadius: 999,
            border: "1px solid rgba(16,163,127,0.25)",
          }}
        >
          ● GPT-4o LIVE
        </span>
        {/* Gemini badge */}
        {geminiEnabled && (
          <span
            style={{
              fontSize: 10,
              fontWeight: 700,
              color: "#7C9FFF",
              background: "rgba(124,159,255,0.1)",
              padding: "2px 8px",
              borderRadius: 999,
              border: "1px solid rgba(124,159,255,0.25)",
            }}
          >
            ✦ Gemini Macro
          </span>
        )}
        <span style={{ marginLeft: "auto", fontSize: 10, color: "var(--text-muted)" }}>
          {agentLabel}
        </span>
      </div>

      {/* Latest decision — expanded */}
      {latest ? (
        <DecisionCard decision={latest} geminiEnabled={geminiEnabled} />
      ) : (
        <div
          style={{
            background: "var(--surface-2)",
            border: "1px dashed var(--border)",
            borderRadius: 12,
            padding: 20,
            textAlign: "center",
            color: "var(--text-muted)",
            fontSize: 12,
          }}
        >
          Waiting for first 5m candle close…
        </div>
      )}

      {/* Recent decisions — compact list */}
      {recent.length > 1 && (
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          <div style={{ fontSize: 10, fontWeight: 600, color: "var(--text-muted)", letterSpacing: "0.12em", textTransform: "uppercase" }}>
            Recent Decisions
          </div>
          {recent.slice(1, 6).map((d) => (
            <DecisionCard key={d.id} decision={d} compact geminiEnabled={geminiEnabled} />
          ))}
        </div>
      )}
    </div>
  );
}
